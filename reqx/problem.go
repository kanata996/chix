package reqx

import (
	"errors"
	"net/http"
)

const (
	InBody   = "body"
	InHeader = "header"
	InPath   = "path"
	InQuery  = "query"
)

const (
	DetailCodeRequired             = "required"
	DetailCodeMalformedJSON        = "malformed_json"
	DetailCodeInvalidType          = "invalid_type"
	DetailCodeUnknownField         = "unknown_field"
	DetailCodeTrailingData         = "trailing_data"
	DetailCodeMultipleValues       = "multiple_values"
	DetailCodeUnsupportedMediaType = "unsupported_media_type"
	DetailCodePayloadTooLarge      = "payload_too_large"
	DetailCodeInvalidUUID          = "invalid_uuid"
	DetailCodeInvalidInteger       = "invalid_integer"
	DetailCodeOutOfRange           = "out_of_range"
	DetailCodeInvalidValue         = "invalid_value"
)

type Detail struct {
	In    string `json:"in,omitempty"`
	Field string `json:"field"`
	Code  string `json:"code,omitempty"`
}

// Problem 是请求侧错误的统一载体。
// StatusCode 只表达 HTTP 语义；稳定错误码由 resp 层映射。
type Problem struct {
	StatusCode int
	Details    []Detail
}

func (p *Problem) Error() string {
	if p == nil {
		return "request problem"
	}
	if p.StatusCode == http.StatusUnprocessableEntity {
		return "Validation Failed"
	}
	if message := http.StatusText(p.StatusCode); message != "" {
		return message
	}
	return "request problem"
}

func AsProblem(err error) (*Problem, bool) {
	var problem *Problem
	if !errors.As(err, &problem) {
		return nil, false
	}
	return problem, true
}

// BadRequest 用于 path/query 解析错误和通用 400 请求错误。
func BadRequest(details ...Detail) *Problem {
	return newProblem(http.StatusBadRequest, details...)
}

// ValidationFailed 用于 body 已成功解析，但 DTO 校验失败的场景。
func ValidationFailed(details ...Detail) *Problem {
	return newProblem(http.StatusUnprocessableEntity, details...)
}

// UnsupportedMediaType 用于 body 媒体类型不受支持的场景。
func UnsupportedMediaType(details ...Detail) *Problem {
	if len(details) == 0 {
		details = []Detail{{In: InBody, Field: "body", Code: DetailCodeUnsupportedMediaType}}
	}
	return newProblem(http.StatusUnsupportedMediaType, details...)
}

// PayloadTooLarge 用于 body 超过服务端允许上限的场景。
func PayloadTooLarge(details ...Detail) *Problem {
	if len(details) == 0 {
		details = []Detail{{In: InBody, Field: "body", Code: DetailCodePayloadTooLarge}}
	}
	return newProblem(http.StatusRequestEntityTooLarge, details...)
}

// 下列 helper 用于构造稳定的请求错误细节。
func Required(in string, field string) Detail {
	return Detail{In: in, Field: field, Code: DetailCodeRequired}
}

func InvalidType(in string, field string) Detail {
	return Detail{In: in, Field: field, Code: DetailCodeInvalidType}
}

func UnknownField(field string) Detail {
	return Detail{In: InBody, Field: field, Code: DetailCodeUnknownField}
}

func MalformedJSON() Detail {
	return Detail{In: InBody, Field: "body", Code: DetailCodeMalformedJSON}
}

func TrailingData() Detail {
	return Detail{In: InBody, Field: "body", Code: DetailCodeTrailingData}
}

func MultipleValues(in string, field string) Detail {
	return Detail{In: in, Field: field, Code: DetailCodeMultipleValues}
}

func InvalidUUID(in string, field string) Detail {
	return Detail{In: in, Field: field, Code: DetailCodeInvalidUUID}
}

func InvalidInteger(in string, field string) Detail {
	return Detail{In: in, Field: field, Code: DetailCodeInvalidInteger}
}

func OutOfRange(in string, field string) Detail {
	return Detail{In: in, Field: field, Code: DetailCodeOutOfRange}
}

func InvalidValue(in string, field string) Detail {
	return Detail{In: in, Field: field, Code: DetailCodeInvalidValue}
}

func newProblem(statusCode int, details ...Detail) *Problem {
	return &Problem{
		StatusCode: statusCode,
		Details:    append([]Detail(nil), details...),
	}
}
