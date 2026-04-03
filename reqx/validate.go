package reqx

import (
	"errors"
	"fmt"
	"net/http"
	"net/textproto"
	"reflect"
	"sort"
	"strings"
	"sync"

	"github.com/go-playground/validator/v10"
	"github.com/kanata996/chix/resp"
)

type Normalizer interface {
	Normalize()
}

type sourceKind string

const (
	sourceBody    sourceKind = "json"
	sourceQuery   sourceKind = "query"
	sourcePath    sourceKind = "param"
	sourceHeader  sourceKind = "header"
	sourceRequest sourceKind = "request"
)

var (
	validatorOnce sync.Once
	validators    map[sourceKind]*validator.Validate
)

func BindAndValidate[T any](r *http.Request, dst *T, opts ...BindOption) error {
	if err := Bind(r, dst, opts...); err != nil {
		return err
	}
	return ValidateRequest(dst)
}

func BindAndValidateBody[T any](r *http.Request, dst *T, opts ...BindBodyOption) error {
	if err := BindBody(r, dst, opts...); err != nil {
		return err
	}
	return ValidateBody(dst)
}

func BindAndValidateQuery[T any](r *http.Request, dst *T, opts ...BindQueryParamsOption) error {
	if err := BindQueryParams(r, dst, opts...); err != nil {
		return err
	}
	return ValidateQuery(dst)
}

func BindAndValidatePath[T any](r *http.Request, dst *T) error {
	if err := BindPathValues(r, dst); err != nil {
		return err
	}
	return ValidatePath(dst)
}

func BindAndValidateHeaders[T any](r *http.Request, dst *T, opts ...BindHeadersOption) error {
	if err := BindHeaders(r, dst, opts...); err != nil {
		return err
	}
	return ValidateHeaders(dst)
}

func ValidateBody[T any](target *T) error {
	return validate(target, sourceBody)
}

func ValidateRequest[T any](target *T) error {
	return validate(target, sourceRequest)
}

func ValidateQuery[T any](target *T) error {
	return validate(target, sourceQuery)
}

func ValidatePath[T any](target *T) error {
	return validate(target, sourcePath)
}

func ValidateHeaders[T any](target *T) error {
	return validate(target, sourceHeader)
}

func BadRequest(violations ...Violation) error {
	return resp.NewError(
		http.StatusBadRequest,
		"invalid_request",
		"request contains invalid fields",
		violationsToAny(violations)...,
	)
}

func InvalidField(field string) Violation {
	return InvalidFieldIn(ViolationInBody, field)
}

func InvalidFieldIn(input, field string) Violation {
	return Violation{
		Field:   field,
		In:      input,
		Code:    "invalid",
		Detail:  "is invalid",
		Message: "is invalid",
	}
}

func RequiredField(field string) Violation {
	return RequiredFieldIn(ViolationInBody, field)
}

func RequiredFieldIn(input, field string) Violation {
	return Violation{
		Field:   field,
		In:      input,
		Code:    "required",
		Detail:  "is required",
		Message: "is required",
	}
}

func validate[T any](target *T, source sourceKind) error {
	if err := validateTarget(target); err != nil {
		return err
	}

	normalizeTarget(target)

	violations, err := validateStruct(target, source)
	if err != nil {
		return err
	}
	if len(violations) == 0 {
		return nil
	}

	return invalidFieldsError(violations)
}

func validateTarget(target any) error {
	if target == nil {
		return errorsf("target must not be nil")
	}

	value := reflect.ValueOf(target)
	if value.Kind() != reflect.Pointer || value.IsNil() || value.Elem().Kind() != reflect.Struct {
		return errorsf("target must be a non-nil pointer to struct")
	}

	return nil
}

func normalizeTarget(target any) {
	if normalizer, ok := target.(Normalizer); ok {
		normalizer.Normalize()
	}
}

