package resp

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
)

type payloadMap map[string]any

func TestCreatedWritesDirectPayload(t *testing.T) {
	req := httptest.NewRequest(http.MethodPost, "/v1/accounts", nil)
	rr := httptest.NewRecorder()

	err := Created(rr, req, map[string]any{"id": "u_1"})
	if err != nil {
		t.Fatalf("Created() error = %v", err)
	}

	if rr.Code != http.StatusCreated {
		t.Fatalf("status = %d, want %d", rr.Code, http.StatusCreated)
	}
	if got := rr.Header().Get("Content-Type"); got != "application/json" {
		t.Fatalf("Content-Type = %q, want application/json", got)
	}

	payload := decodePayload(t, rr.Body.Bytes())
	if got := payload["id"]; got != "u_1" {
		t.Fatalf("id = %#v, want u_1", got)
	}
}

func TestJSONWritesDirectPayload(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rr := httptest.NewRecorder()

	err := JSON(rr, req, http.StatusAccepted, map[string]any{"id": "u_1"})
	if err != nil {
		t.Fatalf("JSON() error = %v", err)
	}

	if rr.Code != http.StatusAccepted {
		t.Fatalf("status = %d, want %d", rr.Code, http.StatusAccepted)
	}
	if got := rr.Header().Get("Content-Type"); got != "application/json" {
		t.Fatalf("Content-Type = %q, want application/json", got)
	}

	payload := decodePayload(t, rr.Body.Bytes())
	if got := payload["id"]; got != "u_1" {
		t.Fatalf("id = %#v, want u_1", got)
	}
}

func TestJSONAllowsNilData(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rr := httptest.NewRecorder()

	if err := JSON(rr, req, http.StatusOK, nil); err != nil {
		t.Fatalf("JSON() error = %v", err)
	}
	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rr.Code, http.StatusOK)
	}
	if body := rr.Body.String(); body != "null\n" {
		t.Fatalf("body = %q, want %q", body, "null\n")
	}
}

func TestJSONUsesPrettyQueryParam(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/?pretty", nil)
	rr := httptest.NewRecorder()

	err := JSON(rr, req, http.StatusOK, map[string]any{"id": "u_1"})
	if err != nil {
		t.Fatalf("JSON() error = %v", err)
	}

	if body := rr.Body.String(); body != "{\n  \"id\": \"u_1\"\n}\n" {
		t.Fatalf("body = %q, want pretty JSON", body)
	}
}

func TestJSONPrettyWritesIndentedJSON(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rr := httptest.NewRecorder()

	err := JSONPretty(rr, req, http.StatusOK, map[string]any{"id": "u_1"}, "    ")
	if err != nil {
		t.Fatalf("JSONPretty() error = %v", err)
	}

	if body := rr.Body.String(); body != "{\n    \"id\": \"u_1\"\n}\n" {
		t.Fatalf("body = %q, want indented JSON", body)
	}
}

func TestJSONBlobWritesRawJSONBytes(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rr := httptest.NewRecorder()

	err := JSONBlob(rr, req, http.StatusAccepted, []byte(`{"id":"u_1"}`))
	if err != nil {
		t.Fatalf("JSONBlob() error = %v", err)
	}

	if rr.Code != http.StatusAccepted {
		t.Fatalf("status = %d, want %d", rr.Code, http.StatusAccepted)
	}
	if got := rr.Header().Get("Content-Type"); got != "application/json" {
		t.Fatalf("Content-Type = %q, want application/json", got)
	}
	if body := rr.Body.String(); body != `{"id":"u_1"}` {
		t.Fatalf("body = %q, want raw JSON bytes", body)
	}
}

func TestJSONRejectsUnsupportedValue(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rr := httptest.NewRecorder()

	err := JSON(rr, req, http.StatusOK, make(chan int))
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}

func TestJSONRejectsInvalidStatus(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rr := httptest.NewRecorder()

	err := JSON(rr, req, 1000, map[string]any{"id": "u_1"})
	if err == nil || err.Error() != "resp: invalid HTTP status 1000" {
		t.Fatalf("JSON() error = %v, want invalid HTTP status", err)
	}
}

func TestOKRejectsNilData(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rr := httptest.NewRecorder()

	if err := OK(rr, req, nil); err == nil {
		t.Fatal("expected error, got nil")
	}
}

func TestOKRejectsUnsupportedValue(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rr := httptest.NewRecorder()

	err := OK(rr, req, make(chan int))
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}

