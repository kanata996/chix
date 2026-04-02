package resp

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
)

// responseWriteError 在 nil 接收者和普通错误场景下都应提供稳定的错误语义。
func TestResponseWriteErrorMethods(t *testing.T) {
	var nilErr *responseWriteError
	if got := nilErr.Error(); got != "resp: write response failed" {
		t.Fatalf("nil Error() = %q", got)
	}
	if got := nilErr.Unwrap(); got != nil {
		t.Fatalf("nil Unwrap() = %v, want nil", got)
	}

	cause := errors.New("socket closed")
	err := &responseWriteError{cause: cause}
	if got := err.Error(); got != "resp: write response failed: socket closed" {
		t.Fatalf("Error() = %q", got)
	}
	if got := err.Unwrap(); !errors.Is(got, cause) {
		t.Fatalf("Unwrap() = %v, want %v", got, cause)
	}
}

// 底层写 JSON 字节时会拒绝空的 ResponseWriter。
func TestWriteJSONBytesRejectsNilWriter(t *testing.T) {
	err := writeJSONBytes(nil, http.StatusOK, []byte(`{"ok":true}`))
	if err == nil || err.Error() != "resp: response writer is nil" {
		t.Fatalf("writeJSONBytes() error = %v, want response writer is nil", err)
	}
}

// 底层写 JSON 字节时会校验 HTTP 状态码合法性。
func TestWriteJSONBytesRejectsInvalidStatus(t *testing.T) {
	rr := httptest.NewRecorder()

	err := writeJSONBytes(rr, 1000, []byte(`{"ok":true}`))
	if err == nil || err.Error() != "resp: invalid HTTP status 1000" {
		t.Fatalf("writeJSONBytes() error = %v, want invalid status", err)
	}
}

// 仅写状态码的辅助函数也会拒绝空的 ResponseWriter。
func TestWriteStatusRejectsNilWriter(t *testing.T) {
	err := writeStatus(nil, http.StatusNoContent)
	if err == nil || err.Error() != "resp: response writer is nil" {
		t.Fatalf("writeStatus() error = %v, want response writer is nil", err)
	}
}

// 仅写状态码的辅助函数会校验 HTTP 状态码合法性。
func TestWriteStatusRejectsInvalidStatus(t *testing.T) {
	rr := httptest.NewRecorder()

	err := writeStatus(rr, 1000)
	if err == nil || err.Error() != "resp: invalid HTTP status 1000" {
		t.Fatalf("writeStatus() error = %v, want invalid status", err)
	}
}

// JSON 编码阶段遇到不支持的值时直接返回编码错误。
func TestEncodeJSONRejectsUnsupportedValue(t *testing.T) {
	_, err := encodeJSON(make(chan int), "")
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}

// HTTP 状态码校验会接受标准范围并拒绝越界值。
func TestValidateHTTPStatus(t *testing.T) {
	if err := validateHTTPStatus(http.StatusOK); err != nil {
		t.Fatalf("validateHTTPStatus(200) error = %v", err)
	}

	testCases := []int{99, 1000}
	for _, status := range testCases {
		if err := validateHTTPStatus(status); err == nil {
			t.Fatalf("validateHTTPStatus(%d) expected error, got nil", status)
		}
	}
}
