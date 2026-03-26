package reqx

import (
	"encoding"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"reflect"
	"strconv"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-playground/validator/v10"

	"github.com/kanata996/chix/internal/reqmeta"
)

var (
	timeType       = reflect.TypeOf(time.Time{})
	defaultDecoder = New()
)

type Option func(*Decoder)

type Decoder struct {
	validate *validator.Validate
}

func WithValidator(validate *validator.Validate) Option {
	return func(d *Decoder) {
		d.validate = validate
	}
}

func New(options ...Option) *Decoder {
	decoder := &Decoder{}
	for _, option := range options {
		option(decoder)
	}
	if decoder.validate == nil {
		decoder.validate = validator.New()
	}
	configureValidator(decoder.validate)
	return decoder
}

func (d *Decoder) Validator() *validator.Validate {
	if d == nil {
		return nil
	}
	return d.validate
}

func Decode(r *http.Request, dst any) error {
	return defaultDecoder.Decode(r, dst)
}

func (d *Decoder) Decode(r *http.Request, dst any) error {
	if dst == nil {
		return nil
	}

	value := reflect.ValueOf(dst)
	if value.Kind() != reflect.Pointer || value.IsNil() {
		return NewError(KindInvalidDestination, "input destination must be a non-nil pointer")
	}

	if err := decodeInput(r, dst, value); err != nil {
		return err
	}

	if d == nil || d.validate == nil {
		return nil
	}

	inputType := reqmeta.IndirectType(value.Elem().Type())
	if inputType.Kind() != reflect.Struct {
		return nil
	}

	if err := d.validate.Struct(dst); err != nil {
		var invalid *validator.InvalidValidationError
		if errors.As(err, &invalid) {
			return NewError(KindInvalidDestination, "input destination must be a struct pointer").WithCause(err)
		}

		var validationErrors validator.ValidationErrors
		if errors.As(err, &validationErrors) {
			return NewError(KindValidation, "request validation failed").WithViolations(translateValidationErrors(inputType, validationErrors)).WithCause(err)
		}

		return NewError(KindValidation, "request validation failed").WithCause(err)
	}

	return nil
}

func decodeInput(r *http.Request, dst any, value reflect.Value) error {
	bodyPresent, bodyRequired := reqmeta.BodyInfo(value.Elem().Type())
	if bodyPresent {
		if requestHasBody(r) {
			decoder := json.NewDecoder(r.Body)
			decoder.DisallowUnknownFields()

			if err := decoder.Decode(dst); err != nil {
				if err != io.EOF {
					return NewError(KindInvalidBody, "invalid JSON body").WithCause(err)
				}
			}

			var extra json.RawMessage
			if err := decoder.Decode(&extra); err != io.EOF {
				return NewError(KindInvalidBody, "request body must contain a single JSON value")
			}
		} else if bodyRequired {
			return NewError(KindMissingBody, "missing JSON body")
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

func bindStructFields(r *http.Request, value reflect.Value) error {
	var bindErr error

	reqmeta.WalkStructFields(value.Type(), func(field reflect.StructField, index []int) bool {
		if bindErr != nil {
			return false
		}
		if field.PkgPath != "" && !field.Anonymous {
			return true
		}

		source, name, ok := reqmeta.ParameterSource(field)
		if !ok {
			return true
		}

		values, exists := parameterValues(r, source, name)
		if !exists || len(values) == 0 || values[0] == "" {
			if field.Tag.Get("required") == "true" || source == "path" {
				requestErr := NewError(KindMissingParameter, fmt.Sprintf("missing required %s parameter %q", source, name))
				requestErr.Source = source
				requestErr.Name = name
				bindErr = requestErr
			}
			return true
		}

		target, err := reqmeta.FieldByIndexAlloc(value, index)
		if err != nil {
			bindErr = NewError(KindInvalidDestination, err.Error())
			return false
		}

		if err := assignValues(target, values); err != nil {
			requestErr := NewError(KindInvalidParameter, fmt.Sprintf("invalid %s parameter %q", source, name)).WithCause(err)
			requestErr.Source = source
			requestErr.Name = name
			bindErr = requestErr
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

func configureValidator(validate *validator.Validate) {
	validate.RegisterTagNameFunc(func(field reflect.StructField) string {
		name := reqmeta.RequestFieldName(field)
		if name == "" {
			return field.Name
		}
		return name
	})
}

func translateValidationErrors(rootType reflect.Type, validationErrors validator.ValidationErrors) []Violation {
	violations := make([]Violation, 0, len(validationErrors))
	for _, fieldErr := range validationErrors {
		source, field := violationLocation(rootType, fieldErr.StructNamespace())
		rule := fieldErr.ActualTag()
		if rule == "" {
			rule = fieldErr.Tag()
		}
		violations = append(violations, Violation{
			Source:  source,
			Field:   field,
			Rule:    rule,
			Param:   fieldErr.Param(),
			Message: validationMessage(field, rule, fieldErr.Param()),
		})
	}
	return violations
}

func violationLocation(rootType reflect.Type, namespace string) (source, field string) {
	source = "body"
	rootType = reqmeta.IndirectType(rootType)

	rootPrefix := rootType.Name()
	if rootPrefix != "" {
		namespace = strings.TrimPrefix(namespace, rootPrefix)
		namespace = strings.TrimPrefix(namespace, ".")
	}
	if namespace == "" {
		return source, field
	}

	current := rootType
	var parts []string
	for _, token := range strings.Split(namespace, ".") {
		name, suffix := splitIndexedToken(token)
		if current.Kind() != reflect.Struct {
			parts = append(parts, token)
			continue
		}

		structField, ok := current.FieldByName(name)
		if !ok {
			parts = append(parts, token)
			continue
		}

		if fieldSource, fieldName, ok := reqmeta.ParameterSource(structField); ok {
			source = fieldSource
			parts = append(parts, fieldName+suffix)
		} else {
			fieldName, _, skip := reqmeta.JSONFieldName(structField)
			if skip || fieldName == "" {
				fieldName = structField.Name
			}
			parts = append(parts, fieldName+suffix)
		}

		current = reqmeta.IndirectType(structField.Type)
		if suffix != "" && (current.Kind() == reflect.Slice || current.Kind() == reflect.Array) {
			current = reqmeta.IndirectType(current.Elem())
		}
	}

	return source, strings.Join(parts, ".")
}

func splitIndexedToken(token string) (name string, suffix string) {
	index := strings.Index(token, "[")
	if index < 0 {
		return token, ""
	}
	return token[:index], token[index:]
}

func validationMessage(field, rule, param string) string {
	label := field
	if label == "" {
		label = "value"
	}

	switch rule {
	case "required":
		return fmt.Sprintf("%s is required", label)
	case "email":
		return fmt.Sprintf("%s must be a valid email address", label)
	case "uuid", "uuid4":
		return fmt.Sprintf("%s must be a valid UUID", label)
	case "oneof":
		return fmt.Sprintf("%s must be one of %s", label, param)
	case "min":
		return fmt.Sprintf("%s must be at least %s", label, param)
	case "max":
		return fmt.Sprintf("%s must be at most %s", label, param)
	case "len":
		return fmt.Sprintf("%s must be exactly %s", label, param)
	default:
		if param == "" {
			return fmt.Sprintf("%s failed %s validation", label, rule)
		}
		return fmt.Sprintf("%s failed %s=%s validation", label, rule, param)
	}
}
