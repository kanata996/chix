package resp

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-chi/chi/v5"
	chimiddleware "github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/httplog/v3"
)

type failingWriter struct {
	header http.Header
	status int
	writes int
}

type rawTestError struct {
	message string
}

func (e *rawTestError) Error() string {
	return e.message
}

type wrappedTestError struct {
	op  string
	err error
}

func (e *wrappedTestError) Error() string {
	return fmt.Sprintf("%s: %v", e.op, e.err)
}

func (e *wrappedTestError) Unwrap() error {
	return e.err
}

func (w *failingWriter) Header() http.Header {
	if w.header == nil {
		w.header = make(http.Header)
	}
	return w.header
}

func (w *failingWriter) WriteHeader(status int) {
	w.status = status
}

func (w *failingWriter) Write(_ []byte) (int, error) {
	w.writes++
	return 0, errors.New("socket closed")
}

func TestWriteErrorWritesEnvelope(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rr := httptest.NewRecorder()

	err := WriteError(rr, req, NewError(
		http.StatusUnprocessableEntity,
		"",
		"",
		map[string]any{"field": "name", "code": "required"},
	))
	if err != nil {
		t.Fatalf("WriteError() error = %v", err)
	}

	if rr.Code != http.StatusUnprocessableEntity {
		t.Fatalf("status = %d, want %d", rr.Code, http.StatusUnprocessableEntity)
	}

	payload := decodePayload(t, rr.Body.Bytes())
	body, ok := payload["error"].(map[string]any)
	if !ok {
		t.Fatalf("error body = %#v, want object", payload["error"])
	}
	if got := body["code"]; got != "unprocessable_entity" {
		t.Fatalf("error.code = %#v, want unprocessable_entity", got)
	}
	if got := body["message"]; got != "Unprocessable Entity" {
		t.Fatalf("error.message = %#v, want Unprocessable Entity", got)
	}
	details, ok := body["details"].([]any)
	if !ok || len(details) != 1 {
		t.Fatalf("error.details = %#v, want 1 detail", body["details"])
	}
}

func TestWriteErrorNilErrorIsNoop(t *testing.T) {
	rr := httptest.NewRecorder()

	if err := WriteError(rr, httptest.NewRequest(http.MethodGet, "/", nil), nil); err != nil {
		t.Fatalf("WriteError() error = %v", err)
	}
	if rr.Body.Len() != 0 {
		t.Fatalf("body = %q, want empty", rr.Body.String())
	}
}

func TestWriteErrorHeadWritesStatusWithoutBody(t *testing.T) {
	req := httptest.NewRequest(http.MethodHead, "/", nil)
	rr := httptest.NewRecorder()

	err := WriteError(rr, req, NewError(http.StatusBadRequest, "", "", "detail"))
	if err != nil {
		t.Fatalf("WriteError() error = %v", err)
	}
	if rr.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", rr.Code, http.StatusBadRequest)
	}
	if got := rr.Header().Get("Content-Type"); got != "application/json" {
		t.Fatalf("Content-Type = %q, want application/json", got)
	}
	if rr.Body.Len() != 0 {
		t.Fatalf("body = %q, want empty", rr.Body.String())
	}
}

func TestWriteErrorNormalizesInvalidStatusAndHidesCause(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rr := httptest.NewRecorder()

	err := WriteError(rr, req, wrapError(99, "", "", errors.New("db timeout")))
	if err != nil {
		t.Fatalf("WriteError() error = %v", err)
	}

	if rr.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want %d", rr.Code, http.StatusInternalServerError)
	}

	payload := decodePayload(t, rr.Body.Bytes())
	body, ok := payload["error"].(map[string]any)
	if !ok {
		t.Fatalf("error body = %#v, want object", payload["error"])
	}
	if got := body["code"]; got != "internal_error" {
		t.Fatalf("error.code = %#v, want internal_error", got)
	}
	if got := body["message"]; got != "Internal Server Error" {
		t.Fatalf("error.message = %#v, want Internal Server Error", got)
	}
	if details, ok := body["details"].([]any); !ok || len(details) != 0 {
		t.Fatalf("error.details = %#v, want empty array", body["details"])
	}
	if bytes.Contains(rr.Body.Bytes(), []byte("db timeout")) {
		t.Fatalf("body leaked internal cause: %q", rr.Body.String())
	}
}

func TestWriteErrorMapsContextCanceled(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rr := httptest.NewRecorder()

	err := WriteError(rr, req, context.Canceled)
	if err != nil {
		t.Fatalf("WriteError() error = %v", err)
	}

	payload := decodePayload(t, rr.Body.Bytes())
	body, ok := payload["error"].(map[string]any)
	if !ok {
		t.Fatalf("error body = %#v, want object", payload["error"])
	}
	if rr.Code != 499 {
		t.Fatalf("status = %d, want 499", rr.Code)
	}
	if got := body["code"]; got != "client_closed_request" {
		t.Fatalf("error.code = %#v, want client_closed_request", got)
	}
	if got := body["message"]; got != "Client Closed Request" {
		t.Fatalf("error.message = %#v, want Client Closed Request", got)
	}
}