func validateStruct(target any, source sourceKind) ([]Violation, error) {
	err := validatorFor(source).Struct(target)
	if err == nil {
		return nil, nil
	}

	var invalidValidationErr *validator.InvalidValidationError
	if errors.As(err, &invalidValidationErr) {
		return nil, err
	}

	// validator/v10's Struct contract returns only nil,
	// InvalidValidationError, or ValidationErrors.
	validationErrs := err.(validator.ValidationErrors)
	return violationsFromValidation(source, target, validationErrs), nil
}

func validatorFor(source sourceKind) *validator.Validate {
	validatorOnce.Do(func() {
		validators = map[sourceKind]*validator.Validate{
			sourceBody:    newValidator(sourceBody),
			sourceQuery:   newValidator(sourceQuery),
			sourcePath:    newValidator(sourcePath),
			sourceHeader:  newValidator(sourceHeader),
			sourceRequest: newValidator(sourceRequest),
		}
	})

	v, ok := validators[source]
	if !ok {
		panic(fmt.Sprintf("reqx: unsupported validation source %q", source))
	}
	return v
}

func newValidator(source sourceKind) *validator.Validate {
	v := validator.New(validator.WithRequiredStructEnabled())
	v.RegisterTagNameFunc(func(field reflect.StructField) string {
		return fieldAlias(field, source)
	})
	mustRegisterValidation(v, "nospace", validateNoSpace)
	return v
}

func mustRegisterValidation(v *validator.Validate, tag string, fn validator.Func) {
	if err := v.RegisterValidation(tag, fn); err != nil {
		panic(fmt.Sprintf("reqx: register validator %q: %v", tag, err))
	}
}

func validateNoSpace(fl validator.FieldLevel) bool {
	field := fl.Field()
	if field.Kind() != reflect.String {
		return false
	}
	return !strings.ContainsRune(field.String(), ' ')
}

func violationsFromValidation(source sourceKind, args ...any) []Violation {
	target, errs := validationArgs(args...)
	if len(errs) == 0 {
		return nil
	}

	seen := make(map[string]struct{}, len(errs))
	type entry struct {
		field string
		in    string
		code  string
	}
	entries := make([]entry, 0, len(errs))

	for _, validationErr := range errs {
		field := validationFieldPath(source, validationErr)
		if _, ok := seen[field]; ok {
			continue
		}
		seen[field] = struct{}{}
		entries = append(entries, entry{
			field: field,
			in:    validationInput(source, target, validationErr),
			code:  validationCode(validationErr.Tag()),
		})
	}

	sort.Slice(entries, func(i, j int) bool {
		return entries[i].field < entries[j].field
	})

	violations := make([]Violation, 0, len(entries))
	for _, entry := range entries {
		violations = append(violations, Violation{
			Field:   entry.field,
			In:      entry.in,
			Code:    entry.code,
			Detail:  validationDetail(entry.code),
			Message: validationDetail(entry.code),
		})
	}
	return violations
}

func validationArgs(args ...any) (any, validator.ValidationErrors) {
	switch len(args) {
	case 1:
		if errs, ok := args[0].(validator.ValidationErrors); ok {
			return nil, errs
		}
	case 2:
		if errs, ok := args[1].(validator.ValidationErrors); ok {
			return args[0], errs
		}
	}
	return nil, nil
}

func validationFieldPath(source sourceKind, err validator.FieldError) string {
	namespace := strings.TrimSpace(err.Namespace())
	if namespace != "" {
		if dot := strings.Index(namespace, "."); dot >= 0 {
			namespace = namespace[dot+1:]
		}
		namespace = strings.TrimSpace(namespace)
		if namespace != "" {
			return namespace
		}
	}

	field := strings.TrimSpace(err.Field())
	if field != "" {
		return field
	}

	switch source {
	case sourceBody:
		return "body"
	default:
		return "request"
	}
}

func validationCode(tag string) string {
	switch tag {
	case "required":
		return "required"
	default:
		return "invalid"
	}
}

func validationDetail(code string) string {
	if code == "required" {
		return "is required"
	}
	return "is invalid"
}

