package resp

import (
	"net/http"
	"strings"
)

// HTTPError 表示 HTTP 边界上的公共错误。
// cause 仅用于内部保留原始错误链，真正返回给客户端的是
// title/status/detail/code/errors。
type HTTPError struct {
	status int
	code   string
	detail string
	errors []any
	cause  error
}

// NewError 构造一个不带底层 cause 的公共 HTTP 错误。
func NewError(status int, code, detail string, errors ...any) *HTTPError {
	return wrapError(status, code, detail, nil, errors...)
}

// wrapError 基于给定 cause 构造公共 HTTP 错误。
// 非法状态码会被归一化，缺省 code/detail 也会按状态码补全。
func wrapError(status int, code, detail string, cause error, errors ...any) *HTTPError {
	status = normalizeErrorStatus(status)
	return &HTTPError{
		status: status,
		code:   normalizeErrorCode(status, code),
		detail: normalizeErrorDetail(status, detail),
		errors: cloneErrors(errors),
		cause:  cause,
	}
}

func (e *HTTPError) Error() string {
	if e == nil {
		return ""
	}
	if e.cause != nil {
		return e.cause.Error()
	}
	return e.detail
}

func (e *HTTPError) Unwrap() error {
	if e == nil {
		return nil
	}
	return e.cause
}

func (e *HTTPError) Status() int {
	if e == nil {
		return http.StatusInternalServerError
	}
	return normalizeErrorStatus(e.status)
}

func (e *HTTPError) Code() string {
	if e == nil {
		return normalizeErrorCode(http.StatusInternalServerError, "")
	}
	return normalizeErrorCode(e.Status(), e.code)
}

func (e *HTTPError) Title() string {
	if e == nil {
		return normalizeErrorTitle(http.StatusInternalServerError)
	}
	return normalizeErrorTitle(e.Status())
}

func (e *HTTPError) Detail() string {
	if e == nil {
		return normalizeErrorDetail(http.StatusInternalServerError, "")
	}
	return normalizeErrorDetail(e.Status(), e.detail)
}

func (e *HTTPError) Errors() []any {
	if e == nil || len(e.errors) == 0 {
		return nil
	}
	return cloneErrors(e.errors)
}

// Message 保留为 Detail 的兼容别名。
func (e *HTTPError) Message() string {
	return e.Detail()
}

// Details 保留为 Errors 的兼容别名。
func (e *HTTPError) Details() []any {
	return e.Errors()
}

func BadRequest(code, detail string, errors ...any) *HTTPError {
	return NewError(http.StatusBadRequest, code, detail, errors...)
}

func Unauthorized(code, detail string, errors ...any) *HTTPError {
	return NewError(http.StatusUnauthorized, code, detail, errors...)
}

func Forbidden(code, detail string, errors ...any) *HTTPError {
	return NewError(http.StatusForbidden, code, detail, errors...)
}

func NotFound(code, detail string, errors ...any) *HTTPError {
	return NewError(http.StatusNotFound, code, detail, errors...)
}

func MethodNotAllowed(code, detail string, errors ...any) *HTTPError {
	return NewError(http.StatusMethodNotAllowed, code, detail, errors...)
}

func Conflict(code, detail string, errors ...any) *HTTPError {
	return NewError(http.StatusConflict, code, detail, errors...)
}

func UnprocessableEntity(code, detail string, errors ...any) *HTTPError {
	return NewError(http.StatusUnprocessableEntity, code, detail, errors...)
}

func TooManyRequests(code, detail string, errors ...any) *HTTPError {
	return NewError(http.StatusTooManyRequests, code, detail, errors...)
}

// cloneErrors 返回 errors 的浅拷贝，避免调用方后续修改影响已构造的错误对象。
func cloneErrors(errors []any) []any {
	if len(errors) == 0 {
		return nil
	}
	cloned := make([]any, len(errors))
	copy(cloned, errors)
	return cloned
}

// normalizeErrorStatus 把非法或越界状态码收敛到 500，避免下游 WriteHeader panic。
func normalizeErrorStatus(status int) int {
	if status < 400 || status > 599 {
		return http.StatusInternalServerError
	}
	return status
}

// normalizeErrorCode 根据状态码补齐默认机器可读错误码。
func normalizeErrorCode(status int, code string) string {
	if trimmed := strings.TrimSpace(code); trimmed != "" {
		return trimmed
	}

	switch status {
	case http.StatusBadRequest:
		return "bad_request"
	case http.StatusUnauthorized:
		return "unauthorized"
	case http.StatusForbidden:
		return "forbidden"
	case http.StatusNotFound:
		return "not_found"
	case http.StatusMethodNotAllowed:
		return "method_not_allowed"
	case http.StatusConflict:
		return "conflict"
	case http.StatusUnprocessableEntity:
		return "unprocessable_entity"
	case http.StatusTooManyRequests:
		return "too_many_requests"
	case http.StatusServiceUnavailable:
		return "service_unavailable"
	case http.StatusGatewayTimeout:
		return "timeout"
	default:
		if status >= 500 {
			return "internal_error"
		}
		return "client_error"
	}
}

// normalizeErrorTitle 根据状态码生成公开错误标题。
func normalizeErrorTitle(status int) string {
	status = normalizeErrorStatus(status)
	switch status {
	case 499:
		return "Client Closed Request"
	}
	if text := http.StatusText(status); text != "" {
		return text
	}
	return http.StatusText(http.StatusInternalServerError)
}

// normalizeErrorDetail 根据状态码补齐默认公共错误详情。
func normalizeErrorDetail(status int, detail string) string {
	if trimmed := strings.TrimSpace(detail); trimmed != "" {
		return trimmed
	}
	return normalizeErrorTitle(status)
}

// normalizeErrorMessage 保留为 normalizeErrorDetail 的兼容别名。
func normalizeErrorMessage(status int, message string) string {
	return normalizeErrorDetail(status, message)
}
