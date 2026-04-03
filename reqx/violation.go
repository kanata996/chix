package reqx

import (
	"fmt"
	"net/http"

	"github.com/kanata996/chix/resp"
)

const (
	ViolationCodeInvalid  = "invalid"
	ViolationCodeRequired = "required"
	ViolationCodeUnknown  = "unknown"
	ViolationCodeType     = "type"
	ViolationCodeMultiple = "multiple"
)

const (
	violationDetailInvalid       = "is invalid"
	violationDetailRequired      = "is required"
	violationDetailUnknownField  = "unknown field"
	violationDetailInvalidType   = "has invalid type"
	violationDetailMustNotRepeat = "must not be repeated"
)

const (
	ViolationInBody    = "body"
	ViolationInQuery   = "query"
	ViolationInPath    = "path"
	ViolationInHeader  = "header"
	ViolationInRequest = "request"
)

// Violation 描述单个字段校验失败。
type Violation struct {
	Field   string `json:"field,omitempty"`
	In      string `json:"in,omitempty"`
	Code    string `json:"code"`
	Detail  string `json:"detail"`
	Message string `json:"-"`
}

// ValidateFunc 校验已经解码好的请求值。
type ValidateFunc[T any] func(*T) []Violation

func Validate[T any](dst *T, fn ValidateFunc[T]) error {
	if fn == nil {
		return nil
	}
	if dst == nil {
		return fmt.Errorf("reqx: destination must not be nil")
	}

	violations := fn(dst)
	if len(violations) == 0 {
		return nil
	}
	return invalidFieldsError(violations)
}

func invalidFieldError(violation Violation) error {
	return invalidFieldsError([]Violation{violation})
}

func invalidFieldsError(violations []Violation) error {
	details := make([]any, 0, len(violations))
	for _, violation := range violations {
		details = append(details, normalizeViolation(violation))
	}
	return resp.NewError(
		http.StatusUnprocessableEntity,
		CodeInvalidRequest,
		"request contains invalid fields",
		details...,
	)
}

func newViolation(field, input, code, detail string) Violation {
	return Violation{
		Field:   field,
		In:      input,
		Code:    code,
		Detail:  detail,
		Message: detail,
	}
}

func normalizeViolation(violation Violation) Violation {
	if violation.Code == "" {
		violation.Code = ViolationCodeInvalid
	}
	if violation.Detail == "" && violation.Message != "" {
		violation.Detail = violation.Message
	}
	if violation.Detail == "" {
		violation.Detail = violationDetailForCode(violation.Code)
	}
	violation.Message = violation.Detail
	return violation
}

func violationDetailForCode(code string) string {
	switch code {
	case ViolationCodeRequired:
		return violationDetailRequired
	case ViolationCodeUnknown:
		return violationDetailUnknownField
	case ViolationCodeType:
		return violationDetailInvalidType
	case ViolationCodeMultiple:
		return violationDetailMustNotRepeat
	default:
		return violationDetailInvalid
	}
}
