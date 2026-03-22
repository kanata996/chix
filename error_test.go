package chix

import (
	"errors"
	"net/http"
	"testing"
)

func TestRequestErrorNormalizesFields(t *testing.T) {
	err := RequestError(200, "", "")

	if got := err.Kind(); got != KindRequest {
		t.Fatalf("kind = %q, want %q", got, KindRequest)
	}
	if got := err.Status(); got != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", got, http.StatusBadRequest)
	}
	if got := err.Code(); got != "request_error" {
		t.Fatalf("code = %q, want request_error", got)
	}
	if got := err.Message(); got != "request error" {
		t.Fatalf("message = %q, want request error", got)
	}
	if got := err.Error(); got != "request error" {
		t.Fatalf("error string = %q, want request error", got)
	}
}

func TestErrorDetailsReturnsCopy(t *testing.T) {
	err := DomainError(http.StatusConflict, "conflict", "conflict", map[string]any{"field": "email"})

	details := err.Details()
	if len(details) != 1 {
		t.Fatalf("details len = %d, want 1", len(details))
	}

	details[0] = "mutated"

	again := err.Details()
	if got, ok := again[0].(map[string]any); !ok || got["field"] != "email" {
		t.Fatalf("details should be copied, got %#v", again[0])
	}
}

func TestNilErrorAccessorsUseInternalDefaults(t *testing.T) {
	var err *Error

	if got := err.Kind(); got != KindInternal {
		t.Fatalf("kind = %q, want %q", got, KindInternal)
	}
	if got := err.Status(); got != http.StatusInternalServerError {
		t.Fatalf("status = %d, want %d", got, http.StatusInternalServerError)
	}
	if got := err.Code(); got != "internal_error" {
		t.Fatalf("code = %q, want internal_error", got)
	}
	if got := err.Message(); got != "internal server error" {
		t.Fatalf("message = %q, want internal server error", got)
	}
	if got := err.Error(); got != "" {
		t.Fatalf("error string = %q, want empty", got)
	}
	if got := err.Details(); got != nil {
		t.Fatalf("details = %#v, want nil", got)
	}
}

func TestDomainErrorNormalizesDefaults(t *testing.T) {
	err := DomainError(200, "", "")

	if got := err.Kind(); got != KindDomain {
		t.Fatalf("kind = %q, want %q", got, KindDomain)
	}
	if got := err.Status(); got != http.StatusUnprocessableEntity {
		t.Fatalf("status = %d, want %d", got, http.StatusUnprocessableEntity)
	}
	if got := err.Code(); got != "domain_error" {
		t.Fatalf("code = %q, want domain_error", got)
	}
	if got := err.Message(); got != "domain error" {
		t.Fatalf("message = %q, want domain error", got)
	}
}

func TestInternalErrorNormalizesDefaults(t *testing.T) {
	err := InternalError(400, "", "")

	if got := err.Kind(); got != KindInternal {
		t.Fatalf("kind = %q, want %q", got, KindInternal)
	}
	if got := err.Status(); got != http.StatusInternalServerError {
		t.Fatalf("status = %d, want %d", got, http.StatusInternalServerError)
	}
	if got := err.Code(); got != "internal_error" {
		t.Fatalf("code = %q, want internal_error", got)
	}
	if got := err.Message(); got != "internal server error" {
		t.Fatalf("message = %q, want internal server error", got)
	}
}

func TestNewErrorInvalidKindFallsBackToInternalDefaults(t *testing.T) {
	err := newError(ErrorKind("weird"), 200, "", "", nil)

	if got := err.Kind(); got != ErrorKind("weird") {
		t.Fatalf("kind = %q, want weird", got)
	}
	if got := err.Status(); got != http.StatusInternalServerError {
		t.Fatalf("status = %d, want %d", got, http.StatusInternalServerError)
	}
	if got := err.Code(); got != "internal_error" {
		t.Fatalf("code = %q, want internal_error", got)
	}
	if got := err.Message(); got != "internal server error" {
		t.Fatalf("message = %q, want internal server error", got)
	}
}

func TestMapErrorSkipsNilMapper(t *testing.T) {
	mapped := mapError(errors.New("boom"), config{
		mappers: []ErrorMapper{nil},
	})

	if got := mapped.Status(); got != http.StatusInternalServerError {
		t.Fatalf("status = %d, want %d", got, http.StatusInternalServerError)
	}
	if got := mapped.Code(); got != "internal_error" {
		t.Fatalf("code = %q, want internal_error", got)
	}
}
