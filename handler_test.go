package chix_test

import (
	"errors"
	"net/http"
	"strings"
	"testing"

	"github.com/kanata996/chix"
)

func TestWrapWritesRequestError(t *testing.T) {
	h := chix.Wrap(func(w http.ResponseWriter, r *http.Request) error {
		return chix.RequestError(http.StatusBadRequest, "invalid_request", "request is invalid")
	})

	rr := newResponseRecorder()
	req := newRequest()
	h.ServeHTTP(rr, req)

	assertErrorResponse(t, rr, http.StatusBadRequest, "invalid_request", "request is invalid")
}

func TestWrapUsesMapper(t *testing.T) {
	notFound := errors.New("user not found")

	h := chix.Wrap(
		func(w http.ResponseWriter, r *http.Request) error {
			return notFound
		},
		chix.WithErrorMappers(func(err error) *chix.Error {
			if errors.Is(err, notFound) {
				return chix.DomainError(http.StatusNotFound, "user_not_found", "user not found")
			}
			return nil
		}),
	)

	rr := newResponseRecorder()
	req := newRequest()
	h.ServeHTTP(rr, req)

	assertErrorResponse(t, rr, http.StatusNotFound, "user_not_found", "user not found")
}

func TestWrapWritesDomainConflict(t *testing.T) {
	h := chix.Wrap(func(w http.ResponseWriter, r *http.Request) error {
		return chix.DomainError(http.StatusConflict, "user_conflict", "user already exists")
	})

	rr := newResponseRecorder()
	req := newRequest()
	h.ServeHTTP(rr, req)

	assertErrorResponse(t, rr, http.StatusConflict, "user_conflict", "user already exists")
}

func TestWrapWritesDomainRuleViolation(t *testing.T) {
	h := chix.Wrap(func(w http.ResponseWriter, r *http.Request) error {
		return chix.DomainError(
			http.StatusUnprocessableEntity,
			"rule_violation",
			"user cannot be activated while billing is overdue",
		)
	})

	rr := newResponseRecorder()
	req := newRequest()
	h.ServeHTTP(rr, req)

	assertErrorResponse(
		t,
		rr,
		http.StatusUnprocessableEntity,
		"rule_violation",
		"user cannot be activated while billing is overdue",
	)
}

func TestWrapUnknownErrorFallsBackToInternal(t *testing.T) {
	h := chix.Wrap(func(w http.ResponseWriter, r *http.Request) error {
		return errors.New("database exploded")
	})

	rr := newResponseRecorder()
	req := newRequest()
	h.ServeHTTP(rr, req)

	assertErrorResponse(t, rr, http.StatusInternalServerError, "internal_error", "internal server error")
	if body := rr.Body.String(); body == "" || strings.Contains(body, "database exploded") {
		t.Fatalf("body should not leak internal error, got %q", body)
	}
}

func TestWrapPanicPropagatesToCaller(t *testing.T) {
	h := chix.Wrap(func(w http.ResponseWriter, r *http.Request) error {
		panic("boom")
	})

	rr := newResponseRecorder()
	req := newRequest()

	defer func() {
		if rec := recover(); rec != "boom" {
			t.Fatalf("panic = %#v, want boom", rec)
		}
	}()

	h.ServeHTTP(rr, req)
}

func TestWrapDoesNotRewriteStartedResponseOnReturnedError(t *testing.T) {
	h := chix.Wrap(func(w http.ResponseWriter, r *http.Request) error {
		if _, err := w.Write([]byte("partial")); err != nil {
			return err
		}
		return chix.DomainError(http.StatusConflict, "user_conflict", "user already exists")
	})

	rr := newResponseRecorder()
	req := newRequest()
	h.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rr.Code, http.StatusOK)
	}
	if got := rr.Body.String(); got != "partial" {
		t.Fatalf("body = %q, want partial", got)
	}
}

func TestWrapNilHandlerFallsBackToInternal(t *testing.T) {
	h := chix.Wrap(nil)

	rr := newResponseRecorder()
	req := newRequest()
	h.ServeHTTP(rr, req)

	assertErrorResponse(t, rr, http.StatusInternalServerError, "internal_error", "internal server error")
}

func TestWrapPanicAfterResponseStartedPropagatesWithoutRewriting(t *testing.T) {
	h := chix.Wrap(func(w http.ResponseWriter, r *http.Request) error {
		if _, err := w.Write([]byte("partial")); err != nil {
			return err
		}
		panic("boom")
	})

	rr := newResponseRecorder()
	req := newRequest()

	defer func() {
		if rec := recover(); rec != "boom" {
			t.Fatalf("panic = %#v, want boom", rec)
		}
		if rr.Code != http.StatusOK {
			t.Fatalf("status = %d, want %d", rr.Code, http.StatusOK)
		}
		if got := rr.Body.String(); got != "partial" {
			t.Fatalf("body = %q, want partial", got)
		}
	}()

	h.ServeHTTP(rr, req)
}
