package paramx

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/kanata996/chix/reqx"
)

const (
	headerUpperUUID = "A2B5AA07-F5E5-4C6D-921D-00F13ED4E580"
	headerUUID      = "a2b5aa07-f5e5-4c6d-921d-00f13ed4e580"
)

func TestHeaderReader(t *testing.T) {
	t.Run("string missing", func(t *testing.T) {
		value, ok, err := Header(httptest.NewRequest(http.MethodGet, "/", nil)).String("X-Request-ID")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if ok || value != "" {
			t.Fatalf("unexpected result: value=%q ok=%v", value, ok)
		}
	})

	t.Run("string trims and matches case-insensitively", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		req.Header.Set("X-Request-ID", "  req-123  ")

		value, ok, err := Header(req).String("x-request-id")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !ok || value != "req-123" {
			t.Fatalf("unexpected result: value=%q ok=%v", value, ok)
		}
	})

	t.Run("string keeps present empty", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		req.Header.Set("X-Request-ID", "   ")

		value, ok, err := Header(req).String("X-Request-ID")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !ok || value != "" {
			t.Fatalf("unexpected result: value=%q ok=%v", value, ok)
		}
	})

	t.Run("string multiple values", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		req.Header.Add("X-Request-ID", "a")
		req.Header.Add("X-Request-ID", "b")

		_, _, err := Header(req).String("X-Request-ID")
		assertProblemDetail(t, err, reqx.InHeader, "X-Request-ID", reqx.DetailCodeMultipleValues)
	})

	t.Run("required string rejects empty", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		req.Header.Set("X-Trace-ID", "  ")

		_, err := Header(req).RequiredString("X-Trace-ID")
		assertProblemDetail(t, err, reqx.InHeader, "X-Trace-ID", reqx.DetailCodeRequired)
	})

	t.Run("strings use repeated headers", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		req.Header.Add("X-Tag", " a ")
		req.Header.Add("X-Tag", "b")

		values, ok, err := Header(req).Strings("X-Tag")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !ok {
			t.Fatal("expected values")
		}
		assertStrings(t, values, []string{"a", "b"})
	})

	t.Run("strings reject blank item", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		req.Header.Add("X-Tag", "a")
		req.Header.Add("X-Tag", " ")

		_, _, err := Header(req).Strings("X-Tag")
		assertProblemDetail(t, err, reqx.InHeader, "X-Tag", reqx.DetailCodeInvalidValue)
	})

	t.Run("int parses value", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		req.Header.Set("X-Limit", " 42 ")

		value, ok, err := Header(req).Int("X-Limit")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !ok || value != 42 {
			t.Fatalf("unexpected result: value=%d ok=%v", value, ok)
		}
	})

	t.Run("int invalid format", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		req.Header.Set("X-Limit", "bad")

		_, _, err := Header(req).Int("X-Limit")
		assertProblemDetail(t, err, reqx.InHeader, "X-Limit", reqx.DetailCodeInvalidInteger)
	})

	t.Run("uuid canonicalizes value", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		req.Header.Set("X-Resource-UUID", " "+headerUpperUUID+" ")

		value, ok, err := Header(req).UUID("X-Resource-UUID")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !ok || value != headerUUID {
			t.Fatalf("unexpected result: value=%q ok=%v", value, ok)
		}
	})

	t.Run("uuid invalid format", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		req.Header.Set("X-Resource-UUID", "bad")

		_, _, err := Header(req).UUID("X-Resource-UUID")
		assertProblemDetail(t, err, reqx.InHeader, "X-Resource-UUID", reqx.DetailCodeInvalidUUID)
	})

	t.Run("bool parses true and false", func(t *testing.T) {
		reqTrue := httptest.NewRequest(http.MethodGet, "/", nil)
		reqTrue.Header.Set("X-Enabled", "true")

		gotTrue, ok, err := Header(reqTrue).Bool("X-Enabled")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !ok || !gotTrue {
			t.Fatalf("unexpected result: value=%v ok=%v", gotTrue, ok)
		}

		reqFalse := httptest.NewRequest(http.MethodGet, "/", nil)
		reqFalse.Header.Set("X-Enabled", "false")

		gotFalse, ok, err := Header(reqFalse).Bool("X-Enabled")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !ok || gotFalse {
			t.Fatalf("unexpected result: value=%v ok=%v", gotFalse, ok)
		}
	})

	t.Run("bool rejects invalid value", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		req.Header.Set("X-Enabled", "TRUE")

		_, _, err := Header(req).Bool("X-Enabled")
		assertProblemDetail(t, err, reqx.InHeader, "X-Enabled", reqx.DetailCodeInvalidValue)
	})

	t.Run("nil request is programming error", func(t *testing.T) {
		_, _, err := Header(nil).String("X-Request-ID")
		assertPlainErrorMessage(t, err, "paramx: request must not be nil")
	})

	t.Run("blank name is programming error", func(t *testing.T) {
		_, _, err := Header(httptest.NewRequest(http.MethodGet, "/", nil)).String("   ")
		assertPlainErrorMessage(t, err, "paramx: name must not be blank")
	})
}