func validationInput(source sourceKind, target any, err validator.FieldError) string {
	if source != sourceRequest {
		return violationInForSource(source)
	}

	field, ok := resolveValidationField(target, err.StructNamespace())
	if !ok {
		return ViolationInRequest
	}

	for _, tagName := range sourceTagPriority(sourceRequest) {
		if name := tagValue(field, tagName); name != "" {
			return violationInForTag(tagName)
		}
	}
	return ViolationInRequest
}

func violationInForSource(source sourceKind) string {
	switch source {
	case sourceBody:
		return ViolationInBody
	case sourceQuery:
		return ViolationInQuery
	case sourcePath:
		return ViolationInPath
	case sourceHeader:
		return ViolationInHeader
	default:
		return ViolationInRequest
	}
}

func violationInForTag(tagName string) string {
	switch tagName {
	case "json":
		return ViolationInBody
	case "query":
		return ViolationInQuery
	case "param":
		return ViolationInPath
	case "header":
		return ViolationInHeader
	default:
		return ViolationInRequest
	}
}

func resolveValidationField(target any, structNamespace string) (reflect.StructField, bool) {
	t := reflect.TypeOf(target)
	for t != nil && t.Kind() == reflect.Pointer {
		t = t.Elem()
	}
	if t == nil || t.Kind() != reflect.Struct {
		return reflect.StructField{}, false
	}

	path := parseStructNamespace(structNamespace)
	if len(path) == 0 {
		return reflect.StructField{}, false
	}

	current := t
	for _, name := range path[:len(path)-1] {
		field, ok := current.FieldByName(name)
		if !ok {
			return reflect.StructField{}, false
		}

		next := field.Type
		for next.Kind() == reflect.Pointer || next.Kind() == reflect.Slice || next.Kind() == reflect.Array {
			next = next.Elem()
		}
		if next.Kind() != reflect.Struct {
			return reflect.StructField{}, false
		}
		current = next
	}

	field, ok := current.FieldByName(path[len(path)-1])
	if !ok {
		return reflect.StructField{}, false
	}
	return field, true
}

func parseStructNamespace(namespace string) []string {
	namespace = strings.TrimSpace(namespace)
	if namespace == "" {
		return nil
	}

	parts := strings.Split(namespace, ".")
	if len(parts) <= 1 {
		return nil
	}

	path := make([]string, 0, len(parts)-1)
	for _, part := range parts[1:] {
		if bracket := strings.Index(part, "["); bracket >= 0 {
			part = part[:bracket]
		}
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		path = append(path, part)
	}
	return path
}

func fieldAlias(field reflect.StructField, source sourceKind) string {
	for _, tagName := range sourceTagPriority(source) {
		if name := tagValue(field, tagName); name != "" {
			if tagName == "header" {
				return textproto.CanonicalMIMEHeaderKey(name)
			}
			return name
		}
	}
	return field.Name
}

func sourceTagPriority(source sourceKind) []string {
	switch source {
	case sourceBody:
		return []string{"json", "query", "param", "header"}
	case sourceQuery:
		return []string{"query", "json", "param", "header"}
	case sourcePath:
		return []string{"param", "json", "query", "header"}
	case sourceHeader:
		return []string{"header", "json", "query", "param"}
	case sourceRequest:
		return []string{"param", "query", "json", "header"}
	default:
		panic(fmt.Sprintf("reqx: unsupported tag source %q", source))
	}
}

func tagValue(field reflect.StructField, tagName string) string {
	value := strings.TrimSpace(field.Tag.Get(tagName))
	if value == "" {
		return ""
	}
	value = strings.Split(value, ",")[0]
	value = strings.TrimSpace(value)
	if value == "" || value == "-" {
		return ""
	}
	return value
}

func violationsToAny(violations []Violation) []any {
	items := make([]any, 0, len(violations))
	for _, violation := range violations {
		items = append(items, violation)
	}
	return items
}

func errorsf(format string, args ...any) error {
	return fmt.Errorf("reqx: "+format, args...)
}
