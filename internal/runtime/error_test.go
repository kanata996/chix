package runtime

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"testing"
)

func TestNormalizeHTTPErrorCopiesAndNormalizesDetails(t *testing.T) {
	src := &HTTPError{
		Status:  http.StatusBadRequest,
		Code:    "bad_request",
		Message: "bad request",
		Details: []any{"first"},
	}

	got := normalizeHTTPError(src)

	if got == src {
		t.Fatal("expected normalizeHTTPError to clone input")
	}
	if got.Status != http.StatusBadRequest || got.Code != "bad_request" || got.Message != "bad request" {
		t.Fatalf("unexpected normalized error: %+v", got)
	}
	if len(got.Details) != 1 || got.Details[0] != "first" {
		t.Fatalf("unexpected details: %+v", got.Details)
	}

	src.Details[0] = "mutated"
	if got.Details[0] != "first" {
		t.Fatalf("expected details clone to be isolated, got %+v", got.Details)
	}
}

func TestNormalizeHTTPErrorPreservesEmptyDetailsAsArray(t *testing.T) {
	got := normalizeHTTPError(&HTTPError{
		Status:  http.StatusConflict,
		Code:    "conflict",
		Message: "conflict",
		Details: []any{},
	})

	if got.Details == nil {
		t.Fatal("expected empty details slice, got nil")
	}
	if len(got.Details) != 0 {
		t.Fatalf("expected empty details slice, got %+v", got.Details)
	}
}

func TestMarshalErrorEnvelopeAlwaysWritesDetailsArray(t *testing.T) {
	t.Run("nil details", func(t *testing.T) {
		payload, err := marshalErrorEnvelope(&HTTPError{
			Status:  http.StatusConflict,
			Code:    "conflict",
			Message: "conflict",
		})
		if err != nil {
			t.Fatalf("marshalErrorEnvelope returned error: %v", err)
		}

		assertDetailsArray(t, payload)
	})

	t.Run("empty details", func(t *testing.T) {
		payload, err := marshalErrorEnvelope(&HTTPError{
			Status:  http.StatusConflict,
			Code:    "conflict",
			Message: "conflict",
			Details: []any{},
		})
		if err != nil {
			t.Fatalf("marshalErrorEnvelope returned error: %v", err)
		}

		assertDetailsArray(t, payload)
	})
}

func TestPublicErrorExtractorsHandleWrappedValues(t *testing.T) {
	public := &HTTPError{Status: http.StatusConflict, Code: "conflict", Message: "conflict"}
	runtimeErr := newInvalidRequestError([]Violation{{
		Source:  "body",
		Field:   "name",
		Code:    "required",
		Message: "name is required",
	}})

	gotValue := publicErrorFromValue(fmt.Errorf("wrapped value: %w", public))
	if gotValue == nil || gotValue.Code != "conflict" {
		t.Fatalf("expected wrapped HTTPError passthrough, got %+v", gotValue)
	}
	if gotValue.Details == nil {
		t.Fatalf("expected wrapped HTTPError details to normalize to empty array, got %+v", gotValue)
	}

	gotRuntime := publicErrorFromRuntime(fmt.Errorf("wrapped runtime: %w", runtimeErr))
	if gotRuntime == nil || gotRuntime.Code != "invalid_request" || len(gotRuntime.Details) != 1 {
		t.Fatalf("expected wrapped runtime public error passthrough, got %+v", gotRuntime)
	}
}

func assertDetailsArray(t *testing.T, payload []byte) {
	t.Helper()

	var envelope struct {
		Error struct {
			DetailsRaw json.RawMessage `json:"details"`
		} `json:"error"`
	}
	if err := json.Unmarshal(payload, &envelope); err != nil {
		t.Fatalf("decode envelope: %v", err)
	}
	if string(envelope.Error.DetailsRaw) == "null" {
		t.Fatalf("expected details array, got %s", envelope.Error.DetailsRaw)
	}
	if len(envelope.Error.DetailsRaw) == 0 || envelope.Error.DetailsRaw[0] != '[' {
		t.Fatalf("expected details JSON array, got %s", envelope.Error.DetailsRaw)
	}
}

func TestHTTPErrorErrorStringFallback(t *testing.T) {
	if (*HTTPError)(nil).Error() != "" {
		t.Fatal("nil HTTPError should stringify to empty string")
	}
	if got := (&HTTPError{Message: "boom"}).Error(); got != "boom" {
		t.Fatalf("expected message fallback, got %q", got)
	}
	if got := (&HTTPError{Code: "boom_code"}).Error(); got != "boom_code" {
		t.Fatalf("expected code fallback, got %q", got)
	}
	if got := (&HTTPError{}).Error(); got != "http error" {
		t.Fatalf("expected generic fallback, got %q", got)
	}
}

func TestRequestShapeAndUnsupportedMediaTypeUnwrap(t *testing.T) {
	cause := errors.New("cause")

	if !errors.Is(newRequestShapeError(cause), cause) {
		t.Fatal("request shape error should unwrap original cause")
	}
	if !errors.Is(newUnsupportedMediaTypeError(cause), cause) {
		t.Fatal("unsupported media type error should unwrap original cause")
	}
}
