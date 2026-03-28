package binding

import (
	"encoding"
	"fmt"
	"net/http"
	"net/url"
	"reflect"
	"strconv"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/kanata996/chix/internal/schema"
)

var timeType = reflect.TypeOf(time.Time{})

func bindParameterFields(r *http.Request, value reflect.Value, schema *schema.Schema) error {
	var (
		queryValues url.Values
		queryLoaded bool
	)

	for _, field := range schema.ParameterFields {
		switch field.Source {
		case "path":
			raw := chi.URLParam(r, field.Name)
			if raw == "" {
				continue
			}

			target, err := fieldByIndexAlloc(value, field.Index)
			if err != nil {
				return err
			}

			if err := assignSingleValue(target, raw); err != nil {
				return newRequestShapeError(fmt.Errorf("invalid %s parameter %q: %w", field.Source, field.Name, err))
			}
		case "query":
			if !queryLoaded {
				queryValues = r.URL.Query()
				queryLoaded = true
			}

			values, exists := queryValues[field.Name]
			if !exists || len(values) == 0 {
				continue
			}

			target, err := fieldByIndexAlloc(value, field.Index)
			if err != nil {
				return err
			}

			if len(values) > 1 && target.Kind() != reflect.Slice {
				return newRequestShapeError(fmt.Errorf("duplicate %s parameter %q", field.Source, field.Name))
			}

			if len(values) == 1 {
				if err := assignSingleValue(target, values[0]); err != nil {
					return newRequestShapeError(fmt.Errorf("invalid %s parameter %q: %w", field.Source, field.Name, err))
				}
				continue
			}

			if err := assignValues(target, values); err != nil {
				return newRequestShapeError(fmt.Errorf("invalid %s parameter %q: %w", field.Source, field.Name, err))
			}
		}
	}

	return nil
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

func assignSingleValue(target reflect.Value, raw string) error {
	target = ensureSettableValue(target)

	if target.Kind() == reflect.Slice {
		item := reflect.New(target.Type().Elem()).Elem()
		if err := assignScalar(item, raw); err != nil {
			return err
		}

		slice := reflect.MakeSlice(target.Type(), 1, 1)
		slice.Index(0).Set(item)
		target.Set(slice)
		return nil
	}

	return assignScalar(target, raw)
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
