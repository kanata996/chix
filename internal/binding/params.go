package binding

import (
	"encoding"
	"fmt"
	"net/http"
	"reflect"
	"strconv"
	"time"

	"github.com/go-chi/chi/v5"
)

var timeType = reflect.TypeOf(time.Time{})

func bindParameterFields(r *http.Request, value reflect.Value) error {
	var bindErr error

	walkStructFields(value.Type(), func(field reflect.StructField, index []int) bool {
		if bindErr != nil {
			return false
		}
		if field.PkgPath != "" && !field.Anonymous {
			return true
		}

		source, name, ok, err := parameterSource(field)
		if err != nil {
			bindErr = err
			return false
		}
		if !ok {
			return true
		}

		values, exists := parameterValues(r, source, name)
		if !exists || len(values) == 0 {
			return true
		}

		target, err := fieldByIndexAlloc(value, index)
		if err != nil {
			bindErr = err
			return false
		}

		if len(values) > 1 && target.Kind() != reflect.Slice {
			bindErr = newRequestShapeError(fmt.Errorf("duplicate %s parameter %q", source, name))
			return false
		}

		if err := assignValues(target, values); err != nil {
			bindErr = newRequestShapeError(fmt.Errorf("invalid %s parameter %q: %w", source, name, err))
			return false
		}

		return true
	})

	return bindErr
}

func parameterValues(r *http.Request, source string, name string) ([]string, bool) {
	switch source {
	case "path":
		value := chi.URLParam(r, name)
		if value == "" {
			return nil, false
		}
		return []string{value}, true
	case "query":
		values, ok := r.URL.Query()[name]
		return values, ok
	default:
		return nil, false
	}
}

func assignValues(target reflect.Value, values []string) error {
	target = ensureSettableValue(target)

	if target.Kind() == reflect.Slice {
		slice := reflect.MakeSlice(target.Type(), 0, len(values))
		for _, raw := range values {
			item := reflect.New(target.Type().Elem()).Elem()
			if err := assignScalar(item, raw); err != nil {
				return err
			}
			slice = reflect.Append(slice, item)
		}
		target.Set(slice)
		return nil
	}

	return assignScalar(target, values[0])
}

func assignScalar(target reflect.Value, raw string) error {
	if target.CanAddr() {
		if unmarshaler, ok := target.Addr().Interface().(encoding.TextUnmarshaler); ok {
			return unmarshaler.UnmarshalText([]byte(raw))
		}
	}

	if target.Type() == timeType {
		parsed, err := time.Parse(time.RFC3339, raw)
		if err != nil {
			return err
		}
		target.Set(reflect.ValueOf(parsed))
		return nil
	}

	switch target.Kind() {
	case reflect.String:
		target.SetString(raw)
	case reflect.Bool:
		value, err := strconv.ParseBool(raw)
		if err != nil {
			return err
		}
		target.SetBool(value)
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		value, err := strconv.ParseInt(raw, 10, target.Type().Bits())
		if err != nil {
			return err
		}
		target.SetInt(value)
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		value, err := strconv.ParseUint(raw, 10, target.Type().Bits())
		if err != nil {
			return err
		}
		target.SetUint(value)
	case reflect.Float32, reflect.Float64:
		value, err := strconv.ParseFloat(raw, target.Type().Bits())
		if err != nil {
			return err
		}
		target.SetFloat(value)
	default:
		return fmt.Errorf("unsupported parameter type %s", target.Type())
	}

	return nil
}
