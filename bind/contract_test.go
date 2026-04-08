package bind

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestBindBody_EmptyBodyPreservesExistingValues(t *testing.T) {
	type request struct {
		Name string `json:"name"`
		Age  int    `json:"age"`
	}

	req := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(""))
	req.Header.Set("Content-Type", mimeApplicationJSON)
	req.ContentLength = 0

	dst := request{Name: "kanata", Age: 17}
	if err := BindBody(req, &dst); err != nil {
		t.Fatalf("BindBody() error = %v", err)
	}
	if dst.Name != "kanata" || dst.Age != 17 {
		t.Fatalf("dst = %#v, want existing values preserved", dst)
	}
}

func TestBindQueryParamsAndHeadersMissingInputsPreserveExistingValues(t *testing.T) {
	t.Run("query", func(t *testing.T) {
		type request struct {
			Page int `query:"page"`
		}

		req := httptest.NewRequest(http.MethodGet, "/?other=1", nil)
		dst := request{Page: 3}
		if err := BindQueryParams(req, &dst); err != nil {
			t.Fatalf("BindQueryParams() error = %v", err)
		}
		if dst.Page != 3 {
			t.Fatalf("page = %d, want 3", dst.Page)
		}
	})

	t.Run("header", func(t *testing.T) {
		type request struct {
			TraceID string `header:"x-trace-id"`
		}

		req := httptest.NewRequest(http.MethodGet, "/", nil)
		dst := request{TraceID: "existing"}
		if err := BindHeaders(req, &dst); err != nil {
			t.Fatalf("BindHeaders() error = %v", err)
		}
		if dst.TraceID != "existing" {
			t.Fatalf("trace_id = %q, want existing", dst.TraceID)
		}
	})
}

func TestBind_PathAndQueryFailuresPreservePreviousStageWrites(t *testing.T) {
	t.Run("query failure preserves path write", func(t *testing.T) {
		type request struct {
			ID   string `param:"id"`
			Page int    `query:"page"`
		}

		req := requestWithPathParams(map[string][]string{
			"id": {"route-id"},
		})
		req.Method = http.MethodGet
		req.URL.RawQuery = "page=oops"

		dst := request{ID: "existing", Page: 1}
		_ = assertHTTPError(t, Bind(req, &dst), http.StatusBadRequest, "bad_request", "Bad Request")
		if dst.ID != "route-id" || dst.Page != 1 {
			t.Fatalf("dst = %#v, want path write preserved", dst)
		}
	})

	t.Run("body failure preserves path and query writes", func(t *testing.T) {
		type request struct {
			ID   string `param:"id"`
			Page int    `query:"page"`
			Age  int    `json:"age"`
		}

		req := requestWithPathParams(map[string][]string{
			"id": {"route-id"},
		})
		req.Method = http.MethodGet
		req.URL.RawQuery = "page=2"
		setRequestBody(req, mimeApplicationJSON, `{"age":"oops"}`)

		dst := request{ID: "existing", Page: 1, Age: 7}
		_ = assertHTTPError(t, Bind(req, &dst), http.StatusBadRequest, CodeInvalidJSON, "request body must be valid JSON")
		if dst.ID != "route-id" || dst.Page != 2 || dst.Age != 7 {
			t.Fatalf("dst = %#v, want path/query writes preserved", dst)
		}
	})
}
