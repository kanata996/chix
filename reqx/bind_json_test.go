package reqx

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestBindBody_ContentTypeContract(t *testing.T) {
	type request struct {
		Name string `json:"name"`
	}

	t.Run("accepts application json", func(t *testing.T) {
		req := newJSONRequest(http.MethodPost, "/", `{"name":"kanata"}`)

		var dst request
		if err := BindBody(req, &dst); err != nil {
			t.Fatalf("BindBody() error = %v", err)
		}
		if dst.Name != "kanata" {
			t.Fatalf("name = %q, want kanata", dst.Name)
		}
	})

	t.Run("rejects missing content type for non empty body", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(`{"name":"kanata"}`))

		var dst request
		_ = assertHTTPError(
			t,
			BindBody(req, &dst),
			http.StatusUnsupportedMediaType,
			CodeUnsupportedMediaType,
			"Content-Type must be application/json",
		)
	})

	t.Run("rejects unsupported media type", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(`{"name":"kanata"}`))
		req.Header.Set("Content-Type", "text/plain")

		var dst request
		_ = assertHTTPError(
			t,
			BindBody(req, &dst),
			http.StatusUnsupportedMediaType,
			CodeUnsupportedMediaType,
			"Content-Type must be application/json",
		)
	})

	t.Run("rejects application json suffix media type", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(`{"name":"kanata"}`))
		req.Header.Set("Content-Type", "application/problem+json")

		var dst request
		_ = assertHTTPError(
			t,
			BindBody(req, &dst),
			http.StatusUnsupportedMediaType,
			CodeUnsupportedMediaType,
			"Content-Type must be application/json",
		)
	})

	t.Run("rejects xml content type", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(`<request><name>kanata</name></request>`))
		req.Header.Set("Content-Type", "application/xml")

		var dst request
		_ = assertHTTPError(
			t,
			BindBody(req, &dst),
			http.StatusUnsupportedMediaType,
			CodeUnsupportedMediaType,
			"Content-Type must be application/json",
		)
	})

	t.Run("rejects form content type", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(`name=kanata`))
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

		var dst request
		_ = assertHTTPError(
			t,
			BindBody(req, &dst),
			http.StatusUnsupportedMediaType,
			CodeUnsupportedMediaType,
			"Content-Type must be application/json",
		)
	})

	t.Run("rejects multipart content type", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(`--boundary`))
		req.Header.Set("Content-Type", "multipart/form-data; boundary=boundary")

		var dst request
		_ = assertHTTPError(
			t,
			BindBody(req, &dst),
			http.StatusUnsupportedMediaType,
			CodeUnsupportedMediaType,
			"Content-Type must be application/json",
		)
	})
}

func TestBindBody_EmptyBodyContract(t *testing.T) {
	type request struct {
		Name string `json:"name"`
	}

	t.Run("content length zero is a noop even with invalid content type", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(""))
		req.Header.Set("Content-Type", "text/plain")
		req.ContentLength = 0

		dst := request{Name: "kanata"}
		if err := BindBody(req, &dst); err != nil {
			t.Fatalf("BindBody() error = %v", err)
		}
		if dst.Name != "kanata" {
			t.Fatalf("name = %q, want kanata", dst.Name)
		}
	})

	t.Run("whitespace body is not treated as empty when content length is non zero", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(" \n\t "))
		req.Header.Set("Content-Type", mimeApplicationJSON)
		req.ContentLength = int64(len(" \n\t "))

		dst := request{Name: "kanata"}
		_ = assertHTTPError(t, BindBody(req, &dst), http.StatusBadRequest, CodeInvalidJSON, "request body must be valid JSON")
		if dst.Name != "kanata" {
			t.Fatalf("name = %q, want kanata", dst.Name)
		}
	})
}

func TestBindBody_JSONContract(t *testing.T) {
	t.Run("top level null follows default decoder semantics", func(t *testing.T) {
		type request struct {
			Name string `json:"name"`
		}

		req := newJSONRequest(http.MethodPost, "/", `null`)
		var dst request
		if err := BindBody(req, &dst); err != nil {
			t.Fatalf("BindBody() error = %v", err)
		}
		if dst.Name != "" {
			t.Fatalf("name = %q, want empty", dst.Name)
		}
	})

	t.Run("array binds to slice target", func(t *testing.T) {
		type item struct {
			Name string `json:"name"`
		}

		req := newJSONRequest(http.MethodPost, "/", `[{"name":"a"},{"name":"b"}]`)
		var dst []item
		if err := BindBody(req, &dst); err != nil {
			t.Fatalf("BindBody() error = %v", err)
		}
		if len(dst) != 2 || dst[0].Name != "a" || dst[1].Name != "b" {
			t.Fatalf("dst = %#v", dst)
		}
	})

	t.Run("type mismatch returns bad request", func(t *testing.T) {
		type request struct {
			Age int `json:"age"`
		}

		req := newJSONRequest(http.MethodPost, "/", `{"age":"oops"}`)
		dst := request{Age: 7}
		_ = assertHTTPError(t, BindBody(req, &dst), http.StatusBadRequest, CodeInvalidJSON, "request body must be valid JSON")
	})
}

func TestBindBody_RequestTooLarge(t *testing.T) {
	type request struct {
		Name string `json:"name"`
	}

	oversizedName := strings.Repeat("a", int(defaultMaxBodyBytes))
	body := []byte(`{"name":"` + oversizedName + `"}`)
	req := httptest.NewRequest(http.MethodPost, "/", bytes.NewReader(body))
	req.Header.Set("Content-Type", mimeApplicationJSON)
	req.ContentLength = int64(len(body))

	var dst request
	_ = assertHTTPError(t, BindBody(req, &dst), http.StatusRequestEntityTooLarge, CodeRequestTooLarge, "request body is too large")
}

func TestBindAndValidateBody_Layering(t *testing.T) {
	type request struct {
		Name string `json:"name" validate:"required"`
	}

	t.Run("empty body noops then validate runs", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(""))
		req.Header.Set("Content-Type", mimeApplicationJSON)
		req.ContentLength = 0

		var dst request
		err := BindAndValidateBody(req, &dst)
		httpErr := assertHTTPError(t, err, http.StatusUnprocessableEntity, CodeInvalidRequest, "request contains invalid fields")
		if got := len(httpErr.Errors()); got != 1 {
			t.Fatalf("errors len = %d, want 1", got)
		}
	})
}
