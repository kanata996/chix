package errx

import (
	"errors"
	"net/http"
	"strings"
	"testing"
)

type blankError struct {
	next error
}

func (e blankError) Error() string { return "   " }
func (e blankError) Unwrap() error { return e.next }

func TestPresetConstructors(t *testing.T) {
	tests := []struct {
		name       string
		mapping    Mapping
		statusCode int
		code       int64
		message    string
	}{
		{name: "forbidden", mapping: AsForbidden(403101, "forbidden"), statusCode: http.StatusForbidden, code: 403101, message: "forbidden"},
		{name: "unprocessable", mapping: AsUnprocessable(422101, "unprocessable"), statusCode: http.StatusUnprocessableEntity, code: 422101, message: "unprocessable"},
		{name: "too many requests", mapping: AsTooManyRequests(429101, "too many requests"), statusCode: http.StatusTooManyRequests, code: 429101, message: "too many requests"},
		{name: "service unavailable", mapping: AsServiceUnavailable(503101, "service unavailable"), statusCode: http.StatusServiceUnavailable, code: 503101, message: "service unavailable"},
		{name: "timeout", mapping: AsTimeout(504101, "timeout"), statusCode: http.StatusGatewayTimeout, code: 504101, message: "timeout"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.mapping.StatusCode != tt.statusCode {
				t.Fatalf("status mismatch: want %d, got %d", tt.statusCode, tt.mapping.StatusCode)
			}
			if tt.mapping.Code != tt.code {
				t.Fatalf("code mismatch: want %d, got %d", tt.code, tt.mapping.Code)
			}
			if tt.mapping.Message != tt.message {
				t.Fatalf("message mismatch: want %q, got %q", tt.message, tt.mapping.Message)
			}
		})
	}
}

func TestMapperMap_EdgeCases(t *testing.T) {
	t.Run("nil mapper uses built-in lookup", func(t *testing.T) {
		var mapper *Mapper

		got := mapper.Map(ErrForbidden)
		if got.StatusCode != http.StatusForbidden || got.Code != CodeForbidden {
			t.Fatalf("unexpected mapping: %#v", got)
		}
	})

	t.Run("zero value mapper returns zero mapping for unknown errors", func(t *testing.T) {
		got := (&Mapper{}).Map(errors.New("unknown"))
		if got != (Mapping{}) {
			t.Fatalf("expected zero mapping, got %#v", got)
		}
	})
}

func TestFormatChain_Edges(t *testing.T) {
	t.Run("nil error", func(t *testing.T) {
		if got := FormatChain(nil); got != "" {
			t.Fatalf("expected empty chain, got %q", got)
		}
	})

	t.Run("blank node is skipped", func(t *testing.T) {
		chain := FormatChain(blankError{next: errors.New("inner")})
		if chain != "inner" {
			t.Fatalf("unexpected chain: %q", chain)
		}
	})

	t.Run("depth is capped", func(t *testing.T) {
		err := errors.New("root")
		for i := 0; i < maxChainDepth+10; i++ {
			err = blankError{next: err}
		}

		chain := FormatChain(err)
		if strings.Count(chain, "==>") >= maxChainDepth {
			t.Fatalf("expected capped chain, got %q", chain)
		}
	})
}
