package resp

import (
	"net/http"
	"strings"
)

// HTTPError 表示 HTTP 边界上的公共错误。
// cause 仅用于内部保留原始错误链，真正返回给客户端的是 status/code/message/details。
type HTTPError struct {
	status  int
	code    string
	message string
	details []any
	cause   error
}

// NewError 构造一个不带底层 cause 的公共 HTTP 错误。
func NewError(status int, code, message string, details ...any) *HTTPError {
	return wrapError(status, code, message, nil, details...)
}

// wrapError 基于给定 cause 构造公共 HTTP 错误。
// 非法状态码会被归一化，缺省 code/message 也会按状态码补全。
func wrapError(status int, code, message string, cause error, details ...any) *HTTPError {
	status = normalizeErrorStatus(status)
	return &HTTPError{
		status:  status,
		code:    normalizeErrorCode(status, code),
		message: normalizeErrorMessage(status, message),
		details: cloneDetails(details),
		cause:   cause,
	}
}

func (e *HTTPError) Error() string {
	if e == nil {
		return ""
	}
	if e.cause != nil {
		return e.cause.Error()
	}
	return e.message
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

func (e *HTTPError) Message() string {
	if e == nil {
		return normalizeErrorMessage(http.StatusInternalServerError, "")
	}
	return normalizeErrorMessage(e.Status(), e.message)
}

func (e *HTTPError) Details() []any {
	if e == nil || len(e.details) == 0 {
		return nil
	}
	return cloneDetails(e.details)
}

func BadRequest(code, message string, details ...any) *HTTPError {
	return NewError(http.StatusBadRequest, code, message, details...)
}

func Unauthorized(code, message string, details ...any) *HTTPError {
	return NewError(http.StatusUnauthorized, code, message, details...)
}

func Forbidden(code, message string, details ...any) *HTTPError {
	return NewError(http.StatusForbidden, code, message, details...)
}

func NotFound(code, message string, details ...any) *HTTPError {
	return NewError(http.StatusNotFound, code, message, details...)
}

func MethodNotAllowed(code, message string, details ...any) *HTTPError {
	return NewError(http.StatusMethodNotAllowed, code, message, details...)
}

func Conflict(code, message string, details ...any) *HTTPError {
	return NewError(http.StatusConflict, code, message, details...)
}

func UnprocessableEntity(code, message string, details ...any) *HTTPError {
	return NewError(http.StatusUnprocessableEntity, code, message, details...)
}

func TooManyRequests(code, message string, details ...any) *HTTPError {
	return NewError(http.StatusTooManyRequests, code, message, details...)
}

// cloneDetails 返回 details 的浅拷贝，避免调用方后续修改影响已构造的错误对象。
func cloneDetails(details []any) []any {
	if len(details) == 0 {
		return nil
	}
	cloned := make([]any, len(details))
	copy(cloned, details)
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

// normalizeErrorMessage 根据状态码补齐默认公共错误消息。
func normalizeErrorMessage(status int, message string) string {
	if trimmed := strings.TrimSpace(message); trimmed != "" {
		return trimmed
	}
	if text := http.StatusText(status); text != "" {
		return text
	}
	return http.StatusText(http.StatusInternalServerError)
}
