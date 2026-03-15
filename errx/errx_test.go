package errx

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"testing"
)

func TestResolve_StandardErrors(t *testing.T) {
	tests := []struct {
		name       string
		err        error
		statusCode int
		code       int64
		message    string
	}{
		{name: "invalid request", err: ErrInvalidRequest, statusCode: http.StatusBadRequest, code: CodeInvalidRequest, message: "Bad Request"},
		{name: "unauthorized", err: ErrUnauthorized, statusCode: http.StatusUnauthorized, code: CodeUnauthorized, message: "Unauthorized"},
		{name: "forbidden", err: ErrForbidden, statusCode: http.StatusForbidden, code: CodeForbidden, message: "Forbidden"},
		{name: "not found", err: ErrNotFound, statusCode: http.StatusNotFound, code: CodeNotFound, message: "Not Found"},
		{name: "conflict", err: ErrConflict, statusCode: http.StatusConflict, code: CodeConflict, message: "Conflict"},
		{name: "unprocessable", err: ErrUnprocessableEntity, statusCode: http.StatusUnprocessableEntity, code: CodeUnprocessableEntity, message: "Unprocessable Entity"},
		{name: "too many requests", err: ErrTooManyRequests, statusCode: http.StatusTooManyRequests, code: CodeTooManyRequests, message: "Too Many Requests"},
		{name: "service unavailable", err: ErrServiceUnavailable, statusCode: http.StatusServiceUnavailable, code: CodeServiceUnavailable, message: "Service Unavailable"},
		{name: "timeout", err: ErrTimeout, statusCode: http.StatusGatewayTimeout, code: CodeTimeout, message: "Gateway Timeout"},
		{name: "context canceled", err: context.Canceled, statusCode: 499, code: CodeClientClosed, message: "Client Closed Request"},
		{name: "context deadline exceeded", err: context.DeadlineExceeded, statusCode: http.StatusGatewayTimeout, code: CodeTimeout, message: "Gateway Timeout"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mapping, ok := Lookup(fmt.Errorf("wrapped: %w", tt.err))
			if !ok {
				t.Fatal("expected resolved mapping")
			}
			if mapping.StatusCode != tt.statusCode {
				t.Fatalf("status code mismatch: want %d, got %d", tt.statusCode, mapping.StatusCode)
			}
			if mapping.Code != tt.code {
				t.Fatalf("code mismatch: want %d, got %d", tt.code, mapping.Code)
			}
			if mapping.Message != tt.message {
				t.Fatalf("message mismatch: want %q, got %q", tt.message, mapping.Message)
			}
		})
	}
}

func TestResolve_Unknown(t *testing.T) {
	if _, ok := Lookup(errors.New("unknown")); ok {
		t.Fatal("expected no mapping for unknown error")
	}
}

func TestInternal(t *testing.T) {
	mapping := Internal(145500)
	if mapping.StatusCode != http.StatusInternalServerError {
		t.Fatalf("status mismatch: want %d, got %d", http.StatusInternalServerError, mapping.StatusCode)
	}
	if mapping.Code != 145500 {
		t.Fatalf("code mismatch: want %d, got %d", 145500, mapping.Code)
	}
	if mapping.Message != http.StatusText(http.StatusInternalServerError) {
		t.Fatalf("message mismatch: want %q, got %q", http.StatusText(http.StatusInternalServerError), mapping.Message)
	}
}

func TestInternal_InvalidCodePanics(t *testing.T) {
	defer func() {
		recovered := recover()
		if recovered == nil {
			t.Fatal("expected panic")
		}
		msg, ok := recovered.(string)
		if !ok {
			t.Fatalf("expected string panic, got %T", recovered)
		}
		if !strings.Contains(msg, "internal code must be positive") {
			t.Fatalf("panic mismatch: got %q", msg)
		}
	}()

	Internal(0)
}

func TestFormatChain(t *testing.T) {
	err := fmt.Errorf("service login: %w", fmt.Errorf("repo query: %w", errors.New("db timeout")))
	chain := FormatChain(err)
	if !strings.Contains(chain, "==>") {
		t.Fatalf("expected chain separator, got %q", chain)
	}
	if !strings.Contains(chain, "repo query") {
		t.Fatalf("expected repo node in chain, got %q", chain)
	}
}

func TestFormatChain_MultiCause(t *testing.T) {
	err := fmt.Errorf("outer: %w", errors.Join(
		errors.New("cause A"),
		errors.New("cause B"),
	))
	chain := FormatChain(err)
	if !strings.Contains(chain, "cause A") {
		t.Fatalf("expected cause A in chain, got %q", chain)
	}
	if !strings.Contains(chain, "cause B") {
		t.Fatalf("expected cause B in chain, got %q", chain)
	}
}
