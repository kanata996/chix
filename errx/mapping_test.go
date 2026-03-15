package errx

import (
	"context"
	"errors"
	"net/http"
	"strings"
	"testing"
)

func TestMapper_MapToStandardSemantic(t *testing.T) {
	errMissingActor := errors.New("actor missing")

	mapping := NewMapper(CodeInternal,
		MapTo(errMissingActor, ErrUnauthorized),
	).Map(errMissingActor)

	if mapping.StatusCode != http.StatusUnauthorized {
		t.Fatalf("status mismatch: want %d, got %d", http.StatusUnauthorized, mapping.StatusCode)
	}
	if mapping.Code != CodeUnauthorized {
		t.Fatalf("code mismatch: want %d, got %d", CodeUnauthorized, mapping.Code)
	}
	if mapping.Message != http.StatusText(http.StatusUnauthorized) {
		t.Fatalf("message mismatch: want %q, got %q", http.StatusText(http.StatusUnauthorized), mapping.Message)
	}
}

func TestMapper_UseResolvedStandardError(t *testing.T) {
	mapping := NewMapper(CodeInternal).Map(ErrUnauthorized)
	if mapping.StatusCode != http.StatusUnauthorized {
		t.Fatalf("status mismatch: want %d, got %d", http.StatusUnauthorized, mapping.StatusCode)
	}
	if mapping.Code != CodeUnauthorized {
		t.Fatalf("code mismatch: want %d, got %d", CodeUnauthorized, mapping.Code)
	}
}

func TestMapper_Fallback(t *testing.T) {
	mapping := NewMapper(777000).Map(errors.New("unknown"))
	if mapping.StatusCode != http.StatusInternalServerError {
		t.Fatalf("status mismatch: want %d, got %d", http.StatusInternalServerError, mapping.StatusCode)
	}
	if mapping.Code != 777000 {
		t.Fatalf("code mismatch: want %d, got %d", 777000, mapping.Code)
	}
}

func TestMappingValidate(t *testing.T) {
	tests := []struct {
		name    string
		mapping Mapping
		wantErr bool
	}{
		{
			name: "valid",
			mapping: Mapping{
				StatusCode: http.StatusConflict,
				Code:       CodeConflict,
				Message:    "Conflict",
			},
			wantErr: false,
		},
		{
			name: "invalid status",
			mapping: Mapping{
				StatusCode: http.StatusOK,
				Code:       CodeConflict,
				Message:    "Conflict",
			},
			wantErr: true,
		},
		{
			name: "invalid code",
			mapping: Mapping{
				StatusCode: http.StatusConflict,
				Code:       0,
				Message:    "Conflict",
			},
			wantErr: true,
		},
		{
			name: "blank message",
			mapping: Mapping{
				StatusCode: http.StatusConflict,
				Code:       CodeConflict,
				Message:    "  ",
			},
			wantErr: true,
		},
		{
			name: "reserved code status mismatch",
			mapping: Mapping{
				StatusCode: http.StatusConflict,
				Code:       CodeUnauthorized,
				Message:    http.StatusText(http.StatusUnauthorized),
			},
			wantErr: true,
		},
		{
			name: "reserved code message mismatch",
			mapping: Mapping{
				StatusCode: http.StatusUnauthorized,
				Code:       CodeUnauthorized,
				Message:    "custom unauthorized",
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.mapping.Validate()
			if got := err != nil; got != tt.wantErr {
				t.Fatalf("validate mismatch: want error=%v, got err=%v", tt.wantErr, err)
			}
		})
	}
}

func TestMappingValidateReason(t *testing.T) {
	err := (Mapping{}).Validate()
	if err == nil {
		t.Fatal("expected validation error")
	}
	if !strings.Contains(err.Error(), "status code must be 4xx/5xx or 499") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestNewMapper_InvalidConfigPanics(t *testing.T) {
	tests := []struct {
		name    string
		builder func()
		want    string
	}{
		{
			name: "invalid fallback code",
			builder: func() {
				NewMapper(0)
			},
			want: "internal code must be positive",
		},
		{
			name: "reserved fallback code",
			builder: func() {
				NewMapper(CodeUnauthorized)
			},
			want: "invalid fallback mapping",
		},
		{
			name: "zero rule",
			builder: func() {
				NewMapper(CodeInternal, Rule{})
			},
			want: "rule 0 match error must not be nil",
		},
		{
			name: "nil match error",
			builder: func() {
				MapTo(nil, ErrInvalidRequest)
			},
			want: "match error must not be nil",
		},
		{
			name: "invalid target",
			builder: func() {
				MapTo(errors.New("missing"), errors.New("unknown semantic"))
			},
			want: "target must resolve to a standard errx semantic",
		},
		{
			name: "transport target",
			builder: func() {
				MapTo(errors.New("missing"), context.Canceled)
			},
			want: "target must resolve to a standard errx semantic",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			defer func() {
				recovered := recover()
				if recovered == nil {
					t.Fatal("expected panic")
				}
				msg, ok := recovered.(string)
				if !ok {
					t.Fatalf("expected string panic, got %T", recovered)
				}
				if !strings.Contains(msg, tt.want) {
					t.Fatalf("panic mismatch: want substring %q, got %q", tt.want, msg)
				}
			}()

			tt.builder()
		})
	}
}
