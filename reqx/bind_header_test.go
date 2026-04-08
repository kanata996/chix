package reqx

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestBindHeaders_BindsSupportedScalarTypes(t *testing.T) {
	type request struct {
		RequestID string `header:"x-request-id"`
		Retry     int    `header:"x-retry"`
		Enabled   bool   `header:"x-enabled"`
	}

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("X-Request-Id", "req-123")
	req.Header.Set("X-Retry", "2")
	req.Header.Set("X-Enabled", "true")

	var dst request
	if err := BindHeaders(req, &dst); err != nil {
		t.Fatalf("BindHeaders() error = %v", err)
	}
	if dst.RequestID != "req-123" || dst.Retry != 2 || !dst.Enabled {
		t.Fatalf("dst = %#v, want bound header values", dst)
	}
}

func TestBindHeaders_HandlesTrimmedAndRepeatedKeys(t *testing.T) {
	t.Run("trimmed non canonical keys still bind", func(t *testing.T) {
		type request struct {
			RequestID string `header:"x-request-id"`
		}

		req := httptest.NewRequest(http.MethodGet, "/", nil)
		req.Header = http.Header{
			" x-request-id ": {"req-123"},
		}

		var dst request
		if err := BindHeaders(req, &dst); err != nil {
			t.Fatalf("BindHeaders() error = %v", err)
		}
		if dst.RequestID != "req-123" {
			t.Fatalf("request_id = %q, want req-123", dst.RequestID)
		}
	})

	t.Run("repeated scalar values pick the first value", func(t *testing.T) {
		type request struct {
			RequestID string `header:"x-request-id"`
		}

		req := httptest.NewRequest(http.MethodGet, "/", nil)
		req.Header.Add("X-Request-Id", "req-1")
		req.Header.Add("X-Request-Id", "req-2")

		var dst request
		if err := BindHeaders(req, &dst); err != nil {
			t.Fatalf("BindHeaders() error = %v", err)
		}
		if dst.RequestID != "req-1" {
			t.Fatalf("request_id = %q, want req-1", dst.RequestID)
		}
	})
}

func TestBindHeaders_BindingErrorsAreBadRequest(t *testing.T) {
	type request struct {
		Retry int `header:"x-retry"`
	}

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("X-Retry", "oops")

	var dst request
	_ = assertHTTPError(t, BindHeaders(req, &dst), http.StatusBadRequest, "bad_request", "Bad Request")
}

func TestBindAndValidateHeaders_ReturnsBindingErrorBeforeValidation(t *testing.T) {
	type request struct {
		Retry int `header:"x-retry" validate:"required"`
	}

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("X-Retry", "oops")

	var dst request
	_ = assertHTTPError(t, BindAndValidateHeaders(req, &dst), http.StatusBadRequest, "bad_request", "Bad Request")
}
