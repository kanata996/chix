package middleware

import (
	"bufio"
	"bytes"
	"encoding/json"
	"errors"
	"net"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestRecovererWritesInternalErrorOnPanic(t *testing.T) {
	logs := captureRecovererLogs(t)

	h := Recoverer()(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		panic("boom")
	}))

	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	h.ServeHTTP(rr, req)

	assertErrorResponse(t, rr, http.StatusInternalServerError, "internal_error", "internal server error")
	logText := logs.String()
	if logText == "" || !bytes.Contains(logs.Bytes(), []byte("panic recovered: boom")) {
		t.Fatalf("logs = %q, want panic details", logText)
	}
	if !bytes.Contains(logs.Bytes(), []byte("method=GET")) || !bytes.Contains(logs.Bytes(), []byte("target=/")) {
		t.Fatalf("logs = %q, want request details", logText)
	}
}

func TestRecovererLogsRequestIDFromHeader(t *testing.T) {
	logs := captureRecovererLogs(t)

	h := Recoverer()(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		panic("boom")
	}))

	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/panic", nil)
	req.Header.Set("X-Request-Id", "req-123")
	h.ServeHTTP(rr, req)

	if !bytes.Contains(logs.Bytes(), []byte("request_id=req-123")) {
		t.Fatalf("logs = %q, want request id", logs.String())
	}
}

func TestRecovererWithNilNextWritesInternalError(t *testing.T) {
	h := Recoverer()(nil)

	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	h.ServeHTTP(rr, req)

	assertErrorResponse(t, rr, http.StatusInternalServerError, "internal_error", "internal server error")
}

func TestRecovererDoesNotRewriteStartedResponseOnPanic(t *testing.T) {
	h := Recoverer()(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if _, err := w.Write([]byte("partial")); err != nil {
			panic(err)
		}
		panic("boom")
	}))

	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	h.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rr.Code, http.StatusOK)
	}
	if got := rr.Body.String(); got != "partial" {
		t.Fatalf("body = %q, want partial", got)
	}
}

func TestRecovererPropagatesErrAbortHandler(t *testing.T) {
	h := Recoverer()(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		panic(http.ErrAbortHandler)
	}))

	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/", nil)

	defer func() {
		rec := recover()
		err, ok := rec.(error)
		if !ok || !errors.Is(err, http.ErrAbortHandler) {
			t.Fatalf("panic = %#v, want %v", rec, http.ErrAbortHandler)
		}
	}()

	h.ServeHTTP(rr, req)
}

func TestRecovererTreatsBareConnectionUpgradeAsNormalHTTPRequest(t *testing.T) {
	h := Recoverer()(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		panic("boom")
	}))

	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("Connection", "keep-alive, Upgrade")
	h.ServeHTTP(rr, req)

	assertErrorResponse(t, rr, http.StatusInternalServerError, "internal_error", "internal server error")
}

func TestRecovererDoesNotWriteJSONOnActualUpgradeRequest(t *testing.T) {
	h := Recoverer()(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		panic("boom")
	}))

	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("Connection", "keep-alive, Upgrade")
	req.Header.Set("Upgrade", "websocket")
	h.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rr.Code, http.StatusOK)
	}
	if rr.Body.Len() != 0 {
		t.Fatalf("body = %q, want empty", rr.Body.String())
	}
	if got := rr.Header().Get("Content-Type"); got != "" {
		t.Fatalf("Content-Type = %q, want empty", got)
	}
}

type hijackableRecorder struct {
	*httptest.ResponseRecorder
	hijackCalls int
}

func (w *hijackableRecorder) Hijack() (net.Conn, *bufio.ReadWriter, error) {
	w.hijackCalls++
	return nil, nil, nil
}

func TestRecovererDoesNotWriteAfterHijack(t *testing.T) {
	h := Recoverer()(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hijacker, ok := w.(http.Hijacker)
		if !ok {
			t.Fatalf("writer should support hijacking")
		}
		if _, _, err := hijacker.Hijack(); err != nil {
			t.Fatalf("Hijack() error = %v", err)
		}
		panic("boom")
	}))

	rr := &hijackableRecorder{ResponseRecorder: httptest.NewRecorder()}
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	h.ServeHTTP(rr, req)

	if rr.hijackCalls != 1 {
		t.Fatalf("hijack calls = %d, want 1", rr.hijackCalls)
	}
	if rr.Body.Len() != 0 {
		t.Fatalf("body = %q, want empty", rr.Body.String())
	}
	if got := rr.Header().Get("Content-Type"); got != "" {
		t.Fatalf("Content-Type = %q, want empty", got)
	}
}

