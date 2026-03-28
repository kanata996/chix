package binding

import (
	"fmt"
	"reflect"
)

func walkStructFields(t reflect.Type, visit func(field reflect.StructField, index []int) bool) {
	t = indirectType(t)
	if t.Kind() != reflect.Struct {
		return
	}
	walkStructFieldsWithIndex(t, nil, visit)
}

func walkStructFieldsWithIndex(t reflect.Type, prefix []int, visit func(field reflect.StructField, index []int) bool) bool {
	for i := 0; i < t.NumField(); i++ {
		field := t.Field(i)
		index := append(append([]int(nil), prefix...), i)
		fieldType := indirectType(field.Type)
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

func fieldByIndexAlloc(value reflect.Value, index []int) (reflect.Value, error) {
	current := value
	for _, part := range index {
		current = ensureSettableValue(current)
		current = current.Field(part)
	}
	if !current.CanSet() {
		return reflect.Value{}, fmt.Errorf("chix: field is not settable")
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

func indirectType(t reflect.Type) reflect.Type {
	for t.Kind() == reflect.Pointer {
		t = t.Elem()
	}
	return t
}
