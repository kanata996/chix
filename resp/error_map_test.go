package resp

import (
	"encoding/json"
	"errors"
	"github.com/kanata996/chix/errx"
	"net/http"
	"net/http/httptest"
	"testing"
)

type stubMapper = errx.Mapper

func TestError_UsesMapperForFeatureError(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()
	featureErr := errors.New("feature conflict")

	Error(rec, req, featureErr, stubMapper(func(err error) errx.Mapping {
		return errx.Mapping{StatusCode: http.StatusConflict, Code: 499999, Message: "Custom Conflict"}
	}))

	if rec.Code != http.StatusConflict {
		t.Fatalf("status mismatch: want %d, got %d", http.StatusConflict, rec.Code)
	}

	var body envelope
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if body.Code != 499999 || body.Message != "Custom Conflict" {
		t.Fatalf("unexpected body: %#v", body)
	}
}

func TestError_BuiltInMappingTakesPrecedence(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()

	Error(rec, req, errx.ErrInvalidRequest, stubMapper(func(error) errx.Mapping {
		return errx.Mapping{StatusCode: http.StatusConflict, Code: 499999, Message: "Custom Conflict"}
	}))

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status mismatch: want %d, got %d", http.StatusBadRequest, rec.Code)
	}

	var body envelope
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if body.Code != errx.CodeInvalidRequest || body.Message != http.StatusText(http.StatusBadRequest) {
		t.Fatalf("unexpected body: %#v", body)
	}
}

func TestError_DefaultMapper(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()
	Error(rec, req, errx.ErrUnauthorized, nil)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status mismatch: want %d, got %d", http.StatusUnauthorized, rec.Code)
	}
}

func TestError_Fallback(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()
	Error(rec, req, errors.New("unknown"), nil)

	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("status mismatch: want %d, got %d", http.StatusInternalServerError, rec.Code)
	}

	var body envelope
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if body.Code != errx.CodeInternal {
		t.Fatalf("code mismatch: want %d, got %d", errx.CodeInternal, body.Code)
	}
}

func TestError_InvalidMapperFailsClosed(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()
	featureErr := errors.New("feature unauthorized")

	Error(rec, req, featureErr, stubMapper(func(error) errx.Mapping {
		return errx.Mapping{}
	}))

	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("status mismatch: want %d, got %d", http.StatusInternalServerError, rec.Code)
	}

	var body envelope
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if body.Code != errx.CodeInternal || body.Message != http.StatusText(http.StatusInternalServerError) {
		t.Fatalf("unexpected body: %#v", body)
	}
}

func TestError_ReservedCodeMismatchFailsClosed(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()
	featureErr := errors.New("feature unauthorized")

	Error(rec, req, featureErr, stubMapper(func(error) errx.Mapping {
		return errx.Mapping{
			StatusCode: http.StatusConflict,
			Code:       errx.CodeUnauthorized,
			Message:    http.StatusText(http.StatusUnauthorized),
		}
	}))

	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("status mismatch: want %d, got %d", http.StatusInternalServerError, rec.Code)
	}

	var body envelope
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if body.Code != errx.CodeInternal || body.Message != http.StatusText(http.StatusInternalServerError) {
		t.Fatalf("unexpected body: %#v", body)
	}
}
