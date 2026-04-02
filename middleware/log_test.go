package middleware

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"github.com/go-chi/chi/v5"
	chimw "github.com/go-chi/chi/v5/middleware"
)

// NewLogger 会为请求日志追加 app/version/env 等公共字段，并配合中间件写入 traceId。
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

// 传入普通 slog.Logger 时，中间件也应保证请求日志和上下文日志都带 traceId。
func TestRequestLogger_MakesPlainLoggerTraceAware(t *testing.T) {
	var buf bytes.Buffer

	logger := slog.New(slog.NewJSONHandler(&buf, nil))

	r := chi.NewRouter()
	r.Use(RequestLogger(RequestLoggerOptions{
		Logger: logger,
		Level:  slog.LevelInfo,
	}))
	r.Get("/trace", func(w http.ResponseWriter, r *http.Request) {
		ctxLogger := LoggerFromContext(r.Context())
		if ctxLogger == nil {
			t.Fatal("LoggerFromContext() = nil, want logger")
		}
		ctxLogger.InfoContext(r.Context(), "inside handler")
		w.WriteHeader(http.StatusNoContent)
	})

	req := httptest.NewRequest(http.MethodGet, "/trace", nil)
	rr := httptest.NewRecorder()

	r.ServeHTTP(rr, req)

	lines := splitLogLines(t, buf.Bytes())
	if len(lines) != 2 {
		t.Fatalf("log lines = %d, want 2", len(lines))
	}

	for i, line := range lines {
		if got := strings.Count(string(line), `"traceId"`); got != 1 {
			t.Fatalf("line %d traceId count = %d, want 1, payload = %s", i, got, line)
		}

		entry := decodeLogEntry(t, line)
		if got, ok := entry["traceId"].(string); !ok || got == "" {
			t.Fatalf("line %d traceId = %#v, want non-empty string", i, entry["traceId"])
		}
	}
}

// 已经 trace-aware 的 logger 不应被再次包装，否则会重复输出 traceId。
func TestRequestLogger_DoesNotDuplicateTraceIDForTraceAwareLogger(t *testing.T) {
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
	r.Get("/trace", func(w http.ResponseWriter, r *http.Request) {
		logger.InfoContext(r.Context(), "inside handler")
		w.WriteHeader(http.StatusNoContent)
	})

	req := httptest.NewRequest(http.MethodGet, "/trace", nil)
	rr := httptest.NewRecorder()

	r.ServeHTTP(rr, req)

	lines := splitLogLines(t, buf.Bytes())
	if len(lines) != 2 {
		t.Fatalf("log lines = %d, want 2", len(lines))
	}

	for i, line := range lines {
		if got := strings.Count(string(line), `"traceId"`); got != 1 {
			t.Fatalf("line %d traceId count = %d, want 1, payload = %s", i, got, line)
		}
	}
}

// RequestLogger 在成功请求上会注入稳定的基础请求日志字段。
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

// RequestLogger 在 panic 恢复路径上也会补齐基础请求日志字段和错误信息。
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

// RequestLogger 会把当前请求使用的 logger 存进上下文，便于下游复用。
func TestRequestLogger_StoresLoggerInContext(t *testing.T) {
	logger := NewLogger(LoggerOptions{
		Output: io.Discard,
		Level:  slog.LevelInfo,
	})

	r := chi.NewRouter()
	r.Use(RequestLogger(RequestLoggerOptions{
		Logger: logger,
		Level:  slog.LevelInfo,
	}))
	r.Get("/ctx", func(w http.ResponseWriter, r *http.Request) {
		if got := LoggerFromContext(r.Context()); got != logger {
			t.Fatalf("LoggerFromContext() = %p, want %p", got, logger)
		}
		w.WriteHeader(http.StatusNoContent)
	})

	req := httptest.NewRequest(http.MethodGet, "/ctx", nil)
	rr := httptest.NewRecorder()
	r.ServeHTTP(rr, req)

	if rr.Code != http.StatusNoContent {
		t.Fatalf("status = %d, want %d", rr.Code, http.StatusNoContent)
	}
}