func TestWriteErrorMapsContextDeadlineExceeded(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rr := httptest.NewRecorder()

	err := WriteError(rr, req, context.DeadlineExceeded)
	if err != nil {
		t.Fatalf("WriteError() error = %v", err)
	}

	payload := decodePayload(t, rr.Body.Bytes())
	body, ok := payload["error"].(map[string]any)
	if !ok {
		t.Fatalf("error body = %#v, want object", payload["error"])
	}
	if rr.Code != http.StatusGatewayTimeout {
		t.Fatalf("status = %d, want %d", rr.Code, http.StatusGatewayTimeout)
	}
	if got := body["code"]; got != "timeout" {
		t.Fatalf("error.code = %#v, want timeout", got)
	}
	if got := body["message"]; got != http.StatusText(http.StatusGatewayTimeout) {
		t.Fatalf("error.message = %#v, want %q", got, http.StatusText(http.StatusGatewayTimeout))
	}
}

func TestWriteErrorMapsUnknownErrorToInternalError(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rr := httptest.NewRecorder()

	err := WriteError(rr, req, errors.New("db timeout"))
	if err != nil {
		t.Fatalf("WriteError() error = %v", err)
	}

	if rr.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want %d", rr.Code, http.StatusInternalServerError)
	}

	payload := decodePayload(t, rr.Body.Bytes())
	body, ok := payload["error"].(map[string]any)
	if !ok {
		t.Fatalf("error body = %#v, want object", payload["error"])
	}
	if got := body["code"]; got != "internal_error" {
		t.Fatalf("error.code = %#v, want internal_error", got)
	}
	if got := body["message"]; got != "Internal Server Error" {
		t.Fatalf("error.message = %#v, want Internal Server Error", got)
	}
	if bytes.Contains(rr.Body.Bytes(), []byte("db timeout")) {
		t.Fatalf("body leaked internal cause: %q", rr.Body.String())
	}
}

func TestWriteErrorDropsUnencodableDetails(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rr := httptest.NewRecorder()

	err := WriteError(rr, req, NewError(
		http.StatusBadRequest,
		"bad_request",
		"bad request",
		func() {},
	))
	if err == nil {
		t.Fatal("expected degraded write error, got nil")
	}

	var degraded *ErrorWriteDegraded
	if !errors.As(err, &degraded) || degraded == nil {
		t.Fatalf("WriteError() error = %T, want *ErrorWriteDegraded", err)
	}
	if !degraded.PreservedPublicResponse {
		t.Fatal("degraded.PreservedPublicResponse = false, want true")
	}

	payload := decodePayload(t, rr.Body.Bytes())
	body, ok := payload["error"].(map[string]any)
	if !ok {
		t.Fatalf("error body = %#v, want object", payload["error"])
	}
	if details, ok := body["details"].([]any); !ok || len(details) != 0 {
		t.Fatalf("error.details = %#v, want empty array", body["details"])
	}
}

func TestErrorWriteDegradedMethods(t *testing.T) {
	var nilErr *ErrorWriteDegraded
	if got := nilErr.Error(); got != "resp: error response details were dropped" {
		t.Fatalf("nil Error() = %q", got)
	}
	if got := nilErr.Unwrap(); got != nil {
		t.Fatalf("nil Unwrap() = %v, want nil", got)
	}

	cause := errors.New("json: unsupported type: func()")
	err := &ErrorWriteDegraded{Cause: cause}
	if got := err.Error(); got != "resp: error response details were dropped: "+cause.Error() {
		t.Fatalf("Error() = %q", got)
	}
	if got := err.Unwrap(); !errors.Is(got, cause) {
		t.Fatalf("Unwrap() = %v, want %v", got, cause)
	}
}

func TestAsHTTPError(t *testing.T) {
	if got := asHTTPError(nil); got != nil {
		t.Fatalf("asHTTPError(nil) = %#v, want nil", got)
	}

	httpErr := NewError(http.StatusBadRequest, "bad_request", "bad request")
	if got := asHTTPError(httpErr); got != httpErr {
		t.Fatalf("asHTTPError(httpErr) = %#v, want same pointer", got)
	}
}

func TestWriteErrorPayloadReturnsJoinedErrorWhenFallbackWriteFails(t *testing.T) {
	w := &failingWriter{}

	err := writeErrorPayload(w, http.StatusBadRequest, "bad_request", "bad request", []any{func() {}})
	if err == nil {
		t.Fatal("expected error, got nil")
	}

	var degraded *ErrorWriteDegraded
	if !errors.As(err, &degraded) {
		t.Fatalf("error = %T, want joined *ErrorWriteDegraded", err)
	}

	var writeErr *responseWriteError
	if !errors.As(err, &writeErr) {
		t.Fatalf("error = %T, want joined *responseWriteError", err)
	}
	if w.status != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", w.status, http.StatusBadRequest)
	}
	if w.writes != 1 {
		t.Fatalf("writes = %d, want 1", w.writes)
	}
}

