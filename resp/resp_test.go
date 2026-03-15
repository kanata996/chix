package resp

import (
	"context"
	"encoding/json"
	"errors"
	"github.com/kanata996/chix/errx"
	"github.com/kanata996/chix/reqx"
	"net/http"
	"net/http/httptest"
	"testing"
)

type errWriter struct {
	header    http.Header
	status    int
	writeCall int
	lastWrite []byte
}

type nullJSONPayload struct{}

func (nullJSONPayload) MarshalJSON() ([]byte, error) {
	return []byte("null"), nil
}

func (w *errWriter) Header() http.Header {
	if w.header == nil {
		w.header = make(http.Header)
	}
	return w.header
}

func (w *errWriter) WriteHeader(statusCode int) {
	w.status = statusCode
}

func (w *errWriter) Write(p []byte) (int, error) {
	w.writeCall++
	w.lastWrite = append(w.lastWrite[:0], p...)
	return 0, errors.New("write failed")
}

func TestJSON_Success(t *testing.T) {
	rec := httptest.NewRecorder()
	writeJSON(rec, http.StatusAccepted, envelope{Code: 2002, Data: map[string]bool{"ok": true}})

	if rec.Code != http.StatusAccepted {
		t.Fatalf("status mismatch: want %d, got %d", http.StatusAccepted, rec.Code)
	}
	if got := rec.Header().Get("Content-Type"); got != "application/json" {
		t.Fatalf("content-type mismatch: want %q, got %q", "application/json", got)
	}

	var payload map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
		t.Fatalf("unmarshal response body failed: %v", err)
	}
	if got, ok := payload["code"].(float64); !ok || int64(got) != 2002 {
		t.Fatalf("code mismatch in body: %#v", payload["code"])
	}
}

func TestJSON_EncodeError(t *testing.T) {
	rec := httptest.NewRecorder()
	writeJSON(rec, http.StatusOK, func() {})

	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("status mismatch: want %d, got %d", http.StatusInternalServerError, rec.Code)
	}
	if got := rec.Header().Get("Content-Type"); got != "" {
		t.Fatalf("content-type mismatch: want empty, got %q", got)
	}
	if rec.Body.Len() != 0 {
		t.Fatalf("expected empty body, got %q", rec.Body.String())
	}
}

func TestJSON_WriteError(t *testing.T) {
	w := &errWriter{}
	writeJSON(w, http.StatusCreated, envelope{Code: 3003, Data: map[string]string{"a": "b"}})

	if w.status != http.StatusCreated {
		t.Fatalf("status mismatch: want %d, got %d", http.StatusCreated, w.status)
	}
	if got := w.Header().Get("Content-Type"); got != "application/json" {
		t.Fatalf("content-type mismatch: want %q, got %q", "application/json", got)
	}
	if w.writeCall != 1 {
		t.Fatalf("write call mismatch: want %d, got %d", 1, w.writeCall)
	}
	if len(w.lastWrite) == 0 {
		t.Fatal("expected non-empty write payload")
	}
}

func TestWriteErrorEnvelope_WithDetails(t *testing.T) {
	rec := httptest.NewRecorder()

	writeErrorEnvelope(rec, errx.Mapping{
		StatusCode: http.StatusConflict,
		Code:       499999,
		Message:    "Custom Conflict",
	}, []reqx.Detail{reqx.Required(reqx.InQuery, "limit")})

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
	if len(body.Details) != 1 || body.Details[0].Field != "limit" {
		t.Fatalf("unexpected details: %#v", body.Details)
	}
}

func TestRequestContext_NilRequestUsesBackground(t *testing.T) {
	if got := requestContext(nil); got != context.Background() {
		t.Fatalf("expected context.Background, got %#v", got)
	}
}

func TestMustMapping_PanicsOnInvalidMapping(t *testing.T) {
	defer func() {
		if recovered := recover(); recovered == nil {
			t.Fatal("expected panic for invalid mapping")
		}
	}()

	_ = mustMapping(errx.Mapping{})
}

func TestSuccess(t *testing.T) {
	rec := httptest.NewRecorder()
	Success(rec, map[string]any{"id": 1})

	if rec.Code != http.StatusOK {
		t.Fatalf("status mismatch: want %d, got %d", http.StatusOK, rec.Code)
	}
}

