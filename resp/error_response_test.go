package resp

import (
	"errors"
	"net/http"
	"testing"
)

// nil 的 HTTPError 接收者应返回一组安全默认值，避免调用方二次判空。
func TestHTTPErrorNilReceiverUsesSafeDefaults(t *testing.T) {
	var err *HTTPError

	if got := err.Error(); got != "" {
		t.Fatalf("Error() = %q, want empty", got)
	}
	if got := err.Unwrap(); got != nil {
		t.Fatalf("Unwrap() = %v, want nil", got)
	}
	if got := err.Status(); got != http.StatusInternalServerError {
		t.Fatalf("Status() = %d, want %d", got, http.StatusInternalServerError)
	}
	if got := err.Code(); got != "internal_error" {
		t.Fatalf("Code() = %q, want internal_error", got)
	}
	if got := err.Message(); got != http.StatusText(http.StatusInternalServerError) {
		t.Fatalf("Message() = %q, want %q", got, http.StatusText(http.StatusInternalServerError))
	}
	if got := err.Details(); got != nil {
		t.Fatalf("Details() = %#v, want nil", got)
	}
}

// HTTPError 会优先暴露底层 cause，并对 details 做防御性拷贝。
func TestHTTPErrorUsesCauseAndClonesDetails(t *testing.T) {
	cause := errors.New("db timeout")
	err := wrapError(http.StatusConflict, "", "", cause, "detail")

	if got := err.Error(); got != cause.Error() {
		t.Fatalf("Error() = %q, want %q", got, cause.Error())
	}
	if got := err.Unwrap(); !errors.Is(got, cause) {
		t.Fatalf("Unwrap() = %v, want %v", got, cause)
	}
	if got := err.Status(); got != http.StatusConflict {
		t.Fatalf("Status() = %d, want %d", got, http.StatusConflict)
	}
	if got := err.Code(); got != "conflict" {
		t.Fatalf("Code() = %q, want conflict", got)
	}
	if got := err.Message(); got != http.StatusText(http.StatusConflict) {
		t.Fatalf("Message() = %q, want %q", got, http.StatusText(http.StatusConflict))
	}

	details := err.Details()
	if len(details) != 1 || details[0] != "detail" {
		t.Fatalf("Details() = %#v, want [detail]", details)
	}
	details[0] = "changed"
	if got := err.Details()[0]; got != "detail" {
		t.Fatalf("Details() after mutation = %#v, want detail", got)
	}
}

// 没有 cause 时，HTTPError.Error 会回退为公开消息本身。
func TestHTTPErrorErrorReturnsMessageWithoutCause(t *testing.T) {
	err := NewError(http.StatusBadRequest, "bad_request", "bad request")

	if got := err.Error(); got != "bad request" {
		t.Fatalf("Error() = %q, want %q", got, "bad request")
	}
}

