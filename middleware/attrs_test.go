package middleware

import (
	"bytes"
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-chi/chi/v5"
	chimw "github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/httplog/v3"
	"github.com/go-chi/traceid"
)

func TestRequestLogAttrs_EnrichesAccessLog(t *testing.T) {
	var buf bytes.Buffer

	logger := slog.New(slog.NewJSONHandler(&buf, nil))
	r := chi.NewRouter()
	r.Use(chimw.RequestID)
	r.Use(traceid.Middleware)
	r.Use(httplog.RequestLogger(logger, &httplog.Options{
		Level:         slog.LevelInfo,
		Schema:        httplog.SchemaECS,
		RecoverPanics: true,
	}))
	r.Use(RequestLogAttrs())
	r.Get("/users/{id}", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	})

	req := httptest.NewRequest(http.MethodGet, "/users/u_123", nil)
	req.Header.Set(chimw.RequestIDHeader, "req-123")
	rr := httptest.NewRecorder()
	r.ServeHTTP(rr, req)

	if rr.Code != http.StatusNoContent {
		t.Fatalf("status = %d, want %d", rr.Code, http.StatusNoContent)
	}

	var entry map[string]any
	if err := json.Unmarshal(bytes.TrimSpace(buf.Bytes()), &entry); err != nil {
		t.Fatalf("json.Unmarshal() error = %v", err)
	}

	if got := entry["request.id"]; got != "req-123" {
		t.Fatalf("request.id = %#v, want req-123", got)
	}
	if got, ok := entry["traceId"].(string); !ok || got == "" {
		t.Fatalf("traceId = %#v, want non-empty string", entry["traceId"])
	}
}

func TestRequestContextAttrs(t *testing.T) {
	if got := requestContextAttrs(nilContext()); got != nil {
		t.Fatalf("requestContextAttrs(nil) = %#v, want nil", got)
	}

	if got := requestContextAttrs(context.Background()); len(got) != 0 {
		t.Fatalf("requestContextAttrs(background) len = %d, want 0", len(got))
	}

	ctx := traceid.NewContext(context.Background())
	ctx = context.WithValue(ctx, chimw.RequestIDKey, "req-123")

	attrs := requestContextAttrs(ctx)
	if len(attrs) != 2 {
		t.Fatalf("requestContextAttrs() len = %d, want 2", len(attrs))
	}

	values := attrsToMap(attrs)
	if got := values["request.id"]; got != "req-123" {
		t.Fatalf("request.id = %#v, want req-123", got)
	}
	if got, ok := values["traceId"].(string); !ok || got == "" {
		t.Fatalf("traceId = %#v, want non-empty string", values["traceId"])
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
