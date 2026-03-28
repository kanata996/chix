package validatorx_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/go-chi/chi/v5"
	playgroundvalidator "github.com/go-playground/validator/v10"

	"github.com/kanata996/chix"
	"github.com/kanata996/chix/validatorx"
)

func TestAdapterWrites422Violations(t *testing.T) {
	type input struct {
		Limit int    `query:"limit" validate:"gt=0"`
		Name  string `json:"name" validate:"required"`
	}

	rt := chix.New()
	router := chi.NewRouter()
	router.Method(http.MethodPost, "/users", chix.Handle(rt, chix.Operation[input, struct{}]{
		Method:   http.MethodPost,
		Validate: validatorx.Adapter[input](playgroundvalidator.New(playgroundvalidator.WithRequiredStructEnabled())),
	}, func(_ context.Context, _ *input) (*struct{}, error) {
		t.Fatal("handler should not run")
		return nil, nil
	}))

	req := httptest.NewRequest(http.MethodPost, "/users?limit=0", strings.NewReader(`{}`))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("expected 422, got %d", rec.Code)
	}

	var envelope struct {
		Error struct {
			Code    string           `json:"code"`
			Message string           `json:"message"`
			Details []chix.Violation `json:"details"`
		} `json:"error"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &envelope); err != nil {
		t.Fatalf("decode response: %v", err)
	}

	if envelope.Error.Code != "invalid_request" {
		t.Fatalf("unexpected error envelope: %+v", envelope)
	}
	if len(envelope.Error.Details) != 2 {
		t.Fatalf("expected 2 violations, got %+v", envelope.Error.Details)
	}

	got := map[string]chix.Violation{}
	for _, violation := range envelope.Error.Details {
		got[violation.Source+":"+violation.Field] = violation
	}

	if violation, ok := got["query:limit"]; !ok || violation.Code != "gt" {
		t.Fatalf("expected query violation, got %+v", envelope.Error.Details)
	}
	if violation, ok := got["body:name"]; !ok || violation.Code != "required" {
		t.Fatalf("expected body violation, got %+v", envelope.Error.Details)
	}
}

func TestAdapterMapsNestedBodyViolations(t *testing.T) {
	type profile struct {
		Name string `json:"name" validate:"required"`
	}
	type input struct {
		Profile profile `json:"profile"`
	}

	rt := chix.New()
	router := chi.NewRouter()
	router.Method(http.MethodPost, "/users", chix.Handle(rt, chix.Operation[input, struct{}]{
		Method:   http.MethodPost,
		Validate: validatorx.Adapter[input](playgroundvalidator.New(playgroundvalidator.WithRequiredStructEnabled())),
	}, func(_ context.Context, _ *input) (*struct{}, error) {
		t.Fatal("handler should not run")
		return nil, nil
	}))

	req := httptest.NewRequest(http.MethodPost, "/users", strings.NewReader(`{"profile":{}}`))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	var envelope struct {
		Error struct {
			Details []chix.Violation `json:"details"`
		} `json:"error"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &envelope); err != nil {
		t.Fatalf("decode response: %v", err)
	}

	if len(envelope.Error.Details) != 1 {
		t.Fatalf("expected 1 violation, got %+v", envelope.Error.Details)
	}

	violation := envelope.Error.Details[0]
	if violation.Source != "body" || violation.Field != "profile.name" || violation.Code != "required" {
		t.Fatalf("unexpected violation: %+v", violation)
	}
}

func TestAdapterMapsEmbeddedQueryViolations(t *testing.T) {
	type paging struct {
		Limit int `query:"limit" validate:"gt=0"`
	}
	type input struct {
		paging
	}

	rt := chix.New()
	router := chi.NewRouter()
	router.Method(http.MethodGet, "/users", chix.Handle(rt, chix.Operation[input, struct{}]{
		Validate: validatorx.Adapter[input](playgroundvalidator.New(playgroundvalidator.WithRequiredStructEnabled())),
	}, func(_ context.Context, _ *input) (*struct{}, error) {
		t.Fatal("handler should not run")
		return nil, nil
	}))

	req := httptest.NewRequest(http.MethodGet, "/users?limit=0", nil)
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	var envelope struct {
		Error struct {
			Details []chix.Violation `json:"details"`
		} `json:"error"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &envelope); err != nil {
		t.Fatalf("decode response: %v", err)
	}

	if len(envelope.Error.Details) != 1 {
		t.Fatalf("expected 1 violation, got %+v", envelope.Error.Details)
	}

	violation := envelope.Error.Details[0]
	if violation.Source != "query" || violation.Field != "limit" || violation.Code != "gt" {
		t.Fatalf("unexpected violation: %+v", violation)
	}
}
