package binding

import (
	"fmt"
	"reflect"
)

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
