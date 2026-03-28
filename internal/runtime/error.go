// 本文件职责：定义公开错误载体、runtime 自产错误以及错误归一化辅助。
// 定位：作为 runtime error model 的基础层，被 failure 执行路径直接消费。
package runtime

import (
	"errors"
	"net/http"
)

// HTTPError 表示写回客户端的公开错误载体；Status 仅用于响应状态码，不参与 JSON 输出。
type HTTPError struct {
	Status  int    `json:"-"`
	Code    string `json:"code"`
	Message string `json:"message"`
	Details []any  `json:"details"`
}

// ErrorMapper 将原始错误映射为公开 HTTP 错误；返回 nil 表示当前 mapper 不处理。
type ErrorMapper func(error) *HTTPError

// 约定 runtime 内部错误可直接提供公开错误表示。
type runtimePublicError interface {
	error
	publicError() *HTTPError
}

// 表示请求形状错误，并保留原始原因以维持错误链。
type requestShapeError struct {
	cause error
}

// 表示请求媒体类型不受支持，并保留原始原因以维持错误链。
type unsupportedMediaTypeError struct {
	cause error
}

// 表示应公开为 422 的无效请求错误，并携带可直接写回的 details。
type invalidRequestError struct {
	details []any
}

// Error 返回可读错误字符串，优先使用 Message，其次使用 Code。
func (e *HTTPError) Error() string {
	if e == nil {
		return ""
	}
	if e.Message != "" {
		return e.Message
	}
	if e.Code != "" {
		return e.Code
	}
	return "http error"
}

// Error 返回底层原因的错误文本；缺失时回退为 bad request。
func (e *requestShapeError) Error() string {
	if e == nil || e.cause == nil {
		return "bad request"
	}
	return e.cause.Error()
}

// Unwrap 返回原始原因，便于通过 errors.Is/As 继续识别错误链。
func (e *requestShapeError) Unwrap() error {
	if e == nil {
		return nil
	}
	return e.cause
}

// publicError 返回对应的公开 bad request 错误。
func (e *requestShapeError) publicError() *HTTPError {
	return badRequestHTTPError()
}

// Error 返回底层原因的错误文本；缺失时回退为 unsupported media type。
func (e *unsupportedMediaTypeError) Error() string {
	if e == nil || e.cause == nil {
		return "unsupported media type"
	}
	return e.cause.Error()
}

// Unwrap 返回原始原因，便于通过 errors.Is/As 继续识别错误链。
func (e *unsupportedMediaTypeError) Unwrap() error {
	if e == nil {
		return nil
	}
	return e.cause
}

// publicError 返回对应的公开 unsupported media type 错误。
func (e *unsupportedMediaTypeError) publicError() *HTTPError {
	return unsupportedMediaTypeHTTPError()
}

// Error 返回固定的无效请求描述。
func (e *invalidRequestError) Error() string {
	return "invalid request"
}

// publicError 返回携带 details 的公开 422 错误，并复制 details 以隔离后续修改。
func (e *invalidRequestError) publicError() *HTTPError {
	return &HTTPError{
		Status:  http.StatusUnprocessableEntity,
		Code:    "invalid_request",
		Message: "invalid request",
		Details: append([]any(nil), e.details...),
	}
}

// newRequestShapeError 包装请求形状错误并保留原始原因。
func newRequestShapeError(cause error) error {
	return &requestShapeError{cause: cause}
}

// newUnsupportedMediaTypeError 包装媒体类型错误并保留原始原因。
func newUnsupportedMediaTypeError(cause error) error {
	return &unsupportedMediaTypeError{cause: cause}
}

// newInvalidRequestError 创建无效请求错误，并复制 details 以避免共享底层切片。
func newInvalidRequestError(details []any) error {
	return &invalidRequestError{details: append([]any(nil), details...)}
}

// publicErrorFromRuntime 从 runtime 内部错误中提取公开错误，并返回归一化后的副本。
func publicErrorFromRuntime(err error) *HTTPError {
	var runtimeErr runtimePublicError
	if errors.As(err, &runtimeErr) {
		return normalizeHTTPError(runtimeErr.publicError())
	}
	return nil
}

// publicErrorFromValue 从错误链中提取 *HTTPError，并返回归一化后的副本。
func publicErrorFromValue(err error) *HTTPError {
	var httpErr *HTTPError
	if errors.As(err, &httpErr) {
		return normalizeHTTPError(httpErr)
	}
	return nil
}

// normalizeHTTPError 补齐默认字段并复制输入，避免共享可变的 details 切片。
func normalizeHTTPError(err *HTTPError) *HTTPError {
	if err == nil {
		return internalHTTPError()
	}

	normalized := *err
	if normalized.Status == 0 {
		normalized.Status = http.StatusInternalServerError
	}
	if normalized.Code == "" {
		normalized.Code = defaultErrorCode(normalized.Status)
	}
	if normalized.Message == "" {
		normalized.Message = defaultErrorMessage(normalized.Status)
	}
	if normalized.Details == nil {
		normalized.Details = []any{}
	} else {
		normalized.Details = append([]any{}, normalized.Details...)
	}
	return &normalized
}

// defaultErrorCode 返回给定状态码的默认错误码。
func defaultErrorCode(status int) string {
	switch status {
	case http.StatusBadRequest:
		return "bad_request"
	case http.StatusUnsupportedMediaType:
		return "unsupported_media_type"
	case http.StatusUnprocessableEntity:
		return "invalid_request"
	case http.StatusInternalServerError:
		return "internal_error"
	default:
		return "http_error"
	}
}

// defaultErrorMessage 返回给定状态码的默认错误消息；未知状态码优先使用标准状态文本。
func defaultErrorMessage(status int) string {
	switch status {
	case http.StatusBadRequest:
		return "bad request"
	case http.StatusUnsupportedMediaType:
		return "unsupported media type"
	case http.StatusUnprocessableEntity:
		return "invalid request"
	case http.StatusInternalServerError:
		return "internal server error"
	default:
		if text := http.StatusText(status); text != "" {
			return text
		}
		return "http error"
	}
}

// badRequestHTTPError 返回固定的 400 公开错误。
func badRequestHTTPError() *HTTPError {
	return &HTTPError{
		Status:  http.StatusBadRequest,
		Code:    "bad_request",
		Message: "bad request",
		Details: []any{},
	}
}

// unsupportedMediaTypeHTTPError 返回固定的 415 公开错误。
func unsupportedMediaTypeHTTPError() *HTTPError {
	return &HTTPError{
		Status:  http.StatusUnsupportedMediaType,
		Code:    "unsupported_media_type",
		Message: "unsupported media type",
		Details: []any{},
	}
}

// internalHTTPError 返回固定的 500 公开错误。
func internalHTTPError() *HTTPError {
	return &HTTPError{
		Status:  http.StatusInternalServerError,
		Code:    "internal_error",
		Message: "internal server error",
		Details: []any{},
	}
}