func TestOKRejectsNilWriter(t *testing.T) {
	err := OK(nil, httptest.NewRequest(http.MethodGet, "/", nil), map[string]any{"id": "u_1"})
	if err == nil || err.Error() != "resp: response writer is nil" {
		t.Fatalf("OK() error = %v, want response writer is nil", err)
	}
}

func TestOKUsesPrettyQueryParam(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/?pretty", nil)
	rr := httptest.NewRecorder()

	if err := OK(rr, req, map[string]any{"id": "u_1"}); err != nil {
		t.Fatalf("OK() error = %v", err)
	}
	if body := rr.Body.String(); body != "{\n  \"id\": \"u_1\"\n}\n" {
		t.Fatalf("body = %q, want pretty JSON", body)
	}
}

func TestNoContentWritesBodylessStatus(t *testing.T) {
	req := httptest.NewRequest(http.MethodDelete, "/", nil)
	rr := httptest.NewRecorder()

	if err := NoContent(rr, req); err != nil {
		t.Fatalf("NoContent() error = %v", err)
	}
	if rr.Code != http.StatusNoContent {
		t.Fatalf("status = %d, want %d", rr.Code, http.StatusNoContent)
	}
	if rr.Body.Len() != 0 {
		t.Fatalf("body = %q, want empty", rr.Body.String())
	}
}

func TestValidateSuccessBodyStatus(t *testing.T) {
	testCases := []struct {
		name   string
		status int
		want   string
	}{
		{name: "ok", status: http.StatusOK},
		{name: "informational", status: http.StatusContinue, want: "resp: success writers with a body cannot use informational status 100"},
		{name: "bodyless", status: http.StatusNoContent, want: "resp: success writers with a body cannot use bodyless status 204"},
		{name: "invalid success status", status: http.StatusBadRequest, want: "resp: invalid success status 400"},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			err := validateSuccessBodyStatus(tc.status)
			if tc.want == "" && err != nil {
				t.Fatalf("validateSuccessBodyStatus(%d) error = %v", tc.status, err)
			}
			if tc.want != "" {
				if err == nil || err.Error() != tc.want {
					t.Fatalf("validateSuccessBodyStatus(%d) error = %v, want %q", tc.status, err, tc.want)
				}
			}
		})
	}
}

func TestValidateSuccessStatus(t *testing.T) {
	testCases := []struct {
		name   string
		status int
		want   string
	}{
		{name: "ok", status: http.StatusCreated},
		{name: "error status", status: http.StatusBadRequest, want: "resp: invalid success status 400"},
		{name: "invalid low status", status: 99, want: "resp: invalid HTTP status 99"},
		{name: "invalid high status", status: 1000, want: "resp: invalid HTTP status 1000"},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			err := validateSuccessStatus(tc.status)
			if tc.want == "" && err != nil {
				t.Fatalf("validateSuccessStatus(%d) error = %v", tc.status, err)
			}
			if tc.want != "" {
				if err == nil || err.Error() != tc.want {
					t.Fatalf("validateSuccessStatus(%d) error = %v, want %q", tc.status, err, tc.want)
				}
			}
		})
	}
}

func TestWriteJSONPropagatesEncodeError(t *testing.T) {
	err := writeJSON(httptest.NewRecorder(), http.StatusOK, make(chan int), "")
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}

func TestWriteSuccessRejectsInvalidStatus(t *testing.T) {
	err := writeSuccess(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/", nil), http.StatusBadRequest, map[string]any{"id": "u_1"})
	if err == nil || err.Error() != "resp: invalid success status 400" {
		t.Fatalf("writeSuccess() error = %v, want invalid success status", err)
	}
}

func TestWriteJSONBytesReturnsWrappedWriteError(t *testing.T) {
	w := &failingWriter{}

	err := writeJSONBytes(w, http.StatusOK, []byte(`{"id":"u_1"}`))
	if err == nil {
		t.Fatal("expected error, got nil")
	}

	var writeErr *responseWriteError
	if !errors.As(err, &writeErr) {
		t.Fatalf("error = %T, want *responseWriteError", err)
	}
	if !writeErr.responseStarted {
		t.Fatal("responseStarted = false, want true")
	}
}

func decodePayload(t *testing.T, body []byte) payloadMap {
	t.Helper()

	var payload payloadMap
	if err := json.Unmarshal(body, &payload); err != nil {
		t.Fatalf("json.Unmarshal() error = %v", err)
	}
	return payload
}
