package chix_test

import (
	"errors"
	"net/http"
	"testing"

	"github.com/kanata996/chix"
)

func TestChainMappersReturnsFirstMatch(t *testing.T) {
	target := errors.New("target")
	mapper := chix.ChainMappers(
		func(err error) *chix.Error { return nil },
		func(err error) *chix.Error {
			if errors.Is(err, target) {
				return chix.DomainError(http.StatusConflict, "conflict", "conflict")
			}
			return nil
		},
		func(err error) *chix.Error {
			return chix.DomainError(http.StatusBadRequest, "wrong", "wrong")
		},
	)

	mapped := mapper(target)
	if mapped == nil {
		t.Fatalf("expected mapped error, got nil")
	}
	if got := mapped.Code(); got != "conflict" {
		t.Fatalf("code = %q, want conflict", got)
	}
}

func TestWithErrorMapperIgnoresNil(t *testing.T) {
	target := errors.New("target")
	handler := chix.Wrap(
		func(w http.ResponseWriter, r *http.Request) error {
			return target
		},
		chix.WithErrorMapper(nil),
	)

	rr := newResponseRecorder()
	req := newRequest()
	handler.ServeHTTP(rr, req)

	assertErrorResponse(t, rr, http.StatusInternalServerError, "internal_error", "internal server error")
}

func TestChainMappersReturnsNilWhenEmpty(t *testing.T) {
	if mapper := chix.ChainMappers(); mapper != nil {
		t.Fatalf("expected nil mapper, got %#v", mapper)
	}
}

func TestChainMappersReturnsNilWhenNoMapperMatches(t *testing.T) {
	mapper := chix.ChainMappers(
		func(err error) *chix.Error { return nil },
	)

	if mapped := mapper(errors.New("miss")); mapped != nil {
		t.Fatalf("expected nil mapped error, got %#v", mapped)
	}
}
