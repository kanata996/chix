package bind

import (
	"io"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"
)

// 测试清单：
// - 标记说明：[✓] 已核对且已有真实覆盖；[x] 尚未完成，不得作为验收依据。
// - [✓] `Bind` / `BindBody` 的基础契约：path/query/body 覆盖顺序、POST 跳过 query、empty-body no-op。
// - [✓] `BindBody` 的 JSON-only 契约：拒绝非 JSON Content-Type，非法 JSON 返回 `400 invalid_json`。
// - [✓] query/path/header 单源 binder 对不受支持的 scalar 指针目标保持 no-op，不误写目标值。
// - [✓] 本文件同时提供 `newRouteRequest`、`assertHTTPError`、`assertBindingError` 等测试支撑 helper，供其余 bind 测试文件复用。
func newRouteRequest(method, target string, path map[string]string, body io.Reader) *http.Request {
	req := httptest.NewRequest(method, target, body)
	if len(path) == 0 {
		return req
	}

	pattern := ""
	for name, value := range path {
		req.SetPathValue(name, value)
		pattern += "/{" + name + "}"
	}
	if pattern == "" {
		pattern = "/"
	}
	req.Pattern = pattern
	return req
}

func TestBindCoreBehavior(t *testing.T) {
	type request struct {
		ID   string `param:"id" query:"id" json:"id"`
		Name string `query:"name" json:"name"`
	}

	t.Run("get path query body ordering", func(t *testing.T) {
		req := newRouteRequest(http.MethodGet, "/items?id=query-id&name=query-name", map[string]string{"id": "route-id"}, stringsNewReader(`{"id":"body-id","name":"body-name"}`))
		req.Header.Set("Content-Type", "application/json")

		var got request
		err := Bind(req, &got)
		if err != nil {
			t.Fatalf("Bind() error = %v", err)
		}
		if !reflect.DeepEqual(got, request{ID: "body-id", Name: "body-name"}) {
			t.Fatalf("got = %#v", got)
		}
	})

	t.Run("post skips query", func(t *testing.T) {
		req := newRouteRequest(http.MethodPost, "/items?id=query-id", map[string]string{"id": "route-id"}, stringsNewReader(`{"id":"body-id"}`))
		req.Header.Set("Content-Type", "application/json")

		var got request
		if err := Bind(req, &got); err != nil {
			t.Fatalf("Bind() error = %v", err)
		}
		if !reflect.DeepEqual(got, request{ID: "body-id"}) {
			t.Fatalf("got = %#v", got)
		}
	})

	t.Run("empty body no-op with invalid content type", func(t *testing.T) {
		req := newRouteRequest(http.MethodPost, "/items", nil, stringsNewReader(""))
		req.Header.Set("Content-Type", "text/plain")
		req.ContentLength = 0

		dst := request{ID: "existing"}
		if err := BindBody(req, &dst); err != nil {
			t.Fatalf("BindBody() error = %v", err)
		}
		if dst.ID != "existing" {
			t.Fatalf("dst = %#v", dst)
		}
	})
}

func TestBindBody_JSONOnlyContract(t *testing.T) {
	type request struct {
		Name string `json:"name"`
	}

	t.Run("rejects unsupported media type", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/", stringsNewReader(`{"name":"kanata"}`))
		req.Header.Set("Content-Type", "application/xml")

		err := BindBody(req, &request{})
		assertHTTPError(t, err, http.StatusUnsupportedMediaType, CodeUnsupportedMediaType, "Content-Type must be application/json")
	})

	t.Run("invalid json returns invalid_json", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/", stringsNewReader(`{"name":`))
		req.Header.Set("Content-Type", "application/json")

		err := BindBody(req, &request{})
		assertHTTPError(t, err, http.StatusBadRequest, CodeInvalidJSON, "request body must be valid JSON")
	})
}

func TestSingleSourceBinders_UnsupportedScalarDestinationIsNoop(t *testing.T) {
	t.Run("query", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/?page=2", nil)
		value := 7
		if err := BindQueryParams(req, &value); err != nil {
			t.Fatalf("BindQueryParams() error = %v", err)
		}
		if value != 7 {
			t.Fatalf("value = %d, want 7", value)
		}
	})

	t.Run("path", func(t *testing.T) {
		req := newRouteRequest(http.MethodGet, "/items", map[string]string{"id": "42"}, nil)
		value := 7
		if err := BindPathValues(req, &value); err != nil {
			t.Fatalf("BindPathValues() error = %v", err)
		}
		if value != 7 {
			t.Fatalf("value = %d, want 7", value)
		}
	})

	t.Run("header", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		req.Header.Set("X-Retry", "2")
		value := 7
		if err := BindHeaders(req, &value); err != nil {
			t.Fatalf("BindHeaders() error = %v", err)
		}
		if value != 7 {
			t.Fatalf("value = %d, want 7", value)
		}
	})
}

func assertHTTPError(t *testing.T, err error, wantStatus int, wantCode, wantDetail string) {
	t.Helper()

	type httpError interface {
		Status() int
		Code() string
		Detail() string
	}

	got, ok := err.(httpError)
	if !ok {
		t.Fatalf("error type = %T, want HTTP error", err)
	}
	if got.Status() != wantStatus || got.Code() != wantCode || got.Detail() != wantDetail {
		t.Fatalf(
			"got error = (%d, %q, %q), want (%d, %q, %q)",
			got.Status(),
			got.Code(),
			got.Detail(),
			wantStatus,
			wantCode,
			wantDetail,
		)
	}
}

func assertBindingError(t *testing.T, err error, wantField string, wantValues []string, wantDetail string) {
	t.Helper()

	var bindingErr *BindingError
	if !asBindingError(err, &bindingErr) {
		t.Fatalf("error type = %T, want *BindingError", err)
	}
	if bindingErr.Field != wantField {
		t.Fatalf("field = %q, want %q", bindingErr.Field, wantField)
	}
	if !reflect.DeepEqual(bindingErr.Values, wantValues) {
		t.Fatalf("values = %#v, want %#v", bindingErr.Values, wantValues)
	}
	if wantDetail != "" && bindingErr.Detail() != wantDetail {
		t.Fatalf("detail = %q, want %q", bindingErr.Detail(), wantDetail)
	}
	if bindingErr.Status() != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", bindingErr.Status(), http.StatusBadRequest)
	}
}

func asBindingError(err error, target **BindingError) bool {
	if err == nil {
		return false
	}
	bindingErr, ok := err.(*BindingError)
	if !ok {
		return false
	}
	*target = bindingErr
	return true
}

func stringsNewReader(body string) io.Reader {
	return strings.NewReader(body)
}