// 各个常用错误构造器都会生成稳定的状态码、错误码和公开消息。
func TestHTTPErrorConstructors(t *testing.T) {
	testCases := []struct {
		name       string
		build      func() *HTTPError
		wantStatus int
		wantCode   string
		wantMsg    string
	}{
		{
			name:       "bad request",
			build:      func() *HTTPError { return BadRequest("", "", "detail") },
			wantStatus: http.StatusBadRequest,
			wantCode:   "bad_request",
			wantMsg:    http.StatusText(http.StatusBadRequest),
		},
		{
			name:       "unauthorized",
			build:      func() *HTTPError { return Unauthorized("", "", "detail") },
			wantStatus: http.StatusUnauthorized,
			wantCode:   "unauthorized",
			wantMsg:    http.StatusText(http.StatusUnauthorized),
		},
		{
			name:       "forbidden",
			build:      func() *HTTPError { return Forbidden("", "", "detail") },
			wantStatus: http.StatusForbidden,
			wantCode:   "forbidden",
			wantMsg:    http.StatusText(http.StatusForbidden),
		},
		{
			name:       "not found",
			build:      func() *HTTPError { return NotFound("", "", "detail") },
			wantStatus: http.StatusNotFound,
			wantCode:   "not_found",
			wantMsg:    http.StatusText(http.StatusNotFound),
		},
		{
			name:       "method not allowed",
			build:      func() *HTTPError { return MethodNotAllowed("", "", "detail") },
			wantStatus: http.StatusMethodNotAllowed,
			wantCode:   "method_not_allowed",
			wantMsg:    http.StatusText(http.StatusMethodNotAllowed),
		},
		{
			name:       "conflict",
			build:      func() *HTTPError { return Conflict("", "", "detail") },
			wantStatus: http.StatusConflict,
			wantCode:   "conflict",
			wantMsg:    http.StatusText(http.StatusConflict),
		},
		{
			name:       "unprocessable entity",
			build:      func() *HTTPError { return UnprocessableEntity("", "", "detail") },
			wantStatus: http.StatusUnprocessableEntity,
			wantCode:   "unprocessable_entity",
			wantMsg:    http.StatusText(http.StatusUnprocessableEntity),
		},
		{
			name:       "too many requests",
			build:      func() *HTTPError { return TooManyRequests("", "", "detail") },
			wantStatus: http.StatusTooManyRequests,
			wantCode:   "too_many_requests",
			wantMsg:    http.StatusText(http.StatusTooManyRequests),
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			err := tc.build()
			if got := err.Status(); got != tc.wantStatus {
				t.Fatalf("Status() = %d, want %d", got, tc.wantStatus)
			}
			if got := err.Code(); got != tc.wantCode {
				t.Fatalf("Code() = %q, want %q", got, tc.wantCode)
			}
			if got := err.Message(); got != tc.wantMsg {
				t.Fatalf("Message() = %q, want %q", got, tc.wantMsg)
			}
			if got := err.Details(); len(got) != 1 || got[0] != "detail" {
				t.Fatalf("Details() = %#v, want [detail]", got)
			}
		})
	}
}

// 错误码标准化会裁剪显式值，并按状态码回退到约定错误码。
func TestNormalizeErrorCode(t *testing.T) {
	testCases := []struct {
		name   string
		status int
		code   string
		want   string
	}{
		{name: "explicit", status: http.StatusBadRequest, code: " invalid_json ", want: "invalid_json"},
		{name: "bad request", status: http.StatusBadRequest, want: "bad_request"},
		{name: "unauthorized", status: http.StatusUnauthorized, want: "unauthorized"},
		{name: "forbidden", status: http.StatusForbidden, want: "forbidden"},
		{name: "not found", status: http.StatusNotFound, want: "not_found"},
		{name: "method not allowed", status: http.StatusMethodNotAllowed, want: "method_not_allowed"},
		{name: "conflict", status: http.StatusConflict, want: "conflict"},
		{name: "unprocessable entity", status: http.StatusUnprocessableEntity, want: "unprocessable_entity"},
		{name: "too many requests", status: http.StatusTooManyRequests, want: "too_many_requests"},
		{name: "service unavailable", status: http.StatusServiceUnavailable, want: "service_unavailable"},
		{name: "gateway timeout", status: http.StatusGatewayTimeout, want: "timeout"},
		{name: "internal error", status: http.StatusInternalServerError, want: "internal_error"},
		{name: "other client error", status: http.StatusTeapot, want: "client_error"},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			if got := normalizeErrorCode(tc.status, tc.code); got != tc.want {
				t.Fatalf("normalizeErrorCode(%d, %q) = %q, want %q", tc.status, tc.code, got, tc.want)
			}
		})
	}
}

// 错误消息标准化会优先使用显式消息，否则回退到合适的状态文本。
func TestNormalizeErrorMessage(t *testing.T) {
	testCases := []struct {
		name    string
		status  int
		message string
		want    string
	}{
		{name: "explicit", status: http.StatusBadRequest, message: " custom ", want: "custom"},
		{name: "status text", status: http.StatusBadRequest, want: http.StatusText(http.StatusBadRequest)},
		{name: "fallback internal server error", status: 777, want: http.StatusText(http.StatusInternalServerError)},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			if got := normalizeErrorMessage(tc.status, tc.message); got != tc.want {
				t.Fatalf("normalizeErrorMessage(%d, %q) = %q, want %q", tc.status, tc.message, got, tc.want)
			}
		})
	}
}
