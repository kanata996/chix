package schema

import (
	"encoding"
	"fmt"
	"reflect"
	"strings"
	"sync"
	"time"
)

// Location 表示结构体命名空间对应的输入来源和字段路径。
type Location struct {
	Source string
	Field  string
}

// Field 表示一个可绑定字段的来源、名称、结构体路径和反射索引。
type Field struct {
	Source string
	Name   string
	Path   string
	Index  []int
}

// Schema 表示输入结构的绑定描述，并维护参数、请求体和位置查找索引。
type Schema struct {
	ParameterFields []Field
	BodyFields      []Field
	HasValidation   bool

	bodyFieldsByName map[string]Field
	locations        map[string]Location
}

// cachedSchema 封装 schemaCache 中缓存的解析结果及其错误。
type cachedSchema struct {
	schema *Schema
	err    error
}

var schemaCache sync.Map

var (
	textUnmarshalerType = reflect.TypeOf((*encoding.TextUnmarshaler)(nil)).Elem()
	timeType            = reflect.TypeOf(time.Time{})
)

// Load 解析并缓存给定结构体类型的输入描述；t 必须是结构体或其指针。
func Load(t reflect.Type) (*Schema, error) {
	t = indirectType(t)
	if t == nil || t.Kind() != reflect.Struct {
		return nil, fmt.Errorf("chix: input schema type must be a struct")
	}

	if cached, ok := schemaCache.Load(t); ok {
		entry := cached.(*cachedSchema)
		return entry.schema, entry.err
	}

	schema, err := build(t)
	entry := &cachedSchema{
		schema: schema,
		err:    err,
	}

	actual, _ := schemaCache.LoadOrStore(t, entry)
	stored := actual.(*cachedSchema)
	return stored.schema, stored.err
}

// LookupLocation 按结构体命名空间查找绑定位置；命名空间包含顶层结构体名时会忽略首段。
func (s *Schema) LookupLocation(structNamespace string) (Location, bool) {
	if s == nil {
		return Location{}, false
	}

	parts := strings.Split(structNamespace, ".")
	switch len(parts) {
	case 0:
		return Location{}, false
	case 1:
		location, ok := s.locations[parts[0]]
		return location, ok
	default:
		location, ok := s.locations[strings.Join(parts[1:], ".")]
		return location, ok
	}
}

// LookupBodyField 按请求体字段名查找顶层 body 字段描述。
func (s *Schema) LookupBodyField(name string) (Field, bool) {
	if s == nil {
		return Field{}, false
	}

	field, ok := s.bodyFieldsByName[name]
	return field, ok
}

// build 基于结构体类型构建完整的输入绑定描述和查找索引。
func build(t reflect.Type) (*Schema, error) {
	schema := &Schema{
		bodyFieldsByName: make(map[string]Field),
		locations:        make(map[string]Location),
	}

	bodyNames := make(map[string]struct{})
	if err := walk(t, nil, "", nil, [][]string{{}}, schema, bodyNames); err != nil {
		return nil, err
	}

	return schema, nil
}

