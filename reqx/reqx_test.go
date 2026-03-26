package reqx

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/go-chi/chi/v5"
)

func TestDecodeJSON(t *testing.T) {
	type input struct {
		ID    string `path:"id"`
		Limit int    `query:"limit"`
		Name  string `json:"name" validate:"required,min=3"`
	}

	req := httptest.NewRequest(http.MethodPost, "/users/42?limit=5", strings.NewReader(`{"name":"Ada"}`))
	req = withRouteParam(req, "id", "42")

	var dst input
	if err := Decode(req, &dst); err != nil {
		t.Fatalf("decode request: %v", err)
	}

	if dst.ID != "42" {
		t.Fatalf("expected path parameter to bind, got %q", dst.ID)
	}
	if dst.Limit != 5 {
		t.Fatalf("expected query parameter to bind, got %d", dst.Limit)
	}
	if dst.Name != "Ada" {
		t.Fatalf("expected JSON field to bind, got %q", dst.Name)
	}
}

func TestDecodeValidationError(t *testing.T) {
	type input struct {
		Role  string `query:"role" validate:"oneof=admin member"`
		Email string `json:"email" validate:"required,email"`
	}

	req := httptest.NewRequest(http.MethodPost, "/users?role=guest", strings.NewReader(`{"email":"broken"}`))

	var dst input
	err := Decode(req, &dst)
	if err == nil {
		t.Fatalf("expected validation error")
	}

	var requestErr *RequestError
	if !errors.As(err, &requestErr) {
		t.Fatalf("expected request error, got %T", err)
	}
	if requestErr.Kind != KindValidation {
		t.Fatalf("expected validation kind, got %s", requestErr.Kind)
	}
	if len(requestErr.Violations) != 2 {
		t.Fatalf("expected 2 violations, got %d", len(requestErr.Violations))
	}

	got := map[string]string{}
	for _, violation := range requestErr.Violations {
		got[violation.Source+":"+violation.Field] = violation.Rule
	}
	if got["query:role"] != "oneof" {
		t.Fatalf("expected query role violation, got %+v", requestErr.Violations)
	}
	if got["body:email"] != "email" {
		t.Fatalf("expected body email violation, got %+v", requestErr.Violations)
	}
}

func TestDecodeInvalidQueryParameter(t *testing.T) {
	type input struct {
		Limit int `query:"limit"`
	}

	req := httptest.NewRequest(http.MethodGet, "/users?limit=oops", nil)

	var dst input
	err := Decode(req, &dst)
	if err == nil {
		t.Fatalf("expected query error")
	}

	var requestErr *RequestError
	if !errors.As(err, &requestErr) {
		t.Fatalf("expected request error, got %T", err)
	}
	if requestErr.Kind != KindInvalidParameter {
		t.Fatalf("expected invalid parameter kind, got %s", requestErr.Kind)
	}
	if requestErr.Source != "query" || requestErr.Name != "limit" {
		t.Fatalf("unexpected parameter metadata: %+v", requestErr)
	}
}

func withRouteParam(r *http.Request, name, value string) *http.Request {
	routeContext := chi.NewRouteContext()
	routeContext.URLParams.Add(name, value)
	return r.WithContext(context.WithValue(r.Context(), chi.RouteCtxKey, routeContext))
}
