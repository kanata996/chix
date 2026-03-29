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

func TestHandleMapsPathQueryAndNestedBodyValidationErrors(t *testing.T) {
	type profile struct {
		Name string `json:"name" validate:"required"`
	}
	type input struct {
		ID      string  `path:"id" validate:"min=4"`
		Limit   int     `query:"limit" validate:"gt=0"`
		Profile profile `json:"profile"`
	}

	rt := New()
	router := chi.NewRouter()
	router.Post("/users/{id}", Handle(rt, func(_ context.Context, _ *input) (*struct{}, error) {
		t.Fatal("handler should not run")
		return nil, nil
	}))

	req := httptest.NewRequest(http.MethodPost, "/users/u1?limit=0", strings.NewReader(`{"profile":{}}`))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("expected 422, got %d", rec.Code)
	}

	details := decodeValidationDetails(t, rec)
	if len(details) != 3 {
		t.Fatalf("expected 3 validation details, got %+v", details)
	}

	got := map[string]validationDetail{}
	for _, detail := range details {
		got[detail.Source+":"+detail.Field] = detail
	}

	if detail, ok := got["path:id"]; !ok || detail.Code != "min" {
		t.Fatalf("expected path validation detail, got %+v", details)
	}
	if detail, ok := got["query:limit"]; !ok || detail.Code != "gt" {
		t.Fatalf("expected query validation detail, got %+v", details)
	}
	if detail, ok := got["body:profile.name"]; !ok || detail.Code != "required" {
		t.Fatalf("expected nested body validation detail, got %+v", details)
	}
}

func TestHandleMapsNestedBodyValidationErrors(t *testing.T) {
	type profile struct {
		Name string `json:"name" validate:"required"`
	}
	type input struct {
		Profile profile `json:"profile"`
	}

	rt := New()
	router := chi.NewRouter()
	router.Post("/users", Handle(rt, func(_ context.Context, _ *input) (*struct{}, error) {
		t.Fatal("handler should not run")
		return nil, nil
	}))

	req := httptest.NewRequest(http.MethodPost, "/users", strings.NewReader(`{"profile":{}}`))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	details := decodeValidationDetails(t, rec)
	if len(details) != 1 {
		t.Fatalf("expected 1 validation detail, got %+v", details)
	}

	detail := details[0]
	if detail.Source != "body" || detail.Field != "profile.name" || detail.Code != "required" {
		t.Fatalf("unexpected validation detail: %+v", detail)
	}
}

func TestHandleMapsEmbeddedQueryValidationErrors(t *testing.T) {
	type paging struct {
		Limit int `query:"limit" validate:"gt=0"`
	}
	type input struct {
		paging
	}

	rt := New()
	router := chi.NewRouter()
	router.Get("/users", Handle(rt, func(_ context.Context, _ *input) (*struct{}, error) {
		t.Fatal("handler should not run")
		return nil, nil
	}))

	req := httptest.NewRequest(http.MethodGet, "/users?limit=0", nil)
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	details := decodeValidationDetails(t, rec)
	if len(details) != 1 {
		t.Fatalf("expected 1 validation detail, got %+v", details)
	}

	detail := details[0]
	if detail.Source != "query" || detail.Field != "limit" || detail.Code != "gt" {
		t.Fatalf("unexpected validation detail: %+v", detail)
	}
}

func TestHandleMapsEmbeddedBodyValidationErrors(t *testing.T) {
	type payload struct {
		Name string `json:"name" validate:"required"`
	}
	type input struct {
		payload
	}

	rt := New()
	router := chi.NewRouter()
	router.Post("/users", Handle(rt, func(_ context.Context, _ *input) (*struct{}, error) {
		t.Fatal("handler should not run")
		return nil, nil
	}))

	req := httptest.NewRequest(http.MethodPost, "/users", strings.NewReader(`{}`))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	details := decodeValidationDetails(t, rec)
	if len(details) != 1 {
		t.Fatalf("expected 1 validation detail, got %+v", details)
	}

	detail := details[0]
	if detail.Source != "body" || detail.Field != "name" || detail.Code != "required" {
		t.Fatalf("unexpected validation detail: %+v", detail)
	}
}

func decodeValidationDetails(t *testing.T, rec *httptest.ResponseRecorder) []validationDetail {
	t.Helper()

	var envelope struct {
		Error struct {
			Details []validationDetail `json:"details"`
		} `json:"error"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &envelope); err != nil {
		t.Fatalf("decode response: %v", err)
	}

	return envelope.Error.Details
}
