package reqmeta

import (
	"fmt"
	"reflect"
	"strings"
)

func IndirectType(t reflect.Type) reflect.Type {
	for t.Kind() == reflect.Pointer {
		t = t.Elem()
	}
	return t
}

func TagName(tag string) string {
	if tag == "" {
		return ""
	}
	name, _, _ := strings.Cut(tag, ",")
	return name
}

func JSONFieldName(field reflect.StructField) (name string, omitempty bool, skip bool) {
	tag := field.Tag.Get("json")
	if tag == "-" {
		return "", false, true
	}

	if tag == "" {
		return field.Name, false, false
	}

	parts := strings.Split(tag, ",")
	name = parts[0]
	if name == "" {
		name = field.Name
	}

	for _, part := range parts[1:] {
		if part == "omitempty" {
			omitempty = true
		}
	}

	return name, omitempty, false
}

func ParameterSource(field reflect.StructField) (source, name string, ok bool) {
	for _, source := range []string{"path", "query", "header"} {
		if name = TagName(field.Tag.Get(source)); name != "" {
			return source, name, true
		}
	}
	return "", "", false
}

func IsParameterField(field reflect.StructField) bool {
	_, _, ok := ParameterSource(field)
	return ok
}

func RequestFieldName(field reflect.StructField) string {
	if _, name, ok := ParameterSource(field); ok {
		return name
	}

	name, _, skip := JSONFieldName(field)
	if skip {
		return ""
	}
	return name
}

func WalkStructFields(t reflect.Type, visit func(field reflect.StructField, index []int) bool) {
	t = IndirectType(t)
	if t.Kind() != reflect.Struct {
		return
	}
	walkStructFieldsWithIndex(t, nil, visit)
}

func walkStructFieldsWithIndex(t reflect.Type, prefix []int, visit func(field reflect.StructField, index []int) bool) bool {
	for i := 0; i < t.NumField(); i++ {
		field := t.Field(i)
		index := append(append([]int(nil), prefix...), i)
		fieldType := IndirectType(field.Type)
		anonymousStruct := field.Anonymous && field.Tag == "" && fieldType.Kind() == reflect.Struct
		if anonymousStruct {
			if !walkStructFieldsWithIndex(fieldType, index, visit) {
				return false
			}
			continue
		}
		if !visit(field, index) {
			return false
		}
	}
	return true
}

func BodyInfo(t reflect.Type) (present bool, required bool) {
	t = IndirectType(t)
	if t.Kind() != reflect.Struct {
		return true, true
	}

	WalkStructFields(t, func(field reflect.StructField, _ []int) bool {
		if field.PkgPath != "" && !field.Anonymous {
			return true
		}
		if IsParameterField(field) {
			return true
		}
		name, omitempty, skip := JSONFieldName(field)
		if skip || name == "" {
			return true
		}
		present = true
		if !omitempty && field.Type.Kind() != reflect.Pointer {
			required = true
		}
		return true
	})

	return present, required
}

func FieldByIndexAlloc(value reflect.Value, index []int) (reflect.Value, error) {
	current := value
	for _, part := range index {
		current = ensureSettableValue(current)
		current = current.Field(part)
	}
	if !current.CanSet() {
		return reflect.Value{}, fmt.Errorf("field is not settable")
	}
	return current, nil
}

func ensureSettableValue(value reflect.Value) reflect.Value {
	for value.Kind() == reflect.Pointer {
		if value.IsNil() {
			value.Set(reflect.New(value.Type().Elem()))
		}
		value = value.Elem()
	}
	return value
}
