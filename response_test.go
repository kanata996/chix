package chix_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/kanata996/chix"
)

type statusTrackingRecorder struct {
	*httptest.ResponseRecorder
	status       int
	bytesWritten int
}

func (w *statusTrackingRecorder) WriteHeader(status int) {
	if w.status == 0 {
		w.status = status
	}
	w.ResponseRecorder.WriteHeader(status)
}

func (w *statusTrackingRecorder) Write(p []byte) (int, error) {
	if w.status == 0 {
		w.status = http.StatusOK
	}
	n, err := w.ResponseRecorder.Write(p)
	w.bytesWritten += n
	return n, err
}

func (w *statusTrackingRecorder) Status() int {
	return w.status
}

func (w *statusTrackingRecorder) BytesWritten() int {
	return w.bytesWritten
}

func TestWriteSuccess(t *testing.T) {
	rr := newResponseRecorder()

	err := chix.Write(rr, http.StatusCreated, map[string]any{"id": "u_1"})
	if err != nil {
		t.Fatalf("Write() error = %v", err)
	}

	if rr.Code != http.StatusCreated {
		t.Fatalf("status = %d, want %d", rr.Code, http.StatusCreated)
	}
	if got := rr.Header().Get("Content-Type"); got != "application/json" {
		t.Fatalf("Content-Type = %q, want application/json", got)
	}

	var payload map[string]any
	if err := json.Unmarshal(rr.Body.Bytes(), &payload); err != nil {
		t.Fatalf("unmarshal response: %v", err)
	}

	data, ok := payload["data"].(map[string]any)
	if !ok {
		t.Fatalf("data envelope missing or wrong type: %#v", payload["data"])
	}
	if got := data["id"]; got != "u_1" {
		t.Fatalf("data.id = %#v, want u_1", got)
	}
	if _, ok := payload["meta"]; ok {
		t.Fatalf("meta should be omitted, payload = %#v", payload)
	}
}

func TestWriteMetaSuccess(t *testing.T) {
	rr := newResponseRecorder()

	err := chix.WriteMeta(
		rr,
		http.StatusOK,
		[]string{"a", "b"},
		map[string]any{"request_id": "req_1"},
	)
	if err != nil {
		t.Fatalf("WriteMeta() error = %v", err)
	}

	var payload map[string]any
	if err := json.Unmarshal(rr.Body.Bytes(), &payload); err != nil {
		t.Fatalf("unmarshal response: %v", err)
	}

	if _, ok := payload["data"].([]any); !ok {
		t.Fatalf("data should be an array, got %#v", payload["data"])
	}
	meta, ok := payload["meta"].(map[string]any)
	if !ok {
		t.Fatalf("meta missing or wrong type: %#v", payload["meta"])
	}
	if got := meta["request_id"]; got != "req_1" {
		t.Fatalf("meta.request_id = %#v, want req_1", got)
	}
}

func TestWriteEmpty(t *testing.T) {
	rr := newResponseRecorder()

	err := chix.WriteEmpty(rr, http.StatusNoContent)
	if err != nil {
		t.Fatalf("WriteEmpty() error = %v", err)
	}

	if rr.Code != http.StatusNoContent {
		t.Fatalf("status = %d, want %d", rr.Code, http.StatusNoContent)
	}
	if rr.Body.Len() != 0 {
		t.Fatalf("body = %q, want empty", rr.Body.String())
	}
	if got := rr.Header().Get("Content-Type"); got != "" {
		t.Fatalf("Content-Type = %q, want empty", got)
	}
}

func TestWriteRejectsInvalidSuccessInputs(t *testing.T) {
	cases := []struct {
		name string
		fn   func() error
	}{
		{
			name: "error status",
			fn: func() error {
				return chix.Write(httptest.NewRecorder(), http.StatusBadRequest, map[string]any{"ok": true})
			},
		},
		{
			name: "nil data",
			fn: func() error {
				return chix.Write(httptest.NewRecorder(), http.StatusOK, nil)
			},
		},
		{
			name: "invalid meta",
			fn: func() error {
				return chix.WriteMeta(httptest.NewRecorder(), http.StatusOK, map[string]any{"ok": true}, "bad-meta")
			},
		},
	}

	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			if err := tt.fn(); err == nil {
				t.Fatalf("expected error, got nil")
			}
		})
	}
}

func TestWriteErrorDoesNotRewriteStartedResponse(t *testing.T) {
	h := chix.Wrap(func(w http.ResponseWriter, r *http.Request) error {
		if _, err := w.Write([]byte("partial")); err != nil {
			return err
		}
		chix.WriteError(w, r, chix.RequestError(http.StatusUnauthorized, "unauthorized", "unauthorized"))
		return nil
	})

	rr := newResponseRecorder()
	req := newRequest()
	h.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rr.Code, http.StatusOK)
	}
	if got := rr.Body.String(); got != "partial" {
		t.Fatalf("body = %q, want partial", got)
	}
}

func TestWriteErrorDirect(t *testing.T) {
	rr := newResponseRecorder()
	req := newRequest()

	chix.WriteError(rr, req, chix.RequestError(http.StatusNotFound, "route_not_found", "route not found"))

	assertErrorResponse(t, rr, http.StatusNotFound, "route_not_found", "route not found")
}

func TestWriteErrorRateLimitDirect(t *testing.T) {
	rr := newResponseRecorder()
	req := newRequest()

	chix.WriteError(rr, req, chix.RequestError(http.StatusTooManyRequests, "rate_limited", "rate limit exceeded"))

	assertErrorResponse(t, rr, http.StatusTooManyRequests, "rate_limited", "rate limit exceeded")
}

func TestWriteErrorFromMiddlewareWritesRateLimitResponse(t *testing.T) {
	var nextCalled bool

	rateLimitMiddleware := func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			chix.WriteError(w, r, chix.RequestError(
				http.StatusTooManyRequests,
				"rate_limited",
				"rate limit exceeded",
			))
		})
	}

	h := rateLimitMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		nextCalled = true
		t.Fatal("next handler should not be called after rate limit rejection")
	}))

	rr := newResponseRecorder()
	req := newRequest()
	h.ServeHTTP(rr, req)

	if nextCalled {
		t.Fatal("next handler should not be reached")
	}
	assertErrorResponse(t, rr, http.StatusTooManyRequests, "rate_limited", "rate limit exceeded")
}

func TestWriteErrorNilErrorFallsBackToInternal(t *testing.T) {
	rr := newResponseRecorder()
	req := newRequest()

	chix.WriteError(rr, req, nil)

	assertErrorResponse(t, rr, http.StatusInternalServerError, "internal_error", "internal server error")
}

func TestWriteErrorDoesNotRewriteStartedStatusTrackingWriter(t *testing.T) {
	rr := &statusTrackingRecorder{ResponseRecorder: newResponseRecorder()}
	req := newRequest()

	if _, err := rr.Write([]byte("partial")); err != nil {
		t.Fatalf("Write() error = %v", err)
	}

	chix.WriteError(rr, req, chix.RequestError(http.StatusUnauthorized, "unauthorized", "unauthorized"))

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rr.Code, http.StatusOK)
	}
	if got := rr.Body.String(); got != "partial" {
		t.Fatalf("body = %q, want partial", got)
	}
}
