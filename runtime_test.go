package chix

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
)

func TestHandleBindsInputAndWritesSuccessEnvelope(t *testing.T) {
	type createUserInput struct {
		ID      string `path:"id"`
		Verbose bool   `query:"verbose"`
		Name    string `json:"name"`
	}

	type createUserOutput struct {
		ID      string `json:"id"`
		Name    string `json:"name"`
		Verbose bool   `json:"verbose"`
	}

	rt := New()
	router := chi.NewRouter()
	router.Method(http.MethodPost, "/users/{id}", Handle(rt, Operation[createUserInput, createUserOutput]{
		Method: http.MethodPost,
	}, func(_ context.Context, input *createUserInput) (*createUserOutput, error) {
		return &createUserOutput{
			ID:      input.ID,
			Name:    input.Name,
			Verbose: input.Verbose,
		}, nil
	}))

	req := httptest.NewRequest(http.MethodPost, "/users/u_1?verbose=true", strings.NewReader(`{"name":"Ada"}`))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d", rec.Code)
	}

	var envelope struct {
		Data createUserOutput `json:"data"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &envelope); err != nil {
		t.Fatalf("decode response: %v", err)
	}

	if envelope.Data.ID != "u_1" || envelope.Data.Name != "Ada" || !envelope.Data.Verbose {
		t.Fatalf("unexpected envelope: %+v", envelope)
	}
}

func TestHandleDoesNotImplicitlyBindUntaggedBodyFields(t *testing.T) {
	type input struct {
		Name string
	}

	rt := New()
	router := chi.NewRouter()
	router.Method(http.MethodPost, "/users", Handle(rt, Operation[input, struct{}]{
		Method: http.MethodPost,
	}, func(_ context.Context, in *input) (*struct{}, error) {
		t.Fatalf("handler should not run with invalid request: %+v", in)
		return nil, nil
	}))

	req := httptest.NewRequest(http.MethodPost, "/users", strings.NewReader(`{"name":"Ada"}`))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	assertErrorEnvelopeCode(t, rec, http.StatusBadRequest, "bad_request")
}

func TestHandleRejectsUnsupportedMediaType(t *testing.T) {
	type input struct {
		Name string `json:"name"`
	}

	rt := New()
	router := chi.NewRouter()
	router.Method(http.MethodPost, "/users", Handle(rt, Operation[input, struct{}]{
		Method: http.MethodPost,
	}, func(_ context.Context, _ *input) (*struct{}, error) {
		t.Fatal("handler should not run")
		return nil, nil
	}))

	req := httptest.NewRequest(http.MethodPost, "/users", strings.NewReader(`name=Ada`))
	req.Header.Set("Content-Type", "text/plain")
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	assertErrorEnvelopeCode(t, rec, http.StatusUnsupportedMediaType, "unsupported_media_type")
}

func TestHandleRejectsDuplicateScalarQueryValues(t *testing.T) {
	type input struct {
		Limit int `query:"limit"`
	}

	rt := New()
	router := chi.NewRouter()
	router.Method(http.MethodGet, "/users", Handle(rt, Operation[input, struct{}]{}, func(_ context.Context, _ *input) (*struct{}, error) {
		t.Fatal("handler should not run")
		return nil, nil
	}))

	req := httptest.NewRequest(http.MethodGet, "/users?limit=1&limit=2", nil)
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	assertErrorEnvelopeCode(t, rec, http.StatusBadRequest, "bad_request")
}

func TestHandleValidationWrites422(t *testing.T) {
	type input struct {
		Name string `json:"name"`
	}

	rt := New()
	router := chi.NewRouter()
	router.Method(http.MethodPost, "/users", Handle(rt, Operation[input, struct{}]{
		Method: http.MethodPost,
		Validate: func(_ context.Context, in *input) []Violation {
			if in.Name == "" {
				return []Violation{{
					Source:  "body",
					Field:   "name",
					Code:    "required",
					Message: "name is required",
				}}
			}
			return nil
		},
	}, func(_ context.Context, _ *input) (*struct{}, error) {
		t.Fatal("handler should not run")
		return nil, nil
	}))

	req := httptest.NewRequest(http.MethodPost, "/users", strings.NewReader(`{}`))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("expected 422, got %d", rec.Code)
	}

	var envelope struct {
		Error struct {
			Code    string      `json:"code"`
			Message string      `json:"message"`
			Details []Violation `json:"details"`
		} `json:"error"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &envelope); err != nil {
		t.Fatalf("decode response: %v", err)
	}

	if envelope.Error.Code != "invalid_request" || len(envelope.Error.Details) != 1 {
		t.Fatalf("unexpected validation envelope: %+v", envelope)
	}
}

func TestHandleWritesNilDataAsNull(t *testing.T) {
	type output struct {
		ID string `json:"id"`
	}

	rt := New()
	router := chi.NewRouter()
	router.Method(http.MethodGet, "/users", Handle(rt, Operation[struct{}, output]{}, func(_ context.Context, _ *struct{}) (*output, error) {
		return nil, nil
	}))

	req := httptest.NewRequest(http.MethodGet, "/users", nil)
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
	if strings.TrimSpace(rec.Body.String()) != `{"data":null}` {
		t.Fatalf("unexpected body: %q", rec.Body.String())
	}
}