func TestSuccess_NilPayloadFailsClosed(t *testing.T) {
	rec := httptest.NewRecorder()
	Success(rec, nil)

	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("status mismatch: want %d, got %d", http.StatusInternalServerError, rec.Code)
	}
	if got := rec.Header().Get("Content-Type"); got != "" {
		t.Fatalf("content-type mismatch: want empty, got %q", got)
	}
	if rec.Body.Len() != 0 {
		t.Fatalf("expected empty body, got %q", rec.Body.String())
	}
}

func TestSuccess_TypedNilPayloadFailsClosed(t *testing.T) {
	rec := httptest.NewRecorder()

	var payload *struct{ ID int }
	Success(rec, payload)

	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("status mismatch: want %d, got %d", http.StatusInternalServerError, rec.Code)
	}
	if got := rec.Header().Get("Content-Type"); got != "" {
		t.Fatalf("content-type mismatch: want empty, got %q", got)
	}
	if rec.Body.Len() != 0 {
		t.Fatalf("expected empty body, got %q", rec.Body.String())
	}
}

func TestSuccess_NilSlicePayloadFailsClosed(t *testing.T) {
	rec := httptest.NewRecorder()

	var payload []int
	Success(rec, payload)

	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("status mismatch: want %d, got %d", http.StatusInternalServerError, rec.Code)
	}
	if got := rec.Header().Get("Content-Type"); got != "" {
		t.Fatalf("content-type mismatch: want empty, got %q", got)
	}
	if rec.Body.Len() != 0 {
		t.Fatalf("expected empty body, got %q", rec.Body.String())
	}
}

func TestSuccess_EmptySlicePayloadSucceeds(t *testing.T) {
	rec := httptest.NewRecorder()

	Success(rec, make([]int, 0))

	if rec.Code != http.StatusOK {
		t.Fatalf("status mismatch: want %d, got %d", http.StatusOK, rec.Code)
	}

	var body envelope
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if body.Data == nil {
		t.Fatalf("unexpected nil data: %#v", body)
	}
}

func TestSuccess_CustomNullJSONPayloadFailsClosed(t *testing.T) {
	rec := httptest.NewRecorder()

	Success(rec, nullJSONPayload{})

	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("status mismatch: want %d, got %d", http.StatusInternalServerError, rec.Code)
	}
	if got := rec.Header().Get("Content-Type"); got != "" {
		t.Fatalf("content-type mismatch: want empty, got %q", got)
	}
	if rec.Body.Len() != 0 {
		t.Fatalf("expected empty body, got %q", rec.Body.String())
	}
}

func TestEncodeSuccessData_UnencodablePayloadFailsClosed(t *testing.T) {
	rec := httptest.NewRecorder()

	body, ok := encodeSuccessData(rec, http.StatusOK, func() {})

	if ok {
		t.Fatal("expected encodeSuccessData to fail")
	}
	if body != nil {
		t.Fatalf("expected nil body, got %q", body)
	}
	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("status mismatch: want %d, got %d", http.StatusInternalServerError, rec.Code)
	}
	if got := rec.Header().Get("Content-Type"); got != "" {
		t.Fatalf("content-type mismatch: want empty, got %q", got)
	}
	if rec.Body.Len() != 0 {
		t.Fatalf("expected empty body, got %q", rec.Body.String())
	}
}

func TestFailClosedSuccess_NilWriterDoesNotPanic(t *testing.T) {
	defer func() {
		if recovered := recover(); recovered != nil {
			t.Fatalf("did not expect panic, got %#v", recovered)
		}
	}()

	failClosedSuccess(nil, http.StatusOK, "could not encode success response", errors.New("boom"))
}

func TestCreated(t *testing.T) {
	rec := httptest.NewRecorder()
	Created(rec, map[string]any{"id": 1})

	if rec.Code != http.StatusCreated {
		t.Fatalf("status mismatch: want %d, got %d", http.StatusCreated, rec.Code)
	}
}

func TestNoContent(t *testing.T) {
	rec := httptest.NewRecorder()

	NoContent(rec)

	if rec.Code != http.StatusNoContent {
		t.Fatalf("status mismatch: want %d, got %d", http.StatusNoContent, rec.Code)
	}
	if rec.Body.Len() != 0 {
		t.Fatalf("expected empty body, got %q", rec.Body.String())
	}
}