func TestRecovererUsesCustomPanicReporter(t *testing.T) {
	var report PanicReport
	var called bool

	h := Recoverer(WithPanicReporter(func(r PanicReport) {
		called = true
		report = r
	}))(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		panic("boom")
	}))

	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/panic", nil)
	req.RemoteAddr = "127.0.0.1:1234"
	h.ServeHTTP(rr, req)

	if !called {
		t.Fatalf("expected custom reporter to be called")
	}
	if report.Request != req {
		t.Fatalf("request = %#v, want original request", report.Request)
	}
	if report.Panic != "boom" {
		t.Fatalf("panic = %#v, want boom", report.Panic)
	}
	if len(report.Stack) == 0 {
		t.Fatalf("stack should not be empty")
	}
	if report.ResponseStarted {
		t.Fatalf("response should not be marked started")
	}
	if report.Upgrade {
		t.Fatalf("upgrade should be false")
	}
}

func TestRecovererReporterMarksStartedResponse(t *testing.T) {
	var report PanicReport

	h := Recoverer(WithPanicReporter(func(r PanicReport) {
		report = r
	}))(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if _, err := w.Write([]byte("partial")); err != nil {
			t.Fatalf("write error = %v", err)
		}
		panic("boom")
	}))

	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/started", nil)
	h.ServeHTTP(rr, req)

	if !report.ResponseStarted {
		t.Fatalf("response should be marked started")
	}
	if report.Upgrade {
		t.Fatalf("upgrade should be false")
	}
}

func TestRecovererReporterMarksUpgradeRequest(t *testing.T) {
	var report PanicReport

	h := Recoverer(WithPanicReporter(func(r PanicReport) {
		report = r
	}))(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		panic("boom")
	}))

	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/upgrade", nil)
	req.Header.Set("Connection", "keep-alive, Upgrade")
	req.Header.Set("Upgrade", "websocket")
	h.ServeHTTP(rr, req)

	if !report.Upgrade {
		t.Fatalf("upgrade should be true")
	}
	if report.ResponseStarted {
		t.Fatalf("response should not be marked started")
	}
}

func TestRecovererCanDisablePanicReporting(t *testing.T) {
	logs := captureRecovererLogs(t)

	h := Recoverer(WithPanicReporter(nil))(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		panic("boom")
	}))

	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	h.ServeHTTP(rr, req)

	if logs.Len() != 0 {
		t.Fatalf("logs = %q, want empty", logs.String())
	}
	assertErrorResponse(t, rr, http.StatusInternalServerError, "internal_error", "internal server error")
}

func TestIsUpgradeRequestNilRequest(t *testing.T) {
	if isUpgradeRequest(nil) {
		t.Fatal("nil request should not be treated as upgrade")
	}
}

func TestIsUpgradeRequestRequiresUpgradeHeader(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("Connection", "upgrade")

	if isUpgradeRequest(req) {
		t.Fatal("request without Upgrade header should not be treated as upgrade")
	}
}

func captureRecovererLogs(t *testing.T) *bytes.Buffer {
	t.Helper()

	var logs bytes.Buffer
	prevOutput := recovererLogger.Writer()
	prevFlags := recovererLogger.Flags()
	prevPrefix := recovererLogger.Prefix()

	recovererLogger.SetOutput(&logs)
	recovererLogger.SetFlags(0)
	recovererLogger.SetPrefix("")

	t.Cleanup(func() {
		recovererLogger.SetOutput(prevOutput)
		recovererLogger.SetFlags(prevFlags)
		recovererLogger.SetPrefix(prevPrefix)
	})

	return &logs
}

func assertErrorResponse(t *testing.T, rr *httptest.ResponseRecorder, status int, code, message string) {
	t.Helper()

	if rr.Code != status {
		t.Fatalf("status = %d, want %d", rr.Code, status)
	}
	if got := rr.Header().Get("Content-Type"); got != "application/json" {
		t.Fatalf("Content-Type = %q, want application/json", got)
	}

	var payload map[string]any
	if err := json.Unmarshal(rr.Body.Bytes(), &payload); err != nil {
		t.Fatalf("unmarshal response: %v", err)
	}

	rawError, ok := payload["error"].(map[string]any)
	if !ok {
		t.Fatalf("error envelope missing or wrong type: %#v", payload["error"])
	}
	if got := rawError["code"]; got != code {
		t.Fatalf("error.code = %#v, want %q", got, code)
	}
	if got := rawError["message"]; got != message {
		t.Fatalf("error.message = %#v, want %q", got, message)
	}
	if details, ok := rawError["details"].([]any); !ok || len(details) != 0 {
		t.Fatalf("error.details = %#v, want empty array", rawError["details"])
	}
}
