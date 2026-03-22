package chix_test

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/kanata996/chix"
	reqxpkg "github.com/kanata996/chix/reqx"
)

func TestDecodeAndValidateJSONFacadeSuccess(t *testing.T) {
	type createUserRequest struct {
		Name string `json:"name"`
	}

	h := chix.Wrap(func(w http.ResponseWriter, r *http.Request) error {
		var req createUserRequest
		if err := chix.DecodeAndValidateJSON(r, &req, func(value *createUserRequest) []chix.Violation {
			if strings.TrimSpace(value.Name) == "" {
				return []chix.Violation{{Field: "name", Code: "required", Message: "is required"}}
			}
			return nil
		}); err != nil {
			return err
		}

		return chix.Write(w, http.StatusCreated, map[string]any{"name": strings.TrimSpace(req.Name)})
	})

	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/users", strings.NewReader(`{"name":" alice "}`))
	req.Header.Set("Content-Type", "application/json")
	h.ServeHTTP(rr, req)

	if rr.Code != http.StatusCreated {
		t.Fatalf("status = %d, want %d", rr.Code, http.StatusCreated)
	}

	var payload map[string]any
	if err := json.Unmarshal(rr.Body.Bytes(), &payload); err != nil {
		t.Fatalf("unmarshal response: %v", err)
	}
	data, ok := payload["data"].(map[string]any)
	if !ok {
		t.Fatalf("data = %#v, want object", payload["data"])
	}
	if got := data["name"]; got != "alice" {
		t.Fatalf("data.name = %#v, want alice", got)
	}
}

func TestDecodeJSONFacadeWithOptions(t *testing.T) {
	type createUserRequest struct {
		Name string `json:"name"`
	}

	req := httptest.NewRequest(
		http.MethodPost,
		"/users",
		strings.NewReader(`{"name":"alice","extra":true}`),
	)
	req.Header.Set("Content-Type", "application/json")

	var body createUserRequest
	err := chix.DecodeJSON(
		req,
		&body,
		chix.WithMaxBodyBytes(1024),
		chix.AllowUnknownFields(),
	)
	if err != nil {
		t.Fatalf("DecodeJSON() error = %v", err)
	}
	if body.Name != "alice" {
		t.Fatalf("body.Name = %q, want alice", body.Name)
	}
}

func TestDecodeJSONFacadeAllowsEmptyBody(t *testing.T) {
	req := httptest.NewRequest(http.MethodPost, "/users", http.NoBody)
	req.Header.Set("Content-Type", "application/json")

	var body struct {
		Name string `json:"name"`
	}
	err := chix.DecodeJSON(req, &body, chix.AllowEmptyBody())
	if err != nil {
		t.Fatalf("DecodeJSON() error = %v", err)
	}
	if body.Name != "" {
		t.Fatalf("body.Name = %q, want empty", body.Name)
	}
}

func TestDecodeJSONFacadePassesThroughNonProblemErrors(t *testing.T) {
	var body struct {
		Name string `json:"name"`
	}

	err := chix.DecodeJSON((*http.Request)(nil), &body)
	if err == nil {
		t.Fatal("expected error, got nil")
	}

	var boundaryErr *chix.Error
	if errors.As(err, &boundaryErr) {
		t.Fatalf("error = %T, want non-boundary error", err)
	}
}

func TestDecodeQueryFacadeRejectsUnknownField(t *testing.T) {
	h := chix.Wrap(func(w http.ResponseWriter, r *http.Request) error {
		var query struct {
			Page int `query:"page"`
		}
		return chix.DecodeQuery(r, &query)
	})

	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/users?page=2&extra=yes", nil)
	h.ServeHTTP(rr, req)

	if rr.Code != http.StatusUnprocessableEntity {
		t.Fatalf("status = %d, want %d", rr.Code, http.StatusUnprocessableEntity)
	}

	var payload map[string]any
	if err := json.Unmarshal(rr.Body.Bytes(), &payload); err != nil {
		t.Fatalf("unmarshal response: %v", err)
	}

	rawError, ok := payload["error"].(map[string]any)
	if !ok {
		t.Fatalf("error envelope missing or wrong type: %#v", payload["error"])
	}
	if got := rawError["code"]; got != "invalid_request" {
		t.Fatalf("error.code = %#v, want invalid_request", got)
	}
	details, ok := rawError["details"].([]any)
	if !ok || len(details) != 1 {
		t.Fatalf("error.details = %#v, want one detail", rawError["details"])
	}
	detail, ok := details[0].(map[string]any)
	if !ok {
		t.Fatalf("details[0] = %#v, want object", details[0])
	}
	if got := detail["field"]; got != "extra" {
		t.Fatalf("detail.field = %#v, want extra", got)
	}
	if got := detail["code"]; got != "unknown" {
		t.Fatalf("detail.code = %#v, want unknown", got)
	}
}