func TestWriteErrorSkipsRewriteAfterResponseStarted(t *testing.T) {
	w := &failingWriter{}

	err := OK(w, nil, map[string]any{"id": "u_1"})
	if err == nil {
		t.Fatal("expected write error, got nil")
	}
	if w.status != http.StatusOK {
		t.Fatalf("status = %d, want %d", w.status, http.StatusOK)
	}
	if w.writes != 1 {
		t.Fatalf("writes = %d, want 1", w.writes)
	}

	writtenErr := WriteError(w, httptest.NewRequest(http.MethodGet, "/", nil), err)
	if !errors.Is(writtenErr, err) {
		t.Fatalf("WriteError() error = %v, want original error %v", writtenErr, err)
	}
	if w.writes != 1 {
		t.Fatalf("writes = %d, want still 1", w.writes)
	}
}

func TestWriteErrorEnrichesRequestLog(t *testing.T) {
	var buf bytes.Buffer

	logger := slog.New(slog.NewJSONHandler(&buf, nil))
	r := chi.NewRouter()
	r.Use(httplog.RequestLogger(logger, &httplog.Options{
		Level:         slog.LevelInfo,
		Schema:        httplog.SchemaECS,
		RecoverPanics: true,
	}))
	r.Use(chimiddleware.RequestID)
	r.Get("/users/{id}", func(w http.ResponseWriter, r *http.Request) {
		rawErr := &rawTestError{message: "db timeout"}
		err := wrapError(
			http.StatusInternalServerError,
			"internal_error",
			"",
			&wrappedTestError{op: "load user", err: rawErr},
			map[string]any{"field": "name", "code": "required"},
		)
		_ = WriteError(w, r, err)
	})

	req := httptest.NewRequest(http.MethodGet, "/users/u_123", nil)
	req.Header.Set(chimiddleware.RequestIDHeader, "req-123")
	rr := httptest.NewRecorder()
	r.ServeHTTP(rr, req)

	if rr.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want %d", rr.Code, http.StatusInternalServerError)
	}

	logEntry := decodePayload(t, buf.Bytes())
	if got := logEntry["error.code"]; got != "internal_error" {
		t.Fatalf("error.code = %#v, want internal_error", got)
	}
	if got := logEntry["error.category"]; got != "server" {
		t.Fatalf("error.category = %#v, want server", got)
	}
	if got := logEntry["error.message"]; got != "load user: db timeout" {
		t.Fatalf("error.message = %#v, want wrapped raw error", got)
	}
	if got := logEntry["error.public_message"]; got != "Internal Server Error" {
		t.Fatalf("error.public_message = %#v, want Internal Server Error", got)
	}
	if got := logEntry["error.type"]; got != "*resp.wrappedTestError" {
		t.Fatalf("error.type = %#v, want *resp.wrappedTestError", got)
	}
	if got := logEntry["error.root_message"]; got != "db timeout" {
		t.Fatalf("error.root_message = %#v, want db timeout", got)
	}
	if got := logEntry["error.root_type"]; got != "*resp.rawTestError" {
		t.Fatalf("error.root_type = %#v, want *resp.rawTestError", got)
	}
	if got := logEntry["error.expected"]; got != false {
		t.Fatalf("error.expected = %#v, want false", got)
	}
	if got := logEntry["error.wrapped"]; got != true {
		t.Fatalf("error.wrapped = %#v, want true", got)
	}
	if got := logEntry["error.details_count"]; got != float64(1) {
		t.Fatalf("error.details_count = %#v, want 1", got)
	}
	if got := logEntry["request.id"]; got != "req-123" {
		t.Fatalf("request.id = %#v, want req-123", got)
	}
	if got := logEntry["http.route"]; got != "/users/{id}" {
		t.Fatalf("http.route = %#v, want /users/{id}", got)
	}

	chain, ok := logEntry["error.chain"].([]any)
	if !ok || len(chain) != 2 {
		t.Fatalf("error.chain = %#v, want 2 entries", logEntry["error.chain"])
	}
	if chain[0] != "load user: db timeout" || chain[1] != "db timeout" {
		t.Fatalf("error.chain = %#v, want wrapped/raw messages", chain)
	}

	typeChain, ok := logEntry["error.chain_types"].([]any)
	if !ok || len(typeChain) != 2 {
		t.Fatalf("error.chain_types = %#v, want 2 entries", logEntry["error.chain_types"])
	}
	if typeChain[0] != "*resp.wrappedTestError" || typeChain[1] != "*resp.rawTestError" {
		t.Fatalf("error.chain_types = %#v, want wrapped/raw types", typeChain)
	}

	details, ok := logEntry["error.details"].([]any)
	if !ok || len(details) != 1 {
		t.Fatalf("error.details = %#v, want one entry", logEntry["error.details"])
	}
	detail0, ok := details[0].(map[string]any)
	if !ok {
		t.Fatalf("error.details[0] = %#v, want object", details[0])
	}
	if detail0["field"] != "name" || detail0["code"] != "required" {
		t.Fatalf("error.details[0] = %#v, want field/code pair", detail0)
	}
}