// 没经过请求日志中间件的上下文不会携带 logger 或基础日志标记。
func TestLoggerContextHelpersReturnZeroValuesWithoutMiddleware(t *testing.T) {
	var nilCtx context.Context
	if got := LoggerFromContext(nilCtx); got != nil {
		t.Fatalf("LoggerFromContext(nil) = %p, want nil", got)
	}
	if got := LoggerFromContext(context.Background()); got != nil {
		t.Fatalf("LoggerFromContext(background) = %p, want nil", got)
	}
	if HasBaseRequestLogAttrs(nilCtx) {
		t.Fatal("HasBaseRequestLogAttrs(nil) = true, want false")
	}
	if HasBaseRequestLogAttrs(context.Background()) {
		t.Fatal("HasBaseRequestLogAttrs(background) = true, want false")
	}
}

// BaseRequestLogAttrs 会从请求对象中提取可复用的 request.id 和 http.route 字段。
func TestBaseRequestLogAttrs_ExtractsRequestIDAndRoute(t *testing.T) {
	var attrs []slog.Attr

	r := chi.NewRouter()
	r.Use(chimw.RequestID)
	r.Get("/users/{id}", func(w http.ResponseWriter, r *http.Request) {
		attrs = BaseRequestLogAttrs(r)
		w.WriteHeader(http.StatusNoContent)
	})

	req := httptest.NewRequest(http.MethodGet, "/users/u_1", nil)
	req.Header.Set(chimw.RequestIDHeader, "req-789")
	rr := httptest.NewRecorder()
	r.ServeHTTP(rr, req)

	values := attrsToMap(attrs)
	if got := values["request.id"]; got != "req-789" {
		t.Fatalf("request.id = %#v, want req-789", got)
	}
	if got := values["http.route"]; got != "/users/{id}" {
		t.Fatalf("http.route = %#v, want /users/{id}", got)
	}
}

// BaseRequestLogAttrs 遇到空请求时会安全返回 nil。
func TestBaseRequestLogAttrs_NilRequestReturnsNil(t *testing.T) {
	if got := BaseRequestLogAttrs(nil); got != nil {
		t.Fatalf("BaseRequestLogAttrs(nil) = %#v, want nil", got)
	}
}

// 禁用 panic recovery 后，中间件应让 panic 继续向上传播。
func TestRequestLogger_DisablePanicRecoveryLetsPanicEscape(t *testing.T) {
	logger := NewLogger(LoggerOptions{
		Output: io.Discard,
		Level:  slog.LevelInfo,
	})

	r := chi.NewRouter()
	r.Use(RequestLogger(RequestLoggerOptions{
		Logger:               logger,
		Level:                slog.LevelInfo,
		DisablePanicRecovery: true,
	}))
	r.Get("/panic", func(w http.ResponseWriter, r *http.Request) {
		panic("boom")
	})

	req := httptest.NewRequest(http.MethodGet, "/panic", nil)
	rr := httptest.NewRecorder()

	defer func() {
		if recovered := recover(); recovered != "boom" {
			t.Fatalf("recover() = %#v, want boom", recovered)
		}
	}()

	r.ServeHTTP(rr, req)
	t.Fatal("expected panic, got nil")
}

// 未显式提供 logger 时，请求日志中间件会回退到 slog.Default。
func TestRequestLogger_UsesDefaultLoggerWhenLoggerMissing(t *testing.T) {
	var buf bytes.Buffer

	previousDefault := slog.Default()
	slog.SetDefault(slog.New(slog.NewJSONHandler(&buf, nil)))
	defer slog.SetDefault(previousDefault)

	r := chi.NewRouter()
	r.Use(RequestLogger(RequestLoggerOptions{
		Level: slog.LevelInfo,
	}))
	r.Get("/ok", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	})

	req := httptest.NewRequest(http.MethodGet, "/ok", nil)
	rr := httptest.NewRecorder()
	r.ServeHTTP(rr, req)

	if rr.Code != http.StatusNoContent {
		t.Fatalf("status = %d, want %d", rr.Code, http.StatusNoContent)
	}
	if len(splitLogLines(t, buf.Bytes())) == 0 {
		t.Fatal("default logger did not capture output")
	}
}

// 禁用 trace id 后，请求日志中不应再出现 traceId 字段。
func TestRequestLogger_DisableTraceIDOmitsTraceIDField(t *testing.T) {
	var buf bytes.Buffer

	logger := NewLogger(LoggerOptions{
		Output: &buf,
		Level:  slog.LevelInfo,
	})
	r := chi.NewRouter()
	r.Use(RequestLogger(RequestLoggerOptions{
		Logger:         logger,
		Level:          slog.LevelInfo,
		DisableTraceID: true,
	}))
	r.Get("/ok", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	})

	req := httptest.NewRequest(http.MethodGet, "/ok", nil)
	rr := httptest.NewRecorder()
	r.ServeHTTP(rr, req)

	logEntry := decodeLogEntry(t, lastLogLine(t, buf.Bytes()))
	if _, exists := logEntry["traceId"]; exists {
		t.Fatalf("traceId unexpectedly present: %#v", logEntry["traceId"])
	}
}

