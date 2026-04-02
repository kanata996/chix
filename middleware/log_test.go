package middleware

import (
	"bytes"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-chi/chi/v5"
)

func TestNewLogger_AddsCommonAttrsAndTraceID(t *testing.T) {
	var buf bytes.Buffer

	logger := NewLogger(LoggerOptions{
		Output:  &buf,
		Level:   slog.LevelInfo,
		App:     "app-mall",
		Version: "test",
		Env:     "production",
	})

	r := chi.NewRouter()
	r.Use(RequestLogger(RequestLoggerOptions{
		Logger: logger,
		Level:  slog.LevelInfo,
	}))
	r.Get("/ok", func(w http.ResponseWriter, r *http.Request) {
		logger.InfoContext(r.Context(), "inside handler")
		w.WriteHeader(http.StatusNoContent)
	})

	req := httptest.NewRequest(http.MethodGet, "/ok", nil)
	rr := httptest.NewRecorder()

	r.ServeHTTP(rr, req)

	lines := splitLogLines(t, buf.Bytes())
	if len(lines) != 2 {
		t.Fatalf("log lines = %d, want 2", len(lines))
	}

	requestLog := decodeLogEntry(t, lines[1])
	if requestLog["app"] != "app-mall" {
		t.Fatalf("app = %#v, want app-mall", requestLog["app"])
	}
	if requestLog["version"] != "test" {
		t.Fatalf("version = %#v, want test", requestLog["version"])
	}
	if requestLog["env"] != "production" {
		t.Fatalf("env = %#v, want production", requestLog["env"])
	}
	if got, ok := requestLog["traceId"].(string); !ok || got == "" {
		t.Fatalf("traceId = %#v, want non-empty string", requestLog["traceId"])
	}
}

func TestRequestLogger_InjectsBaseRequestAttrsOnSuccess(t *testing.T) {
	var buf bytes.Buffer

	logger := NewLogger(LoggerOptions{
		Output: &buf,
		Level:  slog.LevelInfo,
	})
	r := chi.NewRouter()
	r.Use(RequestLogger(RequestLoggerOptions{
		Logger: logger,
		Level:  slog.LevelInfo,
	}))
	r.Get("/ok", func(w http.ResponseWriter, r *http.Request) {
		if !HasBaseRequestLogAttrs(r.Context()) {
			t.Fatal("HasBaseRequestLogAttrs() = false, want true")
		}
		w.WriteHeader(http.StatusNoContent)
	})

	req := httptest.NewRequest(http.MethodGet, "/ok", nil)
	req.Header.Set("X-Request-Id", "req-123")
	rr := httptest.NewRecorder()

	r.ServeHTTP(rr, req)

	if rr.Code != http.StatusNoContent {
		t.Fatalf("status = %d, want %d", rr.Code, http.StatusNoContent)
	}

	logEntry := decodeLogEntry(t, lastLogLine(t, buf.Bytes()))
	if got := logEntry["request.id"]; got != "req-123" {
		t.Fatalf("request.id = %#v, want req-123", got)
	}
	if got := logEntry["http.route"]; got != "/ok" {
		t.Fatalf("http.route = %#v, want /ok", got)
	}
}

func TestRequestLogger_InjectsBaseRequestAttrsOnPanic(t *testing.T) {
	var buf bytes.Buffer

	logger := NewLogger(LoggerOptions{
		Output: &buf,
		Level:  slog.LevelInfo,
	})
	r := chi.NewRouter()
	r.Use(RequestLogger(RequestLoggerOptions{
		Logger: logger,
		Level:  slog.LevelInfo,
	}))
	r.Get("/panic", func(w http.ResponseWriter, r *http.Request) {
		panic("boom")
	})

	req := httptest.NewRequest(http.MethodGet, "/panic", nil)
	req.Header.Set("X-Request-Id", "req-456")
	rr := httptest.NewRecorder()

	r.ServeHTTP(rr, req)

	if rr.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want %d", rr.Code, http.StatusInternalServerError)
	}

	logEntry := decodeLogEntry(t, lastLogLine(t, buf.Bytes()))
	if got := logEntry["request.id"]; got != "req-456" {
		t.Fatalf("request.id = %#v, want req-456", got)
	}
	if got := logEntry["http.route"]; got != "/panic" {
		t.Fatalf("http.route = %#v, want /panic", got)
	}
	if got := logEntry["error.message"]; got != "panic: boom" {
		t.Fatalf("error.message = %#v, want panic: boom", got)
	}
}

func decodeLogEntry(t *testing.T, payload []byte) map[string]any {
	t.Helper()

	var entry map[string]any
	if err := json.Unmarshal(payload, &entry); err != nil {
		t.Fatalf("json.Unmarshal() error = %v, payload = %s", err, payload)
	}
	return entry
}

func splitLogLines(t *testing.T, payload []byte) [][]byte {
	t.Helper()

	lines := bytes.Split(bytes.TrimSpace(payload), []byte{'\n'})
	if len(lines) == 1 && len(lines[0]) == 0 {
		return nil
	}
	return lines
}

func lastLogLine(t *testing.T, payload []byte) []byte {
	t.Helper()

	lines := splitLogLines(t, payload)
	if len(lines) == 0 {
		t.Fatal("no log lines captured")
	}
	return lines[len(lines)-1]
}
