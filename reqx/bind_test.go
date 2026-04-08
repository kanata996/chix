package reqx

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestBind_StageOrderAndMethodRules(t *testing.T) {
	t.Run("get applies path query then body", func(t *testing.T) {
		type request struct {
			ID   string `param:"id" query:"id" json:"id"`
			Name string `query:"name" json:"name"`
		}

		req := requestWithPathParams(map[string][]string{
			"id": {"route-id"},
		})
		req.Method = http.MethodGet
		req.URL.RawQuery = "id=query-id&name=query-name"
		setRequestBody(req, mimeApplicationJSON, `{"id":"body-id","name":"body-name"}`)

		var bound request
		if err := Bind(req, &bound); err != nil {
			t.Fatalf("Bind() error = %v", err)
		}
		if bound.ID != "body-id" || bound.Name != "body-name" {
			t.Fatalf("bound = %#v, want body values to win", bound)
		}
	})

	t.Run("delete binds query over path when body is absent", func(t *testing.T) {
		type request struct {
			ID string `param:"id" query:"id"`
		}

		req := requestWithPathParams(map[string][]string{
			"id": {"route-id"},
		})
		req.Method = http.MethodDelete
		req.URL.RawQuery = "id=query-id"

		var bound request
		if err := Bind(req, &bound); err != nil {
			t.Fatalf("Bind() error = %v", err)
		}
		if bound.ID != "query-id" {
			t.Fatalf("Bind() id = %q, want query-id", bound.ID)
		}
	})

	t.Run("head also binds query", func(t *testing.T) {
		type request struct {
			ID string `param:"id" query:"id"`
		}

		req := requestWithPathParams(map[string][]string{
			"id": {"route-id"},
		})
		req.Method = http.MethodHead
		req.URL.RawQuery = "id=query-id"

		var bound request
		if err := Bind(req, &bound); err != nil {
			t.Fatalf("Bind() error = %v", err)
		}
		if bound.ID != "query-id" {
			t.Fatalf("Bind() id = %q, want query-id", bound.ID)
		}
	})

	t.Run("post skips query but still binds body", func(t *testing.T) {
		type request struct {
			ID    string `param:"id" json:"id"`
			Scope string `query:"scope"`
		}

		req := requestWithPathParams(map[string][]string{
			"id": {"route-id"},
		})
		req.Method = http.MethodPost
		req.URL.RawQuery = "scope=query-scope"
		setRequestBody(req, mimeApplicationJSON, `{"id":"body-id"}`)

		var bound request
		if err := Bind(req, &bound); err != nil {
			t.Fatalf("Bind() error = %v", err)
		}
		if bound.ID != "body-id" || bound.Scope != "" {
			t.Fatalf("bound = %#v, want body id and skipped query scope", bound)
		}
	})
}

func TestBind_DoesNotUseHeadersByDefault(t *testing.T) {
	type request struct {
		RequestID string `header:"x-request-id"`
	}

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("X-Request-Id", "req-123")

	var bound request
	if err := Bind(req, &bound); err != nil {
		t.Fatalf("Bind() error = %v", err)
	}
	if bound.RequestID != "" {
		t.Fatalf("request_id = %q, want empty", bound.RequestID)
	}
}

func TestBind_EmptyBodyNoopUsesContentLengthZero(t *testing.T) {
	type request struct {
		ID   string `param:"id"`
		Page int    `query:"page"`
		Name string `json:"name"`
	}

	req := requestWithPathParams(map[string][]string{
		"id": {"route-id"},
	})
	req.Method = http.MethodGet
	req.URL.RawQuery = "page=2"
	req.Header.Set("Content-Type", "text/plain")
	req.Body = io.NopCloser(strings.NewReader(""))
	req.ContentLength = 0

	dst := request{Name: "existing-name"}
	if err := Bind(req, &dst); err != nil {
		t.Fatalf("Bind() error = %v", err)
	}
	if dst.ID != "route-id" || dst.Page != 2 || dst.Name != "existing-name" {
		t.Fatalf("dst = %#v, want path/query updates and body no-op", dst)
	}
}

func TestBind_PartialUpdatesPersistAcrossStageFailure(t *testing.T) {
	t.Run("path update remains when query fails", func(t *testing.T) {
		type request struct {
			ID   string `param:"id"`
			Page int    `query:"page"`
		}

		req := requestWithPathParams(map[string][]string{
			"id": {"route-id"},
		})
		req.Method = http.MethodGet
		req.URL.RawQuery = "page=oops"

		dst := request{ID: "existing-id", Page: 3}
		err := Bind(req, &dst)
		_ = assertHTTPError(t, err, http.StatusBadRequest, "bad_request", "Bad Request")
		if dst.ID != "route-id" || dst.Page != 3 {
			t.Fatalf("dst = %#v, want path update preserved before query failure", dst)
		}
	})

	t.Run("query update remains when body fails", func(t *testing.T) {
		type request struct {
			ID   string `param:"id"`
			Page int    `query:"page"`
			Age  int    `json:"age"`
		}

		req := requestWithPathParams(map[string][]string{
			"id": {"route-id"},
		})
		req.Method = http.MethodGet
		req.URL.RawQuery = "page=7"
		setRequestBody(req, mimeApplicationJSON, `{"age":"oops"}`)

		dst := request{ID: "existing-id", Page: 3, Age: 1}
		err := Bind(req, &dst)
		_ = assertHTTPError(t, err, http.StatusBadRequest, CodeInvalidJSON, "request body must be valid JSON")
		if dst.ID != "route-id" || dst.Page != 7 || dst.Age != 1 {
			t.Fatalf("dst = %#v, want path/query updates preserved before body failure", dst)
		}
	})
}

func TestBindAndValidate_MixedSourceEmptyBodyDefersToValidate(t *testing.T) {
	type request struct {
		OrgID string `param:"org_id"`
		Name  string `json:"name" validate:"required"`
	}

	req := requestWithPathParams(map[string][]string{
		"org_id": {"org-123"},
	})
	req.Method = http.MethodPost
	req.Header.Set("Content-Type", mimeApplicationJSON)
	req.Body = io.NopCloser(strings.NewReader(""))
	req.ContentLength = 0

	var dst request
	err := BindAndValidate(req, &dst)
	httpErr := assertHTTPError(t, err, http.StatusUnprocessableEntity, CodeInvalidRequest, "request contains invalid fields")
	if got := len(httpErr.Errors()); got != 1 {
		t.Fatalf("errors len = %d, want 1", got)
	}
	if dst.OrgID != "org-123" {
		t.Fatalf("org_id = %q, want org-123", dst.OrgID)
	}
}

func setRequestBody(req *http.Request, contentType, body string) {
	req.Header.Set("Content-Type", contentType)
	req.Body = io.NopCloser(strings.NewReader(body))
	req.ContentLength = int64(len(body))
}
