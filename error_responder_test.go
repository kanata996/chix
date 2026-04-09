package chix

import (
	"bytes"
	"context"
	"errors"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-chi/chi/v5"
	chimw "github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/httplog/v3"
	"github.com/go-chi/traceid"
	chixmiddleware "github.com/kanata996/chix/middleware"
)

func TestNewErrorResponderPreset(t *testing.T) {
	responder := NewErrorResponder()
	if responder == nil {
		t.Fatal("NewErrorResponder() = nil")
	}
	if responder.ContextAttrs == nil {
		t.Fatal("ContextAttrs = nil, want preset hook")
	}
	if responder.AnnotateRequestLog == nil {
		t.Fatal("AnnotateRequestLog = nil, want preset hook")
	}

	// Guard branches in the preset annotator should be harmless no-ops.
	responder.AnnotateRequestLog(nil, []slog.Attr{slog.String("k", "v")})
	responder.AnnotateRequestLog(httptest.NewRequest(http.MethodGet, "/", nil), nil)
	responder.AnnotateRequestLog(
		httptest.NewRequest(http.MethodGet, "/", nil),
		[]slog.Attr{slog.String("k", "v")},
	)
}

func TestChixErrorContextAttrs(t *testing.T) {
	if got := chixErrorContextAttrs(nilContext()); got != nil {
		t.Fatalf("chixErrorContextAttrs(nil) = %#v, want nil", got)
	}
	if got := chixErrorContextAttrs(context.Background()); len(got) != 0 {
		t.Fatalf("chixErrorContextAttrs(background) len = %d, want 0", len(got))
	}

	attrs := attrsToMap(chixErrorContextAttrs(traceid.NewContext(context.Background())))
	if got, ok := attrs["traceId"].(string); !ok || got == "" {
		t.Fatalf("traceId = %#v, want non-empty string", attrs["traceId"])
	}
}

func TestWriteErrorUsesRequestLogAttrsMiddlewareForCorrelationFields(t *testing.T) {
	var accessBuf bytes.Buffer

	logger := slog.New(slog.NewJSONHandler(&accessBuf, nil))
	r := chi.NewRouter()
	r.Use(chimw.RequestID)
	r.Use(traceid.Middleware)
	r.Use(httplog.RequestLogger(logger, &httplog.Options{
		Level:         slog.LevelInfo,
		Schema:        httplog.SchemaECS,
		RecoverPanics: true,
	}))
	r.Use(chixmiddleware.RequestLogAttrs())
	r.Get("/users/{id}", func(w http.ResponseWriter, r *http.Request) {
		_ = WriteError(w, r, errors.New("db timeout"))
	})

	req := httptest.NewRequest(http.MethodGet, "/users/u_123", nil)
	req.Header.Set(chimw.RequestIDHeader, "req-123")
	rr := httptest.NewRecorder()
	r.ServeHTTP(rr, req)

	if rr.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want %d", rr.Code, http.StatusInternalServerError)
	}

	entry := decodeRootPayload(t, bytes.TrimSpace(accessBuf.Bytes()))
	if got := entry["request.id"]; got != "req-123" {
		t.Fatalf("request.id = %#v, want req-123", got)
	}
	if got, ok := entry["traceId"].(string); !ok || got == "" {
		t.Fatalf("traceId = %#v, want non-empty string", entry["traceId"])
	}
	if got := entry["error.code"]; got != "internal_error" {
		t.Fatalf("error.code = %#v, want internal_error", got)
	}
}

func TestWriteErrorLogsServerErrorWithTraceID(t *testing.T) {
	var defaultBuf bytes.Buffer
	previousDefault := slog.Default()
	slog.SetDefault(slog.New(slog.NewJSONHandler(&defaultBuf, nil)))
	defer slog.SetDefault(previousDefault)

	r := chi.NewRouter()
	r.Use(chimw.RequestID)
	r.Use(traceid.Middleware)
	r.Get("/failure", func(w http.ResponseWriter, r *http.Request) {
		_ = WriteError(w, r, errors.New("db timeout"))
	})

	req := httptest.NewRequest(http.MethodGet, "/failure", nil)
	req.Header.Set(chimw.RequestIDHeader, "req-server")
	rr := httptest.NewRecorder()
	r.ServeHTTP(rr, req)

	if rr.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want %d", rr.Code, http.StatusInternalServerError)
	}
	if defaultBuf.Len() == 0 {
		t.Fatal("default logger did not capture output")
	}

	entry := decodeRootPayload(t, bytes.TrimSpace(defaultBuf.Bytes()))
	if got := entry["msg"]; got != "resp: request failed with server error" {
		t.Fatalf("msg = %#v, want resp: request failed with server error", got)
	}
	if got := entry["error.code"]; got != "internal_error" {
		t.Fatalf("error.code = %#v, want internal_error", got)
	}
	if got := entry["error.message"]; got != "db timeout" {
		t.Fatalf("error.message = %#v, want db timeout", got)
	}
	if _, exists := entry["request.id"]; exists {
		t.Fatalf("request.id unexpectedly present: %#v", entry["request.id"])
	}
	if got, ok := entry["traceId"].(string); !ok || got == "" {
		t.Fatalf("traceId = %#v, want non-empty string", entry["traceId"])
	}
}

func attrsToMap(attrs []slog.Attr) map[string]any {
	out := make(map[string]any, len(attrs))
	for _, attr := range attrs {
		out[attr.Key] = attr.Value.Any()
	}
	return out
}

func nilContext() context.Context {
	var ctx context.Context
	return ctx
}
