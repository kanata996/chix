package reqx

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

type decodeValidatePayload struct {
	Name string `json:"name" validate:"required"`
}

func (p *decodeValidatePayload) Normalize() {
	if p == nil {
		return
	}
	p.Name = strings.TrimSpace(p.Name)
}

func TestDecodeValidateJSON(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(`{"name":" raw "}`))
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()

		var got decodeValidatePayload
		if err := DecodeValidateJSON(rec, req, &got); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if got.Name != "raw" {
			t.Fatalf("unexpected payload: %#v", got)
		}
	})

	t.Run("validation failed", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(`{"name":"   "}`))
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()

		var got decodeValidatePayload
		err := DecodeValidateJSON(rec, req, &got)
		problem, ok := AsProblem(err)
		if !ok {
			t.Fatalf("expected problem, got %T (%v)", err, err)
		}
		if problem.StatusCode != http.StatusUnprocessableEntity {
			t.Fatalf("unexpected status: %d", problem.StatusCode)
		}
		if len(problem.Details) != 1 || problem.Details[0].Field != "name" {
			t.Fatalf("unexpected details: %#v", problem.Details)
		}
	})

	t.Run("decode error is returned directly", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(`{"extra":1}`))
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()

		var got decodeValidatePayload
		err := DecodeValidateJSON(rec, req, &got)
		problem, ok := AsProblem(err)
		if !ok {
			t.Fatalf("expected problem, got %T (%v)", err, err)
		}
		if problem.StatusCode != http.StatusBadRequest {
			t.Fatalf("unexpected status: %d", problem.StatusCode)
		}
		if len(problem.Details) != 1 || problem.Details[0] != UnknownField("extra") {
			t.Fatalf("unexpected details: %#v", problem.Details)
		}
	})

	t.Run("invalid validate target is boundary error", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(`[]`))
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()

		var got []string
		err := DecodeValidateJSON(rec, req, &got)
		if err == nil {
			t.Fatal("expected error")
		}
		if !errors.Is(err, ErrInvalidValidateTarget) {
			t.Fatalf("expected ErrInvalidValidateTarget, got %v", err)
		}
	})
}
