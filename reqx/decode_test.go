package reqx_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/kanata996/chix"
	"github.com/kanata996/chix/reqx"
)

type createUserRequest struct {
	Name string `json:"name"`
	Age  int    `json:"age"`
}

func TestDecodeJSONSuccess(t *testing.T) {
	h := chix.Wrap(func(w http.ResponseWriter, r *http.Request) error {
		var req createUserRequest
		if err := reqx.DecodeJSON(r, &req); err != nil {
			return err
		}
		return chix.Write(w, http.StatusOK, map[string]any{
			"name": req.Name,
			"age":  req.Age,
		})
	})

	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/users", strings.NewReader(`{"name":"alice","age":20}`))
	req.Header.Set("Content-Type", "application/json")
	h.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rr.Code, http.StatusOK)
	}

	var payload map[string]any
	if err := json.Unmarshal(rr.Body.Bytes(), &payload); err != nil {
		t.Fatalf("unmarshal response: %v", err)
	}

	data, ok := payload["data"].(map[string]any)
	if !ok {
		t.Fatalf("data = %#v, want object", payload["data"])
	}
	if got := data["name"]; got != "alice" {
		t.Fatalf("data.name = %#v, want alice", got)
	}
	if got := data["age"]; got != float64(20) {
		t.Fatalf("data.age = %#v, want 20", got)
	}
}

func TestDecodeJSONAcceptsPlusJSONContentType(t *testing.T) {
	h := chix.Wrap(func(w http.ResponseWriter, r *http.Request) error {
		var req createUserRequest
		if err := reqx.DecodeJSON(r, &req); err != nil {
			return err
		}
		return chix.Write(w, http.StatusOK, map[string]any{"name": req.Name})
	})

	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/users", strings.NewReader(`{"name":"alice"}`))
	req.Header.Set("Content-Type", "application/merge-patch+json")
	h.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rr.Code, http.StatusOK)
	}
}

func TestDecodeJSONAcceptsMissingContentType(t *testing.T) {
	h := chix.Wrap(func(w http.ResponseWriter, r *http.Request) error {
		var req createUserRequest
		if err := reqx.DecodeJSON(r, &req); err != nil {
			return err
		}
		return chix.Write(w, http.StatusOK, map[string]any{"name": req.Name})
	})

	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/users", strings.NewReader(`{"name":"alice"}`))
	h.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rr.Code, http.StatusOK)
	}
}

func TestDecodeAndValidateJSONReturnsDecodeError(t *testing.T) {
	h := chix.Wrap(func(w http.ResponseWriter, r *http.Request) error {
		var req createUserRequest
		return reqx.DecodeAndValidateJSON(r, &req, func(value *createUserRequest) []reqx.Violation {
			return nil
		})
	})

	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/users", strings.NewReader(`{"name":`))
	req.Header.Set("Content-Type", "application/json")
	h.ServeHTTP(rr, req)

	assertErrorResponse(t, rr, http.StatusBadRequest, "invalid_json", "request body must be valid JSON")
}

func TestDecodeJSONRejectsUnsupportedContentType(t *testing.T) {
	h := chix.Wrap(func(w http.ResponseWriter, r *http.Request) error {
		var req createUserRequest
		return reqx.DecodeJSON(r, &req)
	})

	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/users", strings.NewReader(`{"name":"alice"}`))
	req.Header.Set("Content-Type", "text/plain")
	h.ServeHTTP(rr, req)

	assertErrorResponse(t, rr, http.StatusUnsupportedMediaType, "unsupported_media_type", "Content-Type must be application/json")
}

func TestDecodeJSONRejectsEmptyBody(t *testing.T) {
	h := chix.Wrap(func(w http.ResponseWriter, r *http.Request) error {
		var req createUserRequest
		return reqx.DecodeJSON(r, &req)
	})

	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/users", nil)
	req.Header.Set("Content-Type", "application/json")
	h.ServeHTTP(rr, req)

	assertErrorResponse(t, rr, http.StatusBadRequest, "invalid_json", "request body must not be empty")
}

func TestDecodeJSONRejectsMalformedJSON(t *testing.T) {
	h := chix.Wrap(func(w http.ResponseWriter, r *http.Request) error {
		var req createUserRequest
		return reqx.DecodeJSON(r, &req)
	})

	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/users", strings.NewReader(`{"name":`))
	req.Header.Set("Content-Type", "application/json")
	h.ServeHTTP(rr, req)

	assertErrorResponse(t, rr, http.StatusBadRequest, "invalid_json", "request body must be valid JSON")
}

