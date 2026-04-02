package chix

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/go-chi/chi/v5"
)

func TestBindAndValidateBody_DelegatesToReqx(t *testing.T) {
	req := httptest.NewRequest(http.MethodPost, "/accounts", strings.NewReader(`{"name":"kanata"}`))
	req.Header.Set("Content-Type", "application/json")

	var dst struct {
		Name string `json:"name" validate:"required"`
	}

	err := BindAndValidateBody(req, &dst)
	if err != nil {
		t.Fatalf("BindAndValidateBody() error = %v", err)
	}
	if dst.Name != "kanata" {
		t.Fatalf("name = %q, want kanata", dst.Name)
	}
}

func TestBind_DelegatesToReqx(t *testing.T) {
	type request struct {
		ID   string `param:"id" query:"id" json:"id"`
		Name string `json:"name"`
	}

	req := httptest.NewRequest(http.MethodGet, "/accounts?id=query-id", nil)
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("id", "route-id")
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))

	var dst request
	if err := Bind(req, &dst); err != nil {
		t.Fatalf("Bind() error = %v", err)
	}
	if dst.ID != "query-id" {
		t.Fatalf("id = %q, want query-id", dst.ID)
	}
}

func TestBindHeaders_DelegatesToReqx(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/accounts", nil)
	req.Header.Set("X-Request-Id", "req-123")

	var dst struct {
		RequestID string `header:"x-request-id"`
	}

	if err := BindHeaders(req, &dst); err != nil {
		t.Fatalf("BindHeaders() error = %v", err)
	}
	if dst.RequestID != "req-123" {
		t.Fatalf("request_id = %q, want req-123", dst.RequestID)
	}
}

func TestParamUUID_DelegatesToReqx(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/accounts", nil)
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("uuid", "550E8400-E29B-41D4-A716-446655440000")
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))

	got, err := ParamUUID(req, "uuid")
	if err != nil {
		t.Fatalf("ParamUUID() error = %v", err)
	}
	if got != "550e8400-e29b-41d4-a716-446655440000" {
		t.Fatalf("uuid = %q", got)
	}
}

func TestOK_DelegatesToResp(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/accounts", nil)
	rr := httptest.NewRecorder()

	if err := OK(rr, req, map[string]any{"id": "u_1"}); err != nil {
		t.Fatalf("OK() error = %v", err)
	}
	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rr.Code)
	}

	var payload map[string]any
	if err := json.Unmarshal(rr.Body.Bytes(), &payload); err != nil {
		t.Fatalf("json.Unmarshal() error = %v", err)
	}
	if payload["id"] != "u_1" {
		t.Fatalf("id = %#v, want u_1", payload["id"])
	}
}

func TestJSON_DelegatesToResp(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/accounts?pretty", nil)
	rr := httptest.NewRecorder()

	if err := JSON(rr, req, http.StatusAccepted, map[string]any{"id": "u_1"}); err != nil {
		t.Fatalf("JSON() error = %v", err)
	}
	if rr.Code != http.StatusAccepted {
		t.Fatalf("status = %d, want 202", rr.Code)
	}
	if body := rr.Body.String(); body != "{\n  \"id\": \"u_1\"\n}\n" {
		t.Fatalf("body = %q, want pretty JSON", body)
	}
}