func TestHandleNoContentSkipsEnvelope(t *testing.T) {
	rt := New()
	router := chi.NewRouter()
	router.Method(http.MethodDelete, "/users/{id}", Handle(rt, Operation[struct{}, struct{}]{
		Method:        http.MethodDelete,
		SuccessStatus: http.StatusNoContent,
	}, func(_ context.Context, _ *struct{}) (*struct{}, error) {
		return &struct{}{}, nil
	}))

	req := httptest.NewRequest(http.MethodDelete, "/users/u_1", nil)
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusNoContent {
		t.Fatalf("expected 204, got %d", rec.Code)
	}
	if rec.Body.Len() != 0 {
		t.Fatalf("expected empty body, got %q", rec.Body.String())
	}
}

func TestHandleMapperPrecedenceAndPublicErrorPassthrough(t *testing.T) {
	type input struct{}
	type output struct{}

	baseErr := errors.New("boom")
	opPublic := &HTTPError{Status: http.StatusConflict, Code: "conflict", Message: "conflict"}
	runtimeMapped := &HTTPError{Status: http.StatusForbidden, Code: "runtime", Message: "runtime"}
	scopeMapped := &HTTPError{Status: http.StatusTeapot, Code: "scope", Message: "scope"}
	opMapped := &HTTPError{Status: http.StatusNotFound, Code: "op", Message: "op"}

	rt := New(WithErrorMapper(func(err error) *HTTPError {
		if errors.Is(err, baseErr) {
			return runtimeMapped
		}
		return nil
	}))
	scope := rt.Scope(WithErrorMapper(func(err error) *HTTPError {
		if errors.Is(err, baseErr) {
			return scopeMapped
		}
		return nil
	}))

	router := chi.NewRouter()
	router.Method(http.MethodGet, "/mapped", Handle(scope, Operation[input, output]{
		ErrorMappers: []ErrorMapper{
			func(err error) *HTTPError {
				if errors.Is(err, baseErr) {
					return opMapped
				}
				return nil
			},
		},
	}, func(_ context.Context, _ *input) (*output, error) {
		return nil, baseErr
	}))
	router.Method(http.MethodGet, "/public", Handle(scope, Operation[input, output]{
		ErrorMappers: []ErrorMapper{
			func(error) *HTTPError {
				return opMapped
			},
		},
	}, func(_ context.Context, _ *input) (*output, error) {
		return nil, fmt.Errorf("wrapped: %w", opPublic)
	}))

	recMapped := httptest.NewRecorder()
	router.ServeHTTP(recMapped, httptest.NewRequest(http.MethodGet, "/mapped", nil))
	assertErrorEnvelopeCode(t, recMapped, http.StatusNotFound, "op")

	recPublic := httptest.NewRecorder()
	router.ServeHTTP(recPublic, httptest.NewRequest(http.MethodGet, "/public", nil))
	assertErrorEnvelopeCode(t, recPublic, http.StatusConflict, "conflict")
}

func TestScopeOverridesSingleValueOptions(t *testing.T) {
	rt := New(WithSuccessStatus(http.StatusAccepted))
	scope := rt.Scope(WithSuccessStatus(http.StatusCreated))

	router := chi.NewRouter()
	router.Method(http.MethodGet, "/users", Handle(scope, Operation[struct{}, struct{}]{}, func(_ context.Context, _ *struct{}) (*struct{}, error) {
		return &struct{}{}, nil
	}))

	req := httptest.NewRequest(http.MethodGet, "/users", nil)
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d", rec.Code)
	}
}

func TestObserverReceivesBoundaryEvent(t *testing.T) {
	sentinel := errors.New("boom")
	var got Event

	rt := New(WithObserver(func(event Event) {
		got = event
	}), WithErrorMapper(func(err error) *HTTPError {
		if errors.Is(err, sentinel) {
			return &HTTPError{Status: http.StatusBadGateway, Code: "upstream_failure", Message: "upstream failure"}
		}
		return nil
	}))

	router := chi.NewRouter()
	router.Use(middleware.RequestID)
	router.Method(http.MethodGet, "/users", Handle(rt, Operation[struct{}, struct{}]{}, func(_ context.Context, _ *struct{}) (*struct{}, error) {
		return nil, sentinel
	}))

	req := httptest.NewRequest(http.MethodGet, "/users?verbose=true", nil)
	req.RemoteAddr = "203.0.113.8:1234"
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	if !errors.Is(got.Error, sentinel) {
		t.Fatalf("expected raw error to be preserved: %+v", got)
	}
	if got.Public == nil || got.Public.Code != "upstream_failure" {
		t.Fatalf("expected mapped public error: %+v", got)
	}
	if got.RequestID == "" || got.Method != http.MethodGet || got.Target != "/users?verbose=true" || got.RemoteAddr != "203.0.113.8:1234" {
		t.Fatalf("unexpected observer context: %+v", got)
	}
}

func assertErrorEnvelopeCode(t *testing.T, rec *httptest.ResponseRecorder, status int, code string) {
	t.Helper()

	if rec.Code != status {
		t.Fatalf("expected %d, got %d", status, rec.Code)
	}

	var envelope struct {
		Error struct {
			Code    string `json:"code"`
			Message string `json:"message"`
			Details []any  `json:"details"`
		} `json:"error"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &envelope); err != nil {
		t.Fatalf("decode error response: %v", err)
	}

	if envelope.Error.Code != code {
		t.Fatalf("expected error code %q, got %+v", code, envelope)
	}
	if !strings.Contains(rec.Body.String(), `"details"`) {
		t.Fatalf("expected details field in error envelope, got %q", rec.Body.String())
	}
}