// walk 深度遍历结构体字段并收集绑定信息；会继承外层来源与路径，并在顶层校验可绑定字段。
func walk(
	t reflect.Type,
	prefix []int,
	inheritedSource string,
	inheritedPath []string,
	namespaceAliases [][]string,
	schema *Schema,
	bodyNames map[string]struct{},
) error {
	t = indirectType(t)
	if t == nil || t.Kind() != reflect.Struct {
		return nil
	}

	for i := 0; i < t.NumField(); i++ {
		field := t.Field(i)
		index := appendIndex(prefix, i)
		fieldType := indirectType(field.Type)

		if isTransparentEmbed(field, fieldType) {
			if err := walk(
				fieldType,
				index,
				inheritedSource,
				inheritedPath,
				appendAnonymousAlias(namespaceAliases, field.Name),
				schema,
				bodyNames,
			); err != nil {
				return err
			}
			continue
		}

		if field.PkgPath != "" && !field.Anonymous {
			continue
		}
		if tag := field.Tag.Get("validate"); tag != "" && tag != "-" {
			schema.HasValidation = true
		}

		paramSource, paramName, hasParamSource, err := parameterSource(field)
		if err != nil {
			return err
		}

		bodyName, hasBodySource, bodyIgnored := bodyFieldName(field)

		fieldSource, fieldName, fieldPath, bindable, err := resolveField(field, inheritedSource, inheritedPath, paramSource, paramName, hasParamSource, bodyName, hasBodySource, bodyIgnored)
		if err != nil {
			return err
		}
		if !bindable {
			continue
		}

		location := Location{
			Source: fieldSource,
			Field:  strings.Join(fieldPath, "."),
		}
		registerLocation(schema.locations, appendFieldName(namespaceAliases, field.Name), location)

		descriptor := Field{
			Source: fieldSource,
			Name:   fieldName,
			Path:   location.Field,
			Index:  index,
		}

		if inheritedSource == "" {
			switch fieldSource {
			case "path", "query":
				if err := validateParameterType(field, fieldSource); err != nil {
					return err
				}
				schema.ParameterFields = append(schema.ParameterFields, descriptor)
			case "body":
				if _, exists := bodyNames[fieldName]; exists {
					return fmt.Errorf("chix: duplicate body field %q", fieldName)
				}
				bodyNames[fieldName] = struct{}{}
				schema.BodyFields = append(schema.BodyFields, descriptor)
				schema.bodyFieldsByName[fieldName] = descriptor
			}
		}

		if fieldType.Kind() == reflect.Struct {
			if err := walk(
				fieldType,
				index,
				fieldSource,
				fieldPath,
				appendFieldName(namespaceAliases, field.Name),
				schema,
				bodyNames,
			); err != nil {
				return err
			}
		}
	}

	return nil
}

// resolveField 根据字段标签和外层继承来源解析实际输入来源、绑定名称与路径。
// 它会拒绝多来源声明，并要求 body 来源字段显式声明 json 标签。
func resolveField(
	field reflect.StructField,
	inheritedSource string,
	inheritedPath []string,
	paramSource string,
	paramName string,
	hasParamSource bool,
	bodyName string,
	hasBodySource bool,
	bodyIgnored bool,
) (source string, name string, path []string, bindable bool, err error) {
	if inheritedSource == "body" {
		if hasParamSource {
			if hasBodySource {
				return "", "", nil, false, fmt.Errorf(
					"chix: field %q must declare a single input source, found %s and json",
					field.Name,
					paramSource,
				)
			}
			return "", "", nil, false, fmt.Errorf(
				"chix: body field %q must not declare %s source",
				field.Name,
				paramSource,
			)
		}
		if bodyIgnored {
			return "", "", nil, false, nil
		}
		if !hasBodySource {
			return "", "", nil, false, fmt.Errorf("chix: body field %q must declare a json tag", field.Name)
		}
		return "body", bodyName, appendPath(inheritedPath, bodyName), true, nil
	}

	if inheritedSource != "" {
		source = inheritedSource
		name = locationName(field, inheritedSource, paramSource, paramName, hasParamSource, bodyName, hasBodySource)
		return source, name, appendPath(inheritedPath, name), true, nil
	}

	if hasParamSource {
		if hasBodySource {
			return "", "", nil, false, fmt.Errorf(
				"chix: field %q must declare a single input source, found %s and json",
				field.Name,
				paramSource,
			)
		}
		return paramSource, paramName, []string{paramName}, true, nil
	}

	if hasBodySource {
		return "body", bodyName, []string{bodyName}, true, nil
	}

	return "", "", nil, false, nil
}

// registerLocation 为一组命名空间别名注册相同的绑定位置。
func registerLocation(locations map[string]Location, aliases [][]string, location Location) {
	for _, alias := range aliases {
		if len(alias) == 0 {
			continue
		}
		locations[strings.Join(alias, ".")] = location
	}
}

// appendFieldName 为每个命名空间别名追加字段名。
func appendFieldName(aliases [][]string, name string) [][]string {
	next := make([][]string, 0, len(aliases))
	for _, alias := range aliases {
		next = append(next, appendPath(alias, name))
	}
	return next
}

// appendAnonymousAlias 为匿名嵌入同时生成扁平和带字段名的两类命名空间别名。
func appendAnonymousAlias(aliases [][]string, name string) [][]string {
	next := make([][]string, 0, len(aliases)*2)
	for _, alias := range aliases {
		next = append(next, appendPath(alias))
		next = append(next, appendPath(alias, name))
	}
	return next
}

