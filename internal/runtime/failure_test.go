package runtime

import (
	"errors"
	"net/http"
	"testing"
)

func TestExecutionConfigResolvePublicError(t *testing.T) {
	baseErr := errors.New("boom")

	t.Run("non-public error enters mappers", func(t *testing.T) {
		mapperCalls := 0
		cfg := executionConfig{
			errorMappers: []ErrorMapper{
				func(error) *HTTPError {
					mapperCalls++
					return &HTTPError{Status: http.StatusForbidden, Code: "mapped", Message: "mapped"}
				},
			},
		}

		got := cfg.resolvePublicError(baseErr, nil)

		if got.Code != "mapped" {
			t.Fatalf("expected mapper to handle non-public error, got %+v", got)
		}
		if mapperCalls != 1 {
			t.Fatalf("expected mapper to run for non-public error, got %d calls", mapperCalls)
		}
	})

	t.Run("wrapped HTTPError bypasses mappers", func(t *testing.T) {
		mapperCalls := 0
		public := &HTTPError{Status: http.StatusConflict, Code: "conflict", Message: "conflict"}
		cfg := executionConfig{
			errorMappers: []ErrorMapper{
				func(error) *HTTPError {
					mapperCalls++
					return &HTTPError{Status: http.StatusForbidden, Code: "mapped", Message: "mapped"}
				},
			},
		}

		got := cfg.resolvePublicError(errors.Join(errors.New("wrapper"), public), nil)

		if got.Code != "conflict" {
			t.Fatalf("expected wrapped public error passthrough, got %+v", got)
		}
		if mapperCalls != 0 {
			t.Fatalf("expected public error to bypass mappers, got %d calls", mapperCalls)
		}
	})

	t.Run("op mappers run before runtime mappers", func(t *testing.T) {
		var calls []string
		cfg := executionConfig{
			errorMappers: []ErrorMapper{
				func(err error) *HTTPError {
					if !errors.Is(err, baseErr) {
						t.Fatalf("unexpected runtime mapper input: %v", err)
					}
					calls = append(calls, "runtime")
					return &HTTPError{Status: http.StatusForbidden, Code: "runtime", Message: "runtime"}
				},
			},
		}

		got := cfg.resolvePublicError(baseErr, []ErrorMapper{
			func(err error) *HTTPError {
				if !errors.Is(err, baseErr) {
					t.Fatalf("unexpected op mapper input: %v", err)
				}
				calls = append(calls, "op")
				return &HTTPError{Status: http.StatusNotFound, Code: "op", Message: "op"}
			},
		})

		if got.Code != "op" {
			t.Fatalf("expected op mapper precedence, got %+v", got)
		}
		if len(calls) != 1 || calls[0] != "op" {
			t.Fatalf("expected only op mapper to run, got %+v", calls)
		}
	})

	t.Run("internal fallback when no mapper matches", func(t *testing.T) {
		cfg := executionConfig{
			errorMappers: []ErrorMapper{
				func(error) *HTTPError { return nil },
			},
		}

		got := cfg.resolvePublicError(baseErr, []ErrorMapper{
			func(error) *HTTPError { return nil },
		})

		if got.Code != "internal_error" || got.Status != http.StatusInternalServerError {
			t.Fatalf("expected internal fallback, got %+v", got)
		}
	})
}
