package resp

import (
	"context"
	"errors"
	"github.com/kanata996/chix/errx"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
)

type countingHandler struct {
	mu          sync.Mutex
	count       int
	lastContext context.Context
	lastAttrs   map[string]any
}

func (h *countingHandler) Enabled(context.Context, slog.Level) bool { return true }
func (h *countingHandler) Handle(ctx context.Context, rec slog.Record) error {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.count++
	h.lastContext = ctx
	h.lastAttrs = make(map[string]any)
	rec.Attrs(func(attr slog.Attr) bool {
		h.lastAttrs[attr.Key] = attr.Value.Any()
		return true
	})
	return nil
}
func (h *countingHandler) WithAttrs([]slog.Attr) slog.Handler { return h }
func (h *countingHandler) WithGroup(string) slog.Handler      { return h }
func (h *countingHandler) Count() int {
	h.mu.Lock()
	defer h.mu.Unlock()
	return h.count
}
func (h *countingHandler) LastContext() context.Context {
	h.mu.Lock()
	defer h.mu.Unlock()
	return h.lastContext
}
func (h *countingHandler) LastAttr(key string) any {
	h.mu.Lock()
	defer h.mu.Unlock()
	if h.lastAttrs == nil {
		return nil
	}
	return h.lastAttrs[key]
}

func TestError_LogOnlyFor5xx(t *testing.T) {
	prev := slog.Default()
	defer slog.SetDefault(prev)

	counter := &countingHandler{}
	slog.SetDefault(slog.New(counter))

	req4xx := httptest.NewRequest(http.MethodGet, "/", nil)
	rec4xx := httptest.NewRecorder()
	Error(rec4xx, req4xx, errx.ErrInvalidRequest, nil)
	if rec4xx.Code != http.StatusBadRequest {
		t.Fatalf("status mismatch: want %d, got %d", http.StatusBadRequest, rec4xx.Code)
	}
	if counter.Count() != 0 {
		t.Fatalf("expected no error log for 4xx, got %d", counter.Count())
	}

	req5xx := httptest.NewRequest(http.MethodGet, "/", nil)
	rec5xx := httptest.NewRecorder()
	Error(rec5xx, req5xx, errors.New("boom"), nil)
	if rec5xx.Code != http.StatusInternalServerError {
		t.Fatalf("status mismatch: want %d, got %d", http.StatusInternalServerError, rec5xx.Code)
	}
	if counter.Count() != 1 {
		t.Fatalf("expected one error log for 5xx, got %d", counter.Count())
	}
}

func TestError_NoLogForContextCanceled(t *testing.T) {
	prev := slog.Default()
	defer slog.SetDefault(prev)

	counter := &countingHandler{}
	slog.SetDefault(slog.New(counter))

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()
	Error(rec, req, context.Canceled, nil)
	if rec.Code != 499 {
		t.Fatalf("status mismatch: want %d, got %d", 499, rec.Code)
	}
	if counter.Count() != 0 {
		t.Fatalf("expected no log for context.Canceled (499), got %d", counter.Count())
	}
}

func TestError_InvalidMapperLogsReason(t *testing.T) {
	prev := slog.Default()
	defer slog.SetDefault(prev)

	counter := &countingHandler{}
	slog.SetDefault(slog.New(counter))

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()
	Error(rec, req, errors.New("feature boom"), stubMapper(func(error) errx.Mapping {
		return errx.Mapping{}
	}))

	if counter.Count() != 1 {
		t.Fatalf("expected one error log, got %d", counter.Count())
	}
	if got := counter.LastAttr("invalid_mapping"); got != true {
		t.Fatalf("expected invalid_mapping=true, got %#v", got)
	}
	errAttr, ok := counter.LastAttr("invalid_mapping_error").(error)
	if !ok || errAttr == nil {
		t.Fatalf("expected invalid_mapping_error attr, got %#v", counter.LastAttr("invalid_mapping_error"))
	}
	if got := errAttr.Error(); got == "" {
		t.Fatal("expected invalid_mapping_error message")
	}
}