// appendPath 复制 base 并追加路径片段，避免复用底层切片。
func appendPath(base []string, parts ...string) []string {
	next := append([]string(nil), base...)
	next = append(next, parts...)
	return next
}

// appendIndex 复制 base 并追加字段索引片段，避免复用底层切片。
func appendIndex(base []int, parts ...int) []int {
	next := append([]int(nil), base...)
	next = append(next, parts...)
	return next
}

// isTransparentEmbed 判断字段是否为按透明方式展开的匿名结构体嵌入。
func isTransparentEmbed(field reflect.StructField, fieldType reflect.Type) bool {
	return field.Anonymous && field.Tag == "" && fieldType.Kind() == reflect.Struct
}

// locationName 在继承来源场景下决定字段在位置索引中的名称，优先使用对应来源的显式标签名。
func locationName(
	field reflect.StructField,
	source string,
	paramSource string,
	paramName string,
	hasParamSource bool,
	bodyName string,
	hasBodySource bool,
) string {
	switch source {
	case "path", "query":
		if hasParamSource && paramSource == source {
			return paramName
		}
	case "body":
		if hasBodySource {
			return bodyName
		}
	}

	return field.Name
}

// parameterSource 解析字段声明的 path/query 参数来源；同时声明多个来源时返回错误。
func parameterSource(field reflect.StructField) (source string, name string, ok bool, err error) {
	var matches []string

	for _, candidate := range []string{"path", "query"} {
		taggedName := tagName(field.Tag.Get(candidate))
		if taggedName != "" {
			source = candidate
			name = taggedName
			matches = append(matches, candidate)
		}
	}

	if len(matches) > 1 {
		return "", "", false, fmt.Errorf("chix: field %q declares multiple parameter sources", field.Name)
	}
	if len(matches) == 0 {
		return "", "", false, nil
	}

	return source, name, true, nil
}

// bodyFieldName 解析 json 标签对应的 body 字段名，并区分未声明与显式忽略。
func bodyFieldName(field reflect.StructField) (string, bool, bool) {
	tag, ok := field.Tag.Lookup("json")
	if !ok {
		return "", false, false
	}
	if tag == "-" {
		return "", false, true
	}

	name, _, _ := strings.Cut(tag, ",")
	if name == "" {
		name = field.Name
	}
	return name, true, false
}

// tagName 返回标签值中逗号前的名称部分。
func tagName(tag string) string {
	if tag == "" {
		return ""
	}

	name, _, _ := strings.Cut(tag, ",")
	return name
}

// validateParameterType 校验 path/query 字段是否属于支持的参数绑定类型。
func validateParameterType(field reflect.StructField, source string) error {
	if supportsParameterType(field.Type) {
		return nil
	}

	return fmt.Errorf("chix: %s field %q has unsupported type %s", source, field.Name, field.Type)
}

// supportsParameterType 判断类型是否可从 path/query 参数绑定；切片仅支持其元素类型可按单值绑定。
func supportsParameterType(t reflect.Type) bool {
	if t == nil {
		return false
	}

	if t.Kind() == reflect.Slice {
		return supportsParameterScalarType(t.Elem(), false)
	}

	return supportsParameterScalarType(t, true)
}

// supportsParameterScalarType 判断单个参数值是否可绑定到给定类型；allowPointer 控制是否先解引用指针。
func supportsParameterScalarType(t reflect.Type, allowPointer bool) bool {
	if allowPointer {
		t = indirectType(t)
	}
	if t == nil {
		return false
	}

	if t == timeType || reflect.PointerTo(t).Implements(textUnmarshalerType) {
		return true
	}

	switch t.Kind() {
	case reflect.String, reflect.Bool,
		reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64,
		reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64,
		reflect.Float32, reflect.Float64:
		return true
	default:
		return false
	}
}

// indirectType 反复解引用指针并返回最终元素类型。
func indirectType(t reflect.Type) reflect.Type {
	for t != nil && t.Kind() == reflect.Pointer {
		t = t.Elem()
	}
	return t
}
