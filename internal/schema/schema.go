package schema

import (
	"encoding"
	"fmt"
	"reflect"
	"strings"
	"sync"
	"time"
)

type Location struct {
	Source string
	Field  string
}

type Field struct {
	Source string
	Name   string
	Path   string
	Index  []int
}

type Schema struct {
	ParameterFields []Field
	BodyFields      []Field

	bodyFieldsByName map[string]Field
	locations        map[string]Location
}

type cachedSchema struct {
	schema *Schema
	err    error
}

var schemaCache sync.Map

var (
	textUnmarshalerType = reflect.TypeOf((*encoding.TextUnmarshaler)(nil)).Elem()
	timeType            = reflect.TypeOf(time.Time{})
)

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

func (s *Schema) LookupBodyField(name string) (Field, bool) {
	if s == nil {
		return Field{}, false
	}

	field, ok := s.bodyFieldsByName[name]
	return field, ok
}

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

func registerLocation(locations map[string]Location, aliases [][]string, location Location) {
	for _, alias := range aliases {
		if len(alias) == 0 {
			continue
		}
		locations[strings.Join(alias, ".")] = location
	}
}

func appendFieldName(aliases [][]string, name string) [][]string {
	next := make([][]string, 0, len(aliases))
	for _, alias := range aliases {
		next = append(next, appendPath(alias, name))
	}
	return next
}

func appendAnonymousAlias(aliases [][]string, name string) [][]string {
	next := make([][]string, 0, len(aliases)*2)
	for _, alias := range aliases {
		next = append(next, appendPath(alias))
		next = append(next, appendPath(alias, name))
	}
	return next
}

func appendPath(base []string, parts ...string) []string {
	next := append([]string(nil), base...)
	next = append(next, parts...)
	return next
}

func appendIndex(base []int, parts ...int) []int {
	next := append([]int(nil), base...)
	next = append(next, parts...)
	return next
}

func isTransparentEmbed(field reflect.StructField, fieldType reflect.Type) bool {
	return field.Anonymous && field.Tag == "" && fieldType.Kind() == reflect.Struct
}

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

func tagName(tag string) string {
	if tag == "" {
		return ""
	}

	name, _, _ := strings.Cut(tag, ",")
	return name
}

func validateParameterType(field reflect.StructField, source string) error {
	if supportsParameterType(field.Type) {
		return nil
	}

	return fmt.Errorf("chix: %s field %q has unsupported type %s", source, field.Name, field.Type)
}

func supportsParameterType(t reflect.Type) bool {
	if t == nil {
		return false
	}

	if t.Kind() == reflect.Slice {
		return supportsParameterScalarType(t.Elem(), false)
	}

	return supportsParameterScalarType(t, true)
}

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

func indirectType(t reflect.Type) reflect.Type {
	for t != nil && t.Kind() == reflect.Pointer {
		t = t.Elem()
	}
	return t
}