func TestDecodeJSONRejectsTrailingJSON(t *testing.T) {
	h := chix.Wrap(func(w http.ResponseWriter, r *http.Request) error {
		var req createUserRequest
		return reqx.DecodeJSON(r, &req)
	})

	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/users", strings.NewReader(`{"name":"alice"} {"name":"bob"}`))
	req.Header.Set("Content-Type", "application/json")
	h.ServeHTTP(rr, req)

	assertErrorResponse(t, rr, http.StatusBadRequest, "invalid_json", "request body must contain a single JSON value")
}

func TestDecodeJSONRejectsUnknownFields(t *testing.T) {
	h := chix.Wrap(func(w http.ResponseWriter, r *http.Request) error {
		var req createUserRequest
		return reqx.DecodeJSON(r, &req)
	})

	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/users", strings.NewReader(`{"name":"alice","extra":true}`))
	req.Header.Set("Content-Type", "application/json")
	h.ServeHTTP(rr, req)

	assertErrorResponse(t, rr, http.StatusUnprocessableEntity, "invalid_request", "request contains invalid fields")
	assertFirstDetail(t, rr, "extra", "unknown", "unknown field")
}

func TestDecodeJSONRejectsFieldTypeMismatch(t *testing.T) {
	h := chix.Wrap(func(w http.ResponseWriter, r *http.Request) error {
		var req createUserRequest
		return reqx.DecodeJSON(r, &req)
	})

	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/users", strings.NewReader(`{"name":123}`))
	req.Header.Set("Content-Type", "application/json")
	h.ServeHTTP(rr, req)

	assertErrorResponse(t, rr, http.StatusUnprocessableEntity, "invalid_request", "request contains invalid fields")
	assertFirstDetail(t, rr, "name", "type", "must be string")
}

func TestDecodeJSONRejectsLargeBodies(t *testing.T) {
	h := chix.Wrap(func(w http.ResponseWriter, r *http.Request) error {
		var req createUserRequest
		return reqx.DecodeJSON(r, &req, reqx.WithMaxBodyBytes(8))
	})

	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/users", strings.NewReader(`{"name":"alice"}`))
	req.Header.Set("Content-Type", "application/json")
	h.ServeHTTP(rr, req)

	assertErrorResponse(t, rr, http.StatusRequestEntityTooLarge, "request_too_large", "request body is too large")
}

func TestDecodeJSONRejectsNilRequest(t *testing.T) {
	var req createUserRequest

	err := reqx.DecodeJSON(nil, &req)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if got := err.Error(); got != "chix/reqx: request must not be nil" {
		t.Fatalf("error = %q, want request must not be nil", got)
	}
}

func TestDecodeJSONRejectsNilDestination(t *testing.T) {
	req := httptest.NewRequest(http.MethodPost, "/users", strings.NewReader(`{"name":"alice"}`))

	err := reqx.DecodeJSON[createUserRequest](req, nil)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if got := err.Error(); got != "chix/reqx: destination must not be nil" {
		t.Fatalf("error = %q, want destination must not be nil", got)
	}
}

func TestDecodeJSONReturnsReadError(t *testing.T) {
	req := httptest.NewRequest(http.MethodPost, "/users", errReaderCloser{err: simpleError("read failed")})
	req.Header.Set("Content-Type", "application/json")

	var dst createUserRequest
	err := reqx.DecodeJSON(req, &dst)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if got := err.Error(); got != "read failed" {
		t.Fatalf("error = %q, want read failed", got)
	}
}

func TestDecodeJSONCanAllowUnknownFields(t *testing.T) {
	h := chix.Wrap(func(w http.ResponseWriter, r *http.Request) error {
		var req createUserRequest
		if err := reqx.DecodeJSON(r, &req, reqx.AllowUnknownFields()); err != nil {
			return err
		}
		return chix.Write(w, http.StatusOK, map[string]any{"name": req.Name})
	})

	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/users", strings.NewReader(`{"name":"alice","extra":true}`))
	req.Header.Set("Content-Type", "application/json")
	h.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rr.Code, http.StatusOK)
	}
}

func TestDecodeJSONCanAllowEmptyBody(t *testing.T) {
	h := chix.Wrap(func(w http.ResponseWriter, r *http.Request) error {
		var req createUserRequest
		if err := reqx.DecodeJSON(r, &req, reqx.AllowEmptyBody()); err != nil {
			return err
		}
		return chix.Write(w, http.StatusOK, map[string]any{"name": req.Name})
	})

	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/users", nil)
	req.Header.Set("Content-Type", "application/json")
	h.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rr.Code, http.StatusOK)
	}
}