func TestValidateFacadeNormalizesViolation(t *testing.T) {
	h := chix.Wrap(func(w http.ResponseWriter, r *http.Request) error {
		req := struct {
			Name string `json:"name"`
		}{}
		return chix.Validate(&req, func(value *struct {
			Name string `json:"name"`
		}) []chix.Violation {
			return []chix.Violation{{Field: "name"}}
		})
	})

	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/users", nil)
	h.ServeHTTP(rr, req)

	if rr.Code != http.StatusUnprocessableEntity {
		t.Fatalf("status = %d, want %d", rr.Code, http.StatusUnprocessableEntity)
	}

	var payload map[string]any
	if err := json.Unmarshal(rr.Body.Bytes(), &payload); err != nil {
		t.Fatalf("unmarshal response: %v", err)
	}

	rawError, ok := payload["error"].(map[string]any)
	if !ok {
		t.Fatalf("error envelope missing or wrong type: %#v", payload["error"])
	}
	details, ok := rawError["details"].([]any)
	if !ok || len(details) != 1 {
		t.Fatalf("error.details = %#v, want one detail", rawError["details"])
	}
	detail, ok := details[0].(map[string]any)
	if !ok {
		t.Fatalf("details[0] = %#v, want object", details[0])
	}
	if got := detail["field"]; got != "name" {
		t.Fatalf("detail.field = %#v, want name", got)
	}
	if got := detail["code"]; got != "invalid" {
		t.Fatalf("detail.code = %#v, want invalid", got)
	}
}

func TestDecodeQueryFacadeCanAllowUnknownFields(t *testing.T) {
	h := chix.Wrap(func(w http.ResponseWriter, r *http.Request) error {
		var query struct {
			ID string `query:"id"`
		}
		if err := chix.DecodeQuery(r, &query, chix.AllowUnknownQueryFields()); err != nil {
			return err
		}
		return chix.Write(w, http.StatusOK, map[string]any{"id": query.ID})
	})

	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/users?id=u_1&extra=yes", nil)
	h.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rr.Code, http.StatusOK)
	}
}

func TestDecodeQueryFacadeReturnsChixError(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/users?page=bad", nil)

	var query struct {
		Page int `query:"page"`
	}
	err := chix.DecodeQuery(req, &query)
	if err == nil {
		t.Fatal("expected error, got nil")
	}

	var boundaryErr *chix.Error
	if !errors.As(err, &boundaryErr) || boundaryErr == nil {
		t.Fatalf("error = %T, want *chix.Error", err)
	}
	if got := boundaryErr.Code(); got != "invalid_request" {
		t.Fatalf("code = %q, want invalid_request", got)
	}
}

func TestDecodeAndValidateQueryFacadeSuccess(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/users?page=2&q=alice", nil)

	var query struct {
		Page int    `query:"page"`
		Q    string `query:"q"`
	}
	err := chix.DecodeAndValidateQuery(req, &query, func(value *struct {
		Page int    `query:"page"`
		Q    string `query:"q"`
	}) []chix.Violation {
		if value.Page < 1 {
			return []chix.Violation{{Field: "page", Code: "min", Message: "must be at least 1"}}
		}
		if strings.TrimSpace(value.Q) == "" {
			return []chix.Violation{{Field: "q", Code: "required", Message: "is required"}}
		}
		return nil
	})
	if err != nil {
		t.Fatalf("DecodeAndValidateQuery() error = %v", err)
	}
	if query.Page != 2 {
		t.Fatalf("query.Page = %d, want 2", query.Page)
	}
	if query.Q != "alice" {
		t.Fatalf("query.Q = %q, want alice", query.Q)
	}
}

func TestWriteErrorAdaptsReqxProblem(t *testing.T) {
	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/users", nil)

	chix.WriteError(rr, req, reqxpkg.NewProblem(http.StatusBadRequest, "invalid_request", "invalid request"))

	assertErrorResponse(t, rr, http.StatusBadRequest, "invalid_request", "invalid request")
}
