package reqx

import (
	"errors"
	"fmt"
	"reflect"
	"sort"
	"strings"
	"sync"

	"github.com/go-playground/validator/v10"
)

var (
	validatorOnce      sync.Once
	validatorInstances map[sourceKind]*validator.Validate
)

// Normalizer 允许 DTO 在校验前先做最小规范化。
// 常见用法是 trim 字符串、归一化空值或默认值。
type Normalizer interface {
	Normalize()
}

type sourceKind string

const (
	sourceJSON  sourceKind = "json"
	sourceQuery sourceKind = "query"
	sourcePath  sourceKind = "param"
)

func ValidateBody(target any) error {
	return validateForSource(target, sourceJSON)
}

func ValidateQuery(target any) error {
	return validateForSource(target, sourceQuery)
}

func ValidatePath(target any) error {
	return validateForSource(target, sourcePath)
}

// validateForSource 统一处理 normalize、validator 校验以及请求错误映射。
func validateForSource(target any, source sourceKind) error {
	if err := validateTarget(target); err != nil {
		return err
	}

	normalizeTarget(target)

	details := validateStructForSource(target, source)
	if len(details) == 0 {
		return nil
	}

	switch source {
	case sourceJSON:
		return ValidationFailed(details...)
	default:
		return BadRequest(details...)
	}
}

func validateTarget(target any) error {
	if target == nil {
		return ErrNilTarget
	}

	value := reflect.ValueOf(target)
	if !value.IsValid() {
		return ErrNilTarget
	}
	if value.Kind() != reflect.Pointer || value.IsNil() {
		return fmt.Errorf("%w: target must be a non-nil pointer to struct", ErrInvalidValidateTarget)
	}
	if value.Elem().Kind() != reflect.Struct {
		return fmt.Errorf("%w: target must be a non-nil pointer to struct", ErrInvalidValidateTarget)
	}

	return nil
}

// validateStructForSource 只做 validator -> []Detail 的转换，不做响应写回。
func validateStructForSource(target any, source sourceKind) []Detail {
	in := source.in()
	err := validatorEngine(source).Struct(target)
	if err == nil {
		return nil
	}

	var invalidValidationErr *validator.InvalidValidationError
	if errors.As(err, &invalidValidationErr) {
		field := "request"
		if in == InBody {
			field = "body"
		}
		return []Detail{{
			In:    in,
			Field: field,
			Code:  DetailCodeInvalidValue,
		}}
	}

	var validationErrs validator.ValidationErrors
	if !errors.As(err, &validationErrs) {
		return nil
	}

	return detailsFromValidation(source, validationErrs)
}

// validatorEngine 为 body/query/path 分别构造 validator，
// 这样 Field() 直接就是当前 source 对应的 tag 名，无需再做复杂反射修正。
func validatorEngine(source sourceKind) *validator.Validate {
	validatorOnce.Do(func() {
		validatorInstances = map[sourceKind]*validator.Validate{
			sourceJSON:  newValidator(sourceJSON),
			sourceQuery: newValidator(sourceQuery),
			sourcePath:  newValidator(sourcePath),
		}
	})

	if validatorInstance, ok := validatorInstances[source]; ok {
		return validatorInstance
	}
	return validatorInstances[sourceJSON]
}

func newValidator(source sourceKind) *validator.Validate {
	v := validator.New(validator.WithRequiredStructEnabled())
	v.RegisterTagNameFunc(func(field reflect.StructField) string {
		return fieldAlias(field, source)
	})
	mustRegisterValidation(v, "nospace", validateNoSpace)
	return v
}

func fieldAlias(field reflect.StructField, source sourceKind) string {
	for _, tagName := range sourceTagPriority(source) {
		if name := tagValue(field, tagName); name != "" {
			return name
		}
	}
	return field.Name
}

func validationCode(tag string) string {
	switch tag {
	case "required":
		return DetailCodeRequired
	case "uuid", "uuid3", "uuid4", "uuid5", "uuid_rfc4122":
		return DetailCodeInvalidUUID
	case "min", "max", "len", "gt", "gte", "lt", "lte":
		return DetailCodeOutOfRange
	default:
		return DetailCodeInvalidValue
	}
}

// detailsFromValidation 把 validator 错误压成稳定的 Detail 列表。
// 这里有意只保留 field/code，不再生成展示型 message。
func detailsFromValidation(source sourceKind, errs validator.ValidationErrors) []Detail {
	if len(errs) == 0 {
		return nil
	}

	in := source.in()
	seen := make(map[string]struct{})
	type entry struct {
		field string
		code  string
	}
	entries := make([]entry, 0, len(errs))

	for _, validationErr := range errs {
		field := strings.TrimSpace(validationErr.Field())
		if field == "" {
			field = defaultField(source)
		}
		if _, exists := seen[field]; exists {
			continue
		}
		seen[field] = struct{}{}
		entries = append(entries, entry{
			field: field,
			code:  validationCode(validationErr.Tag()),
		})
	}

	sort.Slice(entries, func(i int, j int) bool {
		return entries[i].field < entries[j].field
	})

	details := make([]Detail, 0, len(entries))
	for _, entry := range entries {
		details = append(details, Detail{
			In:    in,
			Field: entry.field,
			Code:  entry.code,
		})
	}
	return details
}

// normalizeTarget 支持 DTO 在校验前自行归一化。
func normalizeTarget(target any) {
	if normalizer, ok := target.(Normalizer); ok {
		normalizer.Normalize()
	}
}

func (source sourceKind) in() string {
	switch source {
	case sourceJSON:
		return InBody
	case sourcePath:
		return InPath
	case sourceQuery:
		return InQuery
	default:
		return InBody
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

func sourceTagPriority(source sourceKind) []string {
	switch source {
	case sourceQuery:
		return []string{"query", "json", "param"}
	case sourcePath:
		return []string{"param", "json", "query"}
	default:
		return []string{"json", "query", "param"}
	}
}

func defaultField(source sourceKind) string {
	if source == sourceJSON {
		return "body"
	}
	return "request"
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