func TestDecodeAndValidateJSONRejectsViolations(t *testing.T) {
	h := chix.Wrap(func(w http.ResponseWriter, r *http.Request) error {
		var req createUserRequest
		return reqx.DecodeAndValidateJSON(r, &req, func(value *createUserRequest) []reqx.Violation {
			if strings.TrimSpace(value.Name) == "" {
				return []reqx.Violation{
					{Field: "name", Code: "required", Message: "is required"},
				}
			}
			return nil
		})
	})

	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/users", strings.NewReader(`{"name":"   "}`))
	req.Header.Set("Content-Type", "application/json")
	h.ServeHTTP(rr, req)

	assertErrorResponse(t, rr, http.StatusUnprocessableEntity, "invalid_request", "request contains invalid fields")
	assertFirstDetail(t, rr, "name", "required", "is required")
}

func TestDecodeAndValidateJSONSuccess(t *testing.T) {
	h := chix.Wrap(func(w http.ResponseWriter, r *http.Request) error {
		var req createUserRequest
		if err := reqx.DecodeAndValidateJSON(r, &req, func(value *createUserRequest) []reqx.Violation {
			if strings.TrimSpace(value.Name) == "" {
				return []reqx.Violation{{Field: "name", Code: "required", Message: "is required"}}
			}
			return nil
		}); err != nil {
			return err
		}

		return chix.Write(w, http.StatusCreated, map[string]any{"name": strings.TrimSpace(req.Name)})
	})

	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/users", strings.NewReader(`{"name":" alice "}`))
	req.Header.Set("Content-Type", "application/json")
	h.ServeHTTP(rr, req)

	if rr.Code != http.StatusCreated {
		t.Fatalf("status = %d, want %d", rr.Code, http.StatusCreated)
	}

	payload := decodePayload(t, rr)
	data, ok := payload["data"].(map[string]any)
	if !ok {
		t.Fatalf("data = %#v, want object", payload["data"])
	}
	if got := data["name"]; got != "alice" {
		t.Fatalf("data.name = %#v, want alice", got)
	}
}

func decodePayload(t *testing.T, rr *httptest.ResponseRecorder) map[string]any {
	t.Helper()

	var payload map[string]any
	if err := json.Unmarshal(rr.Body.Bytes(), &payload); err != nil {
		t.Fatalf("unmarshal response: %v", err)
	}

	return payload
}

func assertErrorResponse(t *testing.T, rr *httptest.ResponseRecorder, status int, code, message string) map[string]any {
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

	return payload
}

func assertFirstDetail(t *testing.T, rr *httptest.ResponseRecorder, field, code, message string) {
	t.Helper()

	payload := assertErrorResponse(t, rr, rr.Code, codeForRecorder(t, rr), messageForRecorder(t, rr))

	rawError := payload["error"].(map[string]any)
	details, ok := rawError["details"].([]any)
	if !ok || len(details) == 0 {
		t.Fatalf("error.details = %#v, want non-empty array", rawError["details"])
	}

	detail, ok := details[0].(map[string]any)
	if !ok {
		t.Fatalf("details[0] = %#v, want object", details[0])
	}
	if got := detail["field"]; got != field {
		t.Fatalf("detail.field = %#v, want %q", got, field)
	}
	if got := detail["code"]; got != code {
		t.Fatalf("detail.code = %#v, want %q", got, code)
	}
	if got := detail["message"]; got != message {
		t.Fatalf("detail.message = %#v, want %q", got, message)
	}
}

func codeForRecorder(t *testing.T, rr *httptest.ResponseRecorder) string {
	t.Helper()

	var payload map[string]any
	if err := json.Unmarshal(rr.Body.Bytes(), &payload); err != nil {
		t.Fatalf("unmarshal response: %v", err)
	}

	rawError, ok := payload["error"].(map[string]any)
	if !ok {
		t.Fatalf("error envelope missing or wrong type: %#v", payload["error"])
	}

	code, _ := rawError["code"].(string)
	return code
}

func messageForRecorder(t *testing.T, rr *httptest.ResponseRecorder) string {
	t.Helper()

	var payload map[string]any
	if err := json.Unmarshal(rr.Body.Bytes(), &payload); err != nil {
		t.Fatalf("unmarshal response: %v", err)
	}

	rawError, ok := payload["error"].(map[string]any)
	if !ok {
		t.Fatalf("error envelope missing or wrong type: %#v", payload["error"])
	}

	message, _ := rawError["message"].(string)
	return message
}

type errReaderCloser struct {
	err error
}

func (r errReaderCloser) Read(_ []byte) (int, error) {
	return 0, r.err
}

func (r errReaderCloser) Close() error {
	return nil
}

type simpleError string

func (e simpleError) Error() string {
	return string(e)
}
