package main

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

type statusTrackingRecorder struct {
	*httptest.ResponseRecorder
	status       int
	bytesWritten int
}

type failingResponseWriter struct {
	header http.Header
	status int
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

func (w *failingResponseWriter) Header() http.Header {
	if w.header == nil {
		w.header = make(http.Header)
	}
	return w.header
}

func (w *failingResponseWriter) WriteHeader(status int) {
	w.status = status
}

func (w *failingResponseWriter) Write(_ []byte) (int, error) {
	return 0, errors.New("write failed")
}

func TestChiIntegrationSmoke(t *testing.T) {
	router := newRouter()

	tests := []struct {
		name        string
		method      string
		path        string
		auth        string
		contentType string
		body        string
		assertions  func(t *testing.T, rr *statusTrackingRecorder)
	}{
		{
			name:   "list users success with meta",
			method: http.MethodGet,
			path:   "/users?page=2&limit=1&role=admin",
			auth:   "Bearer token",
			assertions: func(t *testing.T, rr *statusTrackingRecorder) {
				t.Helper()

				if rr.Code != http.StatusOK {
					t.Fatalf("status = %d, want %d", rr.Code, http.StatusOK)
				}

				payload := decodePayload(t, rr.ResponseRecorder)
				data, ok := payload["data"].([]any)
				if !ok || len(data) != 1 {
					t.Fatalf("data = %#v, want single-item array", payload["data"])
				}

				meta, ok := payload["meta"].(map[string]any)
				if !ok {
					t.Fatalf("meta envelope missing or wrong type: %#v", payload["meta"])
				}
				if got := meta["page"]; got != float64(2) {
					t.Fatalf("meta.page = %#v, want 2", got)
				}
				if got := meta["limit"]; got != float64(1) {
					t.Fatalf("meta.limit = %#v, want 1", got)
				}
			},
		},
		{
			name:   "list users validation error",
			method: http.MethodGet,
			path:   "/users?page=0",
			auth:   "Bearer token",
			assertions: func(t *testing.T, rr *statusTrackingRecorder) {
				t.Helper()
				assertErrorResponse(t, rr.ResponseRecorder, http.StatusUnprocessableEntity, "invalid_request", "request contains invalid fields")
				assertFirstDetail(t, rr.ResponseRecorder, "page", "min", "must be at least 1")
			},
		},
		{
			name:   "list users defaults role",
			method: http.MethodGet,
			path:   "/users",
			auth:   "Bearer token",
			assertions: func(t *testing.T, rr *statusTrackingRecorder) {
				t.Helper()

				if rr.Code != http.StatusOK {
					t.Fatalf("status = %d, want %d", rr.Code, http.StatusOK)
				}

				payload := decodePayload(t, rr.ResponseRecorder)
				data, ok := payload["data"].([]any)
				if !ok || len(data) == 0 {
					t.Fatalf("data = %#v, want non-empty array", payload["data"])
				}

				first, ok := data[0].(map[string]any)
				if !ok {
					t.Fatalf("data[0] = %#v, want object", data[0])
				}
				if got := first["role"]; got != "member" {
					t.Fatalf("data[0].role = %#v, want member", got)
				}
			},
		},
		{
			name:   "list users validation error for limit and role",
			method: http.MethodGet,
			path:   "/users?limit=101&role=owner",
			auth:   "Bearer token",
			assertions: func(t *testing.T, rr *statusTrackingRecorder) {
				t.Helper()

				assertErrorResponse(t, rr.ResponseRecorder, http.StatusUnprocessableEntity, "invalid_request", "request contains invalid fields")
				payload := decodePayload(t, rr.ResponseRecorder)
				rawError := payload["error"].(map[string]any)
				details := rawError["details"].([]any)
				if len(details) != 2 {
					t.Fatalf("error.details len = %d, want 2", len(details))
				}
			},
		},
		{
			name:   "get user success",
			method: http.MethodGet,
			path:   "/users/u_1",
			auth:   "Bearer token",
			assertions: func(t *testing.T, rr *statusTrackingRecorder) {
				t.Helper()

				if rr.Code != http.StatusOK {
					t.Fatalf("status = %d, want %d", rr.Code, http.StatusOK)
				}

				payload := decodePayload(t, rr.ResponseRecorder)
				data, ok := payload["data"].(map[string]any)
				if !ok {
					t.Fatalf("data envelope missing or wrong type: %#v", payload["data"])
				}
				if got := data["id"]; got != "u_1" {
					t.Fatalf("data.id = %#v, want u_1", got)
				}
			},
		},
		{
			name:   "mapped domain error via chain mapper",
			method: http.MethodGet,
			path:   "/users/missing",
			auth:   "Bearer token",
			assertions: func(t *testing.T, rr *statusTrackingRecorder) {
				t.Helper()
				assertErrorResponse(t, rr.ResponseRecorder, http.StatusNotFound, "user_not_found", "user not found")
			},
		},
		{
			name:        "create user success",
			method:      http.MethodPost,
			path:        "/users",
			auth:        "Bearer token",
			contentType: "application/json",
			body:        `{"name":"alice","role":"admin"}`,
			assertions: func(t *testing.T, rr *statusTrackingRecorder) {
				t.Helper()

				if rr.Code != http.StatusCreated {
					t.Fatalf("status = %d, want %d", rr.Code, http.StatusCreated)
				}

				payload := decodePayload(t, rr.ResponseRecorder)
				data, ok := payload["data"].(map[string]any)
				if !ok {
					t.Fatalf("data envelope missing or wrong type: %#v", payload["data"])
				}
				if got := data["name"]; got != "alice" {
					t.Fatalf("data.name = %#v, want alice", got)
				}
				if got := data["role"]; got != "admin" {
					t.Fatalf("data.role = %#v, want admin", got)
				}
			},
		},
		{
			name:        "create user mapped conflict via WithErrorMappers",
			method:      http.MethodPost,
			path:        "/users",
			auth:        "Bearer token",
			contentType: "application/json",
			body:        `{"name":"taken"}`,
			assertions: func(t *testing.T, rr *statusTrackingRecorder) {
				t.Helper()
				assertErrorResponse(t, rr.ResponseRecorder, http.StatusConflict, "user_conflict", "user already exists")
			},
		},
		{
			name:        "create user defaults role",
			method:      http.MethodPost,
			path:        "/users",
			auth:        "Bearer token",
			contentType: "application/json",
			body:        `{"name":"bob"}`,
			assertions: func(t *testing.T, rr *statusTrackingRecorder) {
				t.Helper()

				if rr.Code != http.StatusCreated {
					t.Fatalf("status = %d, want %d", rr.Code, http.StatusCreated)
				}

				payload := decodePayload(t, rr.ResponseRecorder)
				data, ok := payload["data"].(map[string]any)
				if !ok {
					t.Fatalf("data envelope missing or wrong type: %#v", payload["data"])
				}
				if got := data["role"]; got != "member" {
					t.Fatalf("data.role = %#v, want member", got)
				}
			},
		},
		{
			name:        "create user invalid role",
			method:      http.MethodPost,
			path:        "/users",
			auth:        "Bearer token",
			contentType: "application/json",
			body:        `{"name":"alice","role":"owner"}`,
			assertions: func(t *testing.T, rr *statusTrackingRecorder) {
				t.Helper()
				assertErrorResponse(t, rr.ResponseRecorder, http.StatusUnprocessableEntity, "invalid_request", "request contains invalid fields")
				assertFirstDetail(t, rr.ResponseRecorder, "role", "one_of", "must be member or admin")
			},
		},
		{
			name:        "create user body too large",
			method:      http.MethodPost,
			path:        "/users",
			auth:        "Bearer token",
			contentType: "application/json",
			body:        `{"name":"` + strings.Repeat("x", 160) + `"}`,
			assertions: func(t *testing.T, rr *statusTrackingRecorder) {
				t.Helper()
				assertErrorResponse(t, rr.ResponseRecorder, http.StatusRequestEntityTooLarge, "request_too_large", "request body is too large")
			},
		},
		{
			name:        "create user invalid json",
			method:      http.MethodPost,
			path:        "/users",
			auth:        "Bearer token",
			contentType: "application/json",
			body:        `{"name":`,
			assertions: func(t *testing.T, rr *statusTrackingRecorder) {
				t.Helper()
				assertErrorResponse(t, rr.ResponseRecorder, http.StatusBadRequest, "invalid_json", "request body must be valid JSON")
			},
		},
		{
			name:        "create user validation error",
			method:      http.MethodPost,
			path:        "/users",
			auth:        "Bearer token",
			contentType: "application/json",
			body:        `{"name":"   "}`,
			assertions: func(t *testing.T, rr *statusTrackingRecorder) {
				t.Helper()
				assertErrorResponse(t, rr.ResponseRecorder, http.StatusUnprocessableEntity, "invalid_request", "request contains invalid fields")
				assertFirstDetail(t, rr.ResponseRecorder, "name", "required", "is required")
			},
		},
		{
			name:        "patch profile success with unknown fields allowed",
			method:      http.MethodPatch,
			path:        "/users/u_1/profile",
			auth:        "Bearer token",
			contentType: "application/json",
			body:        `{"display_name":"Alice","timezone":"Asia/Shanghai","ignored":"ok"}`,
			assertions: func(t *testing.T, rr *statusTrackingRecorder) {
				t.Helper()

				if rr.Code != http.StatusOK {
					t.Fatalf("status = %d, want %d", rr.Code, http.StatusOK)
				}

				payload := decodePayload(t, rr.ResponseRecorder)
				data, ok := payload["data"].(map[string]any)
				if !ok {
					t.Fatalf("data envelope missing or wrong type: %#v", payload["data"])
				}
				if got := data["display_name"]; got != "Alice" {
					t.Fatalf("data.display_name = %#v, want Alice", got)
				}
			},
		},
		{
			name:        "patch profile validation error",
			method:      http.MethodPatch,
			path:        "/users/u_1/profile",
			auth:        "Bearer token",
			contentType: "application/json",
			body:        `{"display_name":"ab"}`,
			assertions: func(t *testing.T, rr *statusTrackingRecorder) {
				t.Helper()
				assertErrorResponse(t, rr.ResponseRecorder, http.StatusUnprocessableEntity, "invalid_request", "request contains invalid fields")
				assertFirstDetail(t, rr.ResponseRecorder, "display_name", "min_length", "must be at least 3 characters")
			},
		},
		{
			name:        "patch profile requires at least one field",
			method:      http.MethodPatch,
			path:        "/users/u_1/profile",
			auth:        "Bearer token",
			contentType: "application/json",
			body:        `{}`,
			assertions: func(t *testing.T, rr *statusTrackingRecorder) {
				t.Helper()
				assertErrorResponse(t, rr.ResponseRecorder, http.StatusUnprocessableEntity, "invalid_request", "request contains invalid fields")
				assertFirstDetail(t, rr.ResponseRecorder, "profile", "required", "display_name or timezone is required")
			},
		},
		{
			name:        "patch profile decode error",
			method:      http.MethodPatch,
			path:        "/users/u_1/profile",
			auth:        "Bearer token",
			contentType: "application/json",
			body:        `{"display_name":`,
			assertions: func(t *testing.T, rr *statusTrackingRecorder) {
				t.Helper()
				assertErrorResponse(t, rr.ResponseRecorder, http.StatusBadRequest, "invalid_json", "request body must be valid JSON")
			},
		},
		{
			name:        "refresh session allows empty body",
			method:      http.MethodPost,
			path:        "/sessions/refresh",
			auth:        "Bearer token",
			contentType: "application/json",
			assertions: func(t *testing.T, rr *statusTrackingRecorder) {
				t.Helper()

				if rr.Code != http.StatusOK {
					t.Fatalf("status = %d, want %d", rr.Code, http.StatusOK)
				}

				payload := decodePayload(t, rr.ResponseRecorder)
				data, ok := payload["data"].(map[string]any)
				if !ok {
					t.Fatalf("data envelope missing or wrong type: %#v", payload["data"])
				}
				if got := data["refresh_token"]; got != "cookie_refresh_token" {
					t.Fatalf("data.refresh_token = %#v, want cookie_refresh_token", got)
				}
			},
		},
		{
			name:        "refresh session decode error",
			method:      http.MethodPost,
			path:        "/sessions/refresh",
			auth:        "Bearer token",
			contentType: "application/json",
			body:        `{"refresh_token":`,
			assertions: func(t *testing.T, rr *statusTrackingRecorder) {
				t.Helper()
				assertErrorResponse(t, rr.ResponseRecorder, http.StatusBadRequest, "invalid_json", "request body must be valid JSON")
			},
		},
		{
			name:   "export report success with unknown query fields allowed",
			method: http.MethodGet,
			path:   "/reports/export?format=csv&trace_id=req_123",
			auth:   "Bearer token",
			assertions: func(t *testing.T, rr *statusTrackingRecorder) {
				t.Helper()

				if rr.Code != http.StatusOK {
					t.Fatalf("status = %d, want %d", rr.Code, http.StatusOK)
				}

				payload := decodePayload(t, rr.ResponseRecorder)
				data, ok := payload["data"].(map[string]any)
				if !ok {
					t.Fatalf("data envelope missing or wrong type: %#v", payload["data"])
				}
				if got := data["download_url"]; got != "/downloads/report.csv" {
					t.Fatalf("data.download_url = %#v, want /downloads/report.csv", got)
				}
			},
		},
		{
			name:   "export report defaults format",
			method: http.MethodGet,
			path:   "/reports/export",
			auth:   "Bearer token",
			assertions: func(t *testing.T, rr *statusTrackingRecorder) {
				t.Helper()

				if rr.Code != http.StatusOK {
					t.Fatalf("status = %d, want %d", rr.Code, http.StatusOK)
				}

				payload := decodePayload(t, rr.ResponseRecorder)
				data, ok := payload["data"].(map[string]any)
				if !ok {
					t.Fatalf("data envelope missing or wrong type: %#v", payload["data"])
				}
				if got := data["download_url"]; got != "/downloads/report.json" {
					t.Fatalf("data.download_url = %#v, want /downloads/report.json", got)
				}
			},
		},
		{
			name:   "export report request error",
			method: http.MethodGet,
			path:   "/reports/export?format=xml",
			auth:   "Bearer token",
			assertions: func(t *testing.T, rr *statusTrackingRecorder) {
				t.Helper()
				assertErrorResponse(t, rr.ResponseRecorder, http.StatusNotAcceptable, "unsupported_format", "format must be json or csv")
			},
		},
		{
			name:   "accept invite success writes empty body",
			method: http.MethodPost,
			path:   "/invites/live/accept",
			auth:   "Bearer token",
			assertions: func(t *testing.T, rr *statusTrackingRecorder) {
				t.Helper()

				if rr.Code != http.StatusNoContent {
					t.Fatalf("status = %d, want %d", rr.Code, http.StatusNoContent)
				}
				if got := rr.Body.String(); got != "" {
					t.Fatalf("body = %q, want empty", got)
				}
			},
		},
		{
			name:   "accept invite mapped domain error",
			method: http.MethodPost,
			path:   "/invites/expired/accept",
			auth:   "Bearer token",
			assertions: func(t *testing.T, rr *statusTrackingRecorder) {
				t.Helper()
				assertErrorResponse(t, rr.ResponseRecorder, http.StatusGone, "invite_expired", "invite has expired")
			},
		},
		{
			name:   "direct domain error",
			method: http.MethodPost,
			path:   "/users/u_suspended/suspend",
			auth:   "Bearer token",
			assertions: func(t *testing.T, rr *statusTrackingRecorder) {
				t.Helper()
				assertErrorResponse(t, rr.ResponseRecorder, http.StatusConflict, "user_already_suspended", "user already suspended")
			},
		},
		{
			name:   "suspend user success writes empty body",
			method: http.MethodPost,
			path:   "/users/u_1/suspend",
			auth:   "Bearer token",
			assertions: func(t *testing.T, rr *statusTrackingRecorder) {
				t.Helper()

				if rr.Code != http.StatusNoContent {
					t.Fatalf("status = %d, want %d", rr.Code, http.StatusNoContent)
				}
				if got := rr.Body.String(); got != "" {
					t.Fatalf("body = %q, want empty", got)
				}
			},
		},
		{
			name:   "direct internal error",
			method: http.MethodGet,
			path:   "/internal/upstream",
			auth:   "Bearer token",
			assertions: func(t *testing.T, rr *statusTrackingRecorder) {
				t.Helper()
				assertErrorResponse(t, rr.ResponseRecorder, http.StatusBadGateway, "upstream_unavailable", "upstream unavailable")
			},
		},
		{
			name:   "unmapped error falls back to internal error",
			method: http.MethodGet,
			path:   "/internal/unmapped",
			auth:   "Bearer token",
			assertions: func(t *testing.T, rr *statusTrackingRecorder) {
				t.Helper()
				assertErrorResponse(t, rr.ResponseRecorder, http.StatusInternalServerError, "internal_error", "internal server error")
			},
		},
		{
			name:   "route not found",
			method: http.MethodGet,
			path:   "/missing",
			auth:   "Bearer token",
			assertions: func(t *testing.T, rr *statusTrackingRecorder) {
				t.Helper()
				assertErrorResponse(t, rr.ResponseRecorder, http.StatusNotFound, "route_not_found", "route not found")
			},
		},
		{
			name:   "method not allowed",
			method: http.MethodDelete,
			path:   "/users/u_1",
			auth:   "Bearer token",
			assertions: func(t *testing.T, rr *statusTrackingRecorder) {
				t.Helper()
				assertErrorResponse(t, rr.ResponseRecorder, http.StatusMethodNotAllowed, "method_not_allowed", "method not allowed")
			},
		},
		{
			name:   "auth middleware request error",
			method: http.MethodGet,
			path:   "/users/u_1",
			assertions: func(t *testing.T, rr *statusTrackingRecorder) {
				t.Helper()
				assertErrorResponse(t, rr.ResponseRecorder, http.StatusUnauthorized, "unauthorized", "missing authorization token")
			},
		},
		{
			name:   "started response is not rewritten",
			method: http.MethodGet,
			path:   "/partial",
			auth:   "Bearer token",
			assertions: func(t *testing.T, rr *statusTrackingRecorder) {
				t.Helper()

				if rr.Code != http.StatusOK {
					t.Fatalf("status = %d, want %d", rr.Code, http.StatusOK)
				}
				if got := rr.Body.String(); got != "partial" {
					t.Fatalf("body = %q, want partial", got)
				}
			},
		},
		{
			name:   "recoverer handles panic",
			method: http.MethodGet,
			path:   "/panic",
			auth:   "Bearer token",
			assertions: func(t *testing.T, rr *statusTrackingRecorder) {
				t.Helper()
				assertErrorResponse(t, rr.ResponseRecorder, http.StatusInternalServerError, "internal_error", "internal server error")
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(tt.method, tt.path, strings.NewReader(tt.body))
			if tt.auth != "" {
				req.Header.Set("Authorization", tt.auth)
			}
			if tt.contentType != "" {
				req.Header.Set("Content-Type", tt.contentType)
			}

			rr := &statusTrackingRecorder{ResponseRecorder: httptest.NewRecorder()}
			router.ServeHTTP(rr, req)
			tt.assertions(t, rr)
		})
	}
}

func TestChiPartialPanicsWhenWriteFails(t *testing.T) {
	router := newRouter()
	req := httptest.NewRequest(http.MethodGet, "/partial", nil)
	req.Header.Set("Authorization", "Bearer token")

	w := &failingResponseWriter{}
	router.ServeHTTP(w, req)
}

func decodePayload(t *testing.T, rr *httptest.ResponseRecorder) map[string]any {
	t.Helper()

	var payload map[string]any
	if err := json.Unmarshal(rr.Body.Bytes(), &payload); err != nil {
		t.Fatalf("unmarshal response: %v", err)
	}

	return payload
}

func assertErrorResponse(t *testing.T, rr *httptest.ResponseRecorder, status int, code, message string) {
	t.Helper()

	if rr.Code != status {
		t.Fatalf("status = %d, want %d", rr.Code, status)
	}
	if got := rr.Header().Get("Content-Type"); got != "application/json" {
		t.Fatalf("Content-Type = %q, want application/json", got)
	}

	payload := decodePayload(t, rr)

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
	if _, ok := rawError["details"].([]any); !ok {
		t.Fatalf("error.details = %#v, want array", rawError["details"])
	}
}

func assertFirstDetail(t *testing.T, rr *httptest.ResponseRecorder, field, code, message string) {
	t.Helper()

	payload := decodePayload(t, rr)
	rawError := payload["error"].(map[string]any)
	details := rawError["details"].([]any)
	if len(details) == 0 {
		t.Fatalf("error.details = %#v, want non-empty array", details)
	}

	first, ok := details[0].(map[string]any)
	if !ok {
		t.Fatalf("first detail = %#v, want object", details[0])
	}
	if got := first["field"]; got != field {
		t.Fatalf("detail.field = %#v, want %q", got, field)
	}
	if got := first["code"]; got != code {
		t.Fatalf("detail.code = %#v, want %q", got, code)
	}
	if got := first["message"]; got != message {
		t.Fatalf("detail.message = %#v, want %q", got, message)
	}
}
