package reqx

import (
	"fmt"
	"net/http"

	"github.com/yogorobot/app-mall/chix/resp"
)

const (
	ViolationCodeInvalid  = "invalid"
	ViolationCodeRequired = "required"
	ViolationCodeUnknown  = "unknown"
	ViolationCodeType     = "type"
	ViolationCodeMultiple = "multiple"
)

// Violation 描述单个字段校验失败。
type Violation struct {
	Field   string `json:"field,omitempty"`
	Code    string `json:"code"`
	Message string `json:"message"`
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

func normalizeViolation(violation Violation) Violation {
	if violation.Code == "" {
		violation.Code = ViolationCodeInvalid
	}
	if violation.Message != "" {
		return violation
	}

	switch violation.Code {
	case ViolationCodeRequired:
		violation.Message = "is required"
	case ViolationCodeUnknown:
		violation.Message = "unknown field"
	case ViolationCodeType:
		violation.Message = "has invalid type"
	default:
		violation.Message = "is invalid"
	}
	return violation
}
