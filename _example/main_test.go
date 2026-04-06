package main

import (
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
)

func TestAccountStoreCreateAvoidsCompositeKeyCollision(t *testing.T) {
	store := newAccountStore()

	first, err := store.Create("org:a", "team")
	if err != nil {
		t.Fatalf("Create(first) error = %v", err)
	}

	second, err := store.Create("org", "a:team")
	if err != nil {
		t.Fatalf("Create(second) error = %v, want distinct accounts to succeed", err)
	}

	if first.ID == second.ID {
		t.Fatalf("first.ID = %q, second.ID = %q, want distinct IDs", first.ID, second.ID)
	}
}

func TestHealthzReturnsUnavailableWhileDraining(t *testing.T) {
	logger := slog.New(slog.NewJSONHandler(io.Discard, nil))
	var draining atomic.Bool
	router := newRouter(logger, newAccountStore(), &draining)

	healthzBefore := httptest.NewRecorder()
	router.ServeHTTP(healthzBefore, httptest.NewRequest(http.MethodGet, "/healthz", nil))
	if healthzBefore.Code != http.StatusNoContent {
		t.Fatalf("healthz before drain status = %d, want %d", healthzBefore.Code, http.StatusNoContent)
	}

	draining.Store(true)

	healthzAfter := httptest.NewRecorder()
	router.ServeHTTP(healthzAfter, httptest.NewRequest(http.MethodGet, "/healthz", nil))
	if healthzAfter.Code != http.StatusServiceUnavailable {
		t.Fatalf("healthz after drain status = %d, want %d", healthzAfter.Code, http.StatusServiceUnavailable)
	}

	var payload map[string]any
	if err := json.Unmarshal(healthzAfter.Body.Bytes(), &payload); err != nil {
		t.Fatalf("json.Unmarshal(healthz body) error = %v", err)
	}
	if got := payload["code"]; got != "service_unavailable" {
		t.Fatalf("healthz error code = %#v, want service_unavailable", got)
	}
}
