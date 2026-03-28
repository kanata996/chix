// 本文件职责：定义公开错误载体、runtime 自产错误以及 JSON envelope 编码辅助。
// 定位：作为 runtime failure 模型的基础层，被 failure 执行路径直接消费。
package runtime

import (
	"encoding/json"
	"errors"
	"net/http"
)

type HTTPError struct {
	Status  int    `json:"-"`
	Code    string `json:"code"`
	Message string `json:"message"`
	Details []any  `json:"details"`
}

type ErrorMapper func(error) *HTTPError

type runtimePublicError interface {
	error
	publicError() *HTTPError
}

type requestShapeError struct {
	cause error
}

type unsupportedMediaTypeError struct {
	cause error
}

type invalidRequestError struct {
	details []any
}

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

func (e *requestShapeError) Error() string {
	if e == nil || e.cause == nil {
		return "bad request"
	}
	return e.cause.Error()
}

func (e *requestShapeError) Unwrap() error {
	if e == nil {
		return nil
	}
	return e.cause
}

func (e *requestShapeError) publicError() *HTTPError {
	return badRequestHTTPError()
}

func (e *unsupportedMediaTypeError) Error() string {
	if e == nil || e.cause == nil {
		return "unsupported media type"
	}
	return e.cause.Error()
}

func (e *unsupportedMediaTypeError) Unwrap() error {
	if e == nil {
		return nil
	}
	return e.cause
}

func (e *unsupportedMediaTypeError) publicError() *HTTPError {
	return unsupportedMediaTypeHTTPError()
}

func (e *invalidRequestError) Error() string {
	return "invalid request"
}

func (e *invalidRequestError) publicError() *HTTPError {
	return &HTTPError{
		Status:  http.StatusUnprocessableEntity,
		Code:    "invalid_request",
		Message: "invalid request",
		Details: append([]any(nil), e.details...),
	}
}

func newRequestShapeError(cause error) error {
	return &requestShapeError{cause: cause}
}

func newUnsupportedMediaTypeError(cause error) error {
	return &unsupportedMediaTypeError{cause: cause}
}

func newInvalidRequestError(details []any) error {
	return &invalidRequestError{details: append([]any(nil), details...)}
}

func publicErrorFromRuntime(err error) *HTTPError {
	var runtimeErr runtimePublicError
	if errors.As(err, &runtimeErr) {
		return normalizeHTTPError(runtimeErr.publicError())
	}
	return nil
}

func publicErrorFromValue(err error) *HTTPError {
	var httpErr *HTTPError
	if errors.As(err, &httpErr) {
		return normalizeHTTPError(httpErr)
	}
	return nil
}

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

func badRequestHTTPError() *HTTPError {
	return &HTTPError{
		Status:  http.StatusBadRequest,
		Code:    "bad_request",
		Message: "bad request",
		Details: []any{},
	}
}

func unsupportedMediaTypeHTTPError() *HTTPError {
	return &HTTPError{
		Status:  http.StatusUnsupportedMediaType,
		Code:    "unsupported_media_type",
		Message: "unsupported media type",
		Details: []any{},
	}
}

func internalHTTPError() *HTTPError {
	return &HTTPError{
		Status:  http.StatusInternalServerError,
		Code:    "internal_error",
		Message: "internal server error",
		Details: []any{},
	}
}

func marshalSuccessEnvelope(value any) ([]byte, error) {
	return json.Marshal(struct {
		Data any `json:"data"`
	}{
		Data: value,
	})
}

func marshalErrorEnvelope(public *HTTPError) ([]byte, error) {
	return json.Marshal(struct {
		Error *HTTPError `json:"error"`
	}{
		Error: normalizeHTTPError(public),
	})
}

func writeJSONResponse(w http.ResponseWriter, status int, payload []byte) error {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_, err := w.Write(payload)
	return err
}
