package chix

import (
	"encoding"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"reflect"
	"strconv"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
)

var timeType = reflect.TypeOf(time.Time{})

func bindInput(r *http.Request, dst any) error {
	if dst == nil {
		return nil
	}

	value := reflect.ValueOf(dst)
	if value.Kind() != reflect.Pointer || value.IsNil() {
		return StatusError(http.StatusInternalServerError, "input destination must be a non-nil pointer")
	}

	bodyPresent, bodyRequired := bodyInfo(value.Elem().Type())
	if bodyPresent {
		if requestHasBody(r) {
			decoder := json.NewDecoder(r.Body)
			decoder.DisallowUnknownFields()

			if err := decoder.Decode(dst); err != nil {
				if err != io.EOF {
					return StatusError(http.StatusBadRequest, "invalid JSON body").WithCause(err)
				}
			}

			var extra json.RawMessage
			if err := decoder.Decode(&extra); err != io.EOF {
				return StatusError(http.StatusBadRequest, "request body must contain a single JSON value")
			}
		} else if bodyRequired {
			return StatusError(http.StatusBadRequest, "missing JSON body")
		}
	}

	if value.Elem().Kind() == reflect.Struct {
		if err := bindStructFields(r, value.Elem()); err != nil {
			return err
		}
	}

	return nil
}

func requestHasBody(r *http.Request) bool {
	return r.ContentLength > 0 || len(r.TransferEncoding) > 0
}

func bodyInfo(t reflect.Type) (present bool, required bool) {
	t = indirectType(t)
	if t.Kind() != reflect.Struct {
		return true, true
	}

	walkStructFields(t, func(field reflect.StructField, _ []int) bool {
		if field.PkgPath != "" && !field.Anonymous {
			return true
		}
		if isParameterField(field) {
			return true
		}
		name, omitempty, skip := jsonFieldName(field)
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

func bindStructFields(r *http.Request, value reflect.Value) error {
	var bindErr error

	walkStructFields(value.Type(), func(field reflect.StructField, index []int) bool {
		if bindErr != nil {
			return false
		}
		if field.PkgPath != "" && !field.Anonymous {
			return true
		}

		source, name, ok := parameterSource(field)
		if !ok {
			return true
		}

		values, exists := parameterValues(r, source, name)
		if !exists || len(values) == 0 || values[0] == "" {
			if field.Tag.Get("required") == "true" || source == "path" {
				bindErr = StatusError(http.StatusBadRequest, fmt.Sprintf("missing required %s parameter %q", source, name))
			}
			return true
		}

		target, err := fieldByIndexAlloc(value, index)
		if err != nil {
			bindErr = StatusError(http.StatusInternalServerError, err.Error())
			return false
		}

		if err := assignValues(target, values); err != nil {
			bindErr = StatusError(http.StatusBadRequest, fmt.Sprintf("invalid %s parameter %q", source, name)).WithCause(err)
			return false
		}

		return true
	})

	return bindErr
}

func parameterValues(r *http.Request, source, name string) ([]string, bool) {
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
	case "header":
		values := r.Header.Values(name)
		if len(values) == 0 {
			return nil, false
		}
		return values, true
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

func ensureSettableValue(value reflect.Value) reflect.Value {
	for value.Kind() == reflect.Pointer {
		if value.IsNil() {
			value.Set(reflect.New(value.Type().Elem()))
		}
		value = value.Elem()
	}
	return value
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

func parameterSource(field reflect.StructField) (source, name string, ok bool) {
	for _, source := range []string{"path", "query", "header"} {
		if name = tagName(field.Tag.Get(source)); name != "" {
			return source, name, true
		}
	}
	return "", "", false
}

func isParameterField(field reflect.StructField) bool {
	_, _, ok := parameterSource(field)
	return ok
}

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
		return reflect.Value{}, fmt.Errorf("field is not settable")
	}
	return current, nil
}

func indirectType(t reflect.Type) reflect.Type {
	for t.Kind() == reflect.Pointer {
		t = t.Elem()
	}
	return t
}

func tagName(tag string) string {
	if tag == "" {
		return ""
	}
	name, _, _ := strings.Cut(tag, ",")
	return name
}

func jsonFieldName(field reflect.StructField) (name string, omitempty bool, skip bool) {
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