// 禁用 request id 注入后，请求日志中不应再出现 request.id 字段。
func TestRequestLogger_DisableRequestIDOmitRequestIDField(t *testing.T) {
	var buf bytes.Buffer

	logger := NewLogger(LoggerOptions{
		Output: &buf,
		Level:  slog.LevelInfo,
	})
	r := chi.NewRouter()
	r.Use(RequestLogger(RequestLoggerOptions{
		Logger:           logger,
		Level:            slog.LevelInfo,
		DisableRequestID: true,
	}))
	r.Get("/ok", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	})

	req := httptest.NewRequest(http.MethodGet, "/ok", nil)
	rr := httptest.NewRecorder()
	r.ServeHTTP(rr, req)

	logEntry := decodeLogEntry(t, lastLogLine(t, buf.Bytes()))
	if _, exists := logEntry["request.id"]; exists {
		t.Fatalf("request.id unexpectedly present: %#v", logEntry["request.id"])
	}
}

// 默认输出为空时，NewLogger 会回退到 os.Stdout。
func TestNewLogger_UsesStdoutWhenOutputMissing(t *testing.T) {
	reader, writer, err := os.Pipe()
	if err != nil {
		t.Fatalf("os.Pipe() error = %v", err)
	}
	defer func() {
		if closeErr := reader.Close(); closeErr != nil {
			t.Fatalf("reader.Close() error = %v", closeErr)
		}
	}()

	previousStdout := os.Stdout
	os.Stdout = writer
	defer func() {
		os.Stdout = previousStdout
	}()

	logger := NewLogger(LoggerOptions{
		Level: slog.LevelInfo,
		App:   "stdout-app",
	})
	logger.Info("hello stdout")

	if err := writer.Close(); err != nil {
		t.Fatalf("writer.Close() error = %v", err)
	}
	output, err := io.ReadAll(reader)
	if err != nil {
		t.Fatalf("io.ReadAll() error = %v", err)
	}

	if len(output) == 0 {
		t.Fatal("stdout output is empty")
	}
	if !bytes.Contains(output, []byte("hello stdout")) {
		t.Fatalf("stdout output = %q, want message", output)
	}
	if !bytes.Contains(output, []byte("stdout-app")) {
		t.Fatalf("stdout output = %q, want app attr", output)
	}
}

// development 模式下会切换到开发友好的 handler，而不是 JSON handler。
func TestNewHandler_UsesDevelopmentHandler(t *testing.T) {
	var buf bytes.Buffer

	handler := newHandler(&buf, true, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	})
	slog.New(handler).Info("dev mode")

	output := buf.String()
	if output == "" {
		t.Fatal("development handler output is empty")
	}
	if strings.HasPrefix(strings.TrimSpace(output), "{") {
		t.Fatalf("development handler output looks like JSON: %q", output)
	}
	if !strings.Contains(output, "dev mode") {
		t.Fatalf("development handler output = %q, want message", output)
	}
}

// 基础日志上下文中间件在没有可补 attrs 时也应安全透传请求。
func TestRequestLogContextMiddleware_AllowsEmptyBaseAttrs(t *testing.T) {
	logger := slog.New(slog.NewJSONHandler(io.Discard, nil))
	middleware := requestLogContextMiddleware(logger)

	handler := middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !HasBaseRequestLogAttrs(r.Context()) {
			t.Fatal("HasBaseRequestLogAttrs() = false, want true")
		}
		if got := LoggerFromContext(r.Context()); got != logger {
			t.Fatalf("LoggerFromContext() = %p, want %p", got, logger)
		}
		w.WriteHeader(http.StatusNoContent)
	}))

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusNoContent {
		t.Fatalf("status = %d, want %d", rr.Code, http.StatusNoContent)
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

func attrsToMap(attrs []slog.Attr) map[string]any {
	out := make(map[string]any, len(attrs))
	for _, attr := range attrs {
		out[attr.Key] = attr.Value.Any()
	}
	return out
}
