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

type validationDetail struct {
	Source  string `json:"source"`
	Field   string `json:"field"`
	Code    string `json:"code"`
	Message string `json:"message"`
}

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

func TestHandlePanicsOnInvalidInputSchema(t *testing.T) {
	type profile struct {
		Name string
	}
	type input struct {
		Profile profile `json:"profile"`
	}

	defer func() {
		if recovered := recover(); recovered == nil {
			t.Fatal("expected panic for invalid input schema")
		}
	}()

	_ = Handle(New(), Operation[input, struct{}]{}, func(_ context.Context, _ *input) (*struct{}, error) {
		return &struct{}{}, nil
	})
}

func TestHandleBindsAnonymousEmbeddedInputFields(t *testing.T) {
	type route struct {
		ID string `path:"id"`
	}
	type filters struct {
		Verbose bool `query:"verbose"`
	}
	type payload struct {
		Name string `json:"name"`
	}
	type input struct {
		route
		filters
		payload
	}
	type output struct {
		ID      string `json:"id"`
		Name    string `json:"name"`
		Verbose bool   `json:"verbose"`
	}

	rt := New()
	router := chi.NewRouter()
	router.Method(http.MethodPost, "/users/{id}", Handle(rt, Operation[input, output]{
		Method: http.MethodPost,
	}, func(_ context.Context, in *input) (*output, error) {
		return &output{
			ID:      in.ID,
			Name:    in.Name,
			Verbose: in.Verbose,
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
		Data output `json:"data"`
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

func TestHandleRejectsNestedUnknownBodyFields(t *testing.T) {
	type profile struct {
		Name string `json:"name"`
	}
	type input struct {
		Profile profile `json:"profile"`
	}

	rt := New()
	router := chi.NewRouter()
	router.Method(http.MethodPost, "/users", Handle(rt, Operation[input, struct{}]{
		Method: http.MethodPost,
	}, func(_ context.Context, _ *input) (*struct{}, error) {
		t.Fatal("handler should not run")
		return nil, nil
	}))

	req := httptest.NewRequest(http.MethodPost, "/users", strings.NewReader(`{"profile":{"name":"Ada","extra":"x"}}`))
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
		Name string `json:"name" validate:"required"`
	}

	rt := New()
	router := chi.NewRouter()
	router.Method(http.MethodPost, "/users", Handle(rt, Operation[input, struct{}]{
		Method: http.MethodPost,
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
			Code    string             `json:"code"`
			Message string             `json:"message"`
			Details []validationDetail `json:"details"`
		} `json:"error"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &envelope); err != nil {
		t.Fatalf("decode response: %v", err)
	}

	if envelope.Error.Code != "invalid_request" || len(envelope.Error.Details) != 1 {
		t.Fatalf("unexpected validation envelope: %+v", envelope)
	}
	if detail := envelope.Error.Details[0]; detail.Source != "body" || detail.Field != "name" || detail.Code != "required" {
		t.Fatalf("unexpected validation detail: %+v", detail)
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

func TestHandleMapperPrecedenceAcrossOperationNestedScopeAndRuntime(t *testing.T) {
	type input struct{}
	type output struct{}

	baseErr := errors.New("boom")

	makeMapper := func(calls *[]string, name string, mapped *HTTPError) ErrorMapper {
		return func(err error) *HTTPError {
			if !errors.Is(err, baseErr) {
				t.Fatalf("expected mapper input to preserve raw error, got %v", err)
			}
			*calls = append(*calls, name)
			return mapped
		}
	}

	t.Run("operation mappers win first", func(t *testing.T) {
		var calls []string
		rt := New(
			WithErrorMapper(makeMapper(&calls, "runtime-1", nil)),
			WithErrorMapper(makeMapper(&calls, "runtime-2", &HTTPError{Status: http.StatusForbidden, Code: "runtime", Message: "runtime"})),
		)
		scope := rt.Scope(
			WithErrorMapper(makeMapper(&calls, "scope-1", nil)),
			WithErrorMapper(makeMapper(&calls, "scope-2", &HTTPError{Status: http.StatusTeapot, Code: "scope", Message: "scope"})),
		)
		nested := scope.Scope(
			WithErrorMapper(makeMapper(&calls, "nested-1", nil)),
			WithErrorMapper(makeMapper(&calls, "nested-2", &HTTPError{Status: http.StatusGone, Code: "nested", Message: "nested"})),
		)

		handler := Handle(nested, Operation[input, output]{
			ErrorMappers: []ErrorMapper{
				makeMapper(&calls, "op-1", nil),
				makeMapper(&calls, "op-2", &HTTPError{Status: http.StatusNotFound, Code: "op", Message: "op"}),
			},
		}, func(_ context.Context, _ *input) (*output, error) {
			return nil, baseErr
		})

		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/mapped", nil))

		assertErrorEnvelopeCode(t, rec, http.StatusNotFound, "op")
		assertStringSlice(t, calls, []string{"op-1", "op-2"})
	})

	t.Run("inner scope outranks outer scope and runtime", func(t *testing.T) {
		var calls []string
		rt := New(
			WithErrorMapper(makeMapper(&calls, "runtime-1", nil)),
			WithErrorMapper(makeMapper(&calls, "runtime-2", &HTTPError{Status: http.StatusForbidden, Code: "runtime", Message: "runtime"})),
		)
		scope := rt.Scope(
			WithErrorMapper(makeMapper(&calls, "scope-1", nil)),
			WithErrorMapper(makeMapper(&calls, "scope-2", &HTTPError{Status: http.StatusTeapot, Code: "scope", Message: "scope"})),
		)
		nested := scope.Scope(
			WithErrorMapper(makeMapper(&calls, "nested-1", nil)),
			WithErrorMapper(makeMapper(&calls, "nested-2", &HTTPError{Status: http.StatusGone, Code: "nested", Message: "nested"})),
		)

		handler := Handle(nested, Operation[input, output]{
			ErrorMappers: []ErrorMapper{
				makeMapper(&calls, "op-1", nil),
				makeMapper(&calls, "op-2", nil),
			},
		}, func(_ context.Context, _ *input) (*output, error) {
			return nil, baseErr
		})

		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/mapped", nil))

		assertErrorEnvelopeCode(t, rec, http.StatusGone, "nested")
		assertStringSlice(t, calls, []string{"op-1", "op-2", "nested-1", "nested-2"})
	})

	t.Run("runtime mappers run after unmatched scopes", func(t *testing.T) {
		var calls []string
		rt := New(
			WithErrorMapper(makeMapper(&calls, "runtime-1", nil)),
			WithErrorMapper(makeMapper(&calls, "runtime-2", &HTTPError{Status: http.StatusForbidden, Code: "runtime", Message: "runtime"})),
		)
		scope := rt.Scope(
			WithErrorMapper(makeMapper(&calls, "scope-1", nil)),
			WithErrorMapper(makeMapper(&calls, "scope-2", nil)),
		)
		nested := scope.Scope(
			WithErrorMapper(makeMapper(&calls, "nested-1", nil)),
			WithErrorMapper(makeMapper(&calls, "nested-2", nil)),
		)

		handler := Handle(nested, Operation[input, output]{
			ErrorMappers: []ErrorMapper{
				makeMapper(&calls, "op-1", nil),
				makeMapper(&calls, "op-2", nil),
			},
		}, func(_ context.Context, _ *input) (*output, error) {
			return nil, baseErr
		})

		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/mapped", nil))

		assertErrorEnvelopeCode(t, rec, http.StatusForbidden, "runtime")
		assertStringSlice(t, calls, []string{
			"op-1",
			"op-2",
			"nested-1",
			"nested-2",
			"scope-1",
			"scope-2",
			"runtime-1",
			"runtime-2",
		})
	})
}

func TestHandleWrappedHTTPErrorBypassesMappers(t *testing.T) {
	type input struct{}
	type output struct{}

	public := &HTTPError{Status: http.StatusConflict, Code: "conflict", Message: "conflict"}
	mapperCalls := 0
	countingMapper := func(error) *HTTPError {
		mapperCalls++
		return &HTTPError{Status: http.StatusNotFound, Code: "mapped", Message: "mapped"}
	}

	rt := New(WithErrorMapper(countingMapper))
	scope := rt.Scope(WithErrorMapper(countingMapper))
	handler := Handle(scope, Operation[input, output]{
		ErrorMappers: []ErrorMapper{countingMapper},
	}, func(_ context.Context, _ *input) (*output, error) {
		return nil, fmt.Errorf("wrapped: %w", public)
	})

	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/public", nil))

	assertErrorEnvelopeCode(t, rec, http.StatusConflict, "conflict")
	if mapperCalls != 0 {
		t.Fatalf("wrapped public error should bypass mapper chain, got %d calls", mapperCalls)
	}
}

func TestHandleRuntimePublicErrorsBypassMappers(t *testing.T) {
	type input struct {
		Limit int    `query:"limit"`
		Name  string `json:"name" validate:"required"`
	}

	cases := []struct {
		name       string
		operation  Operation[input, struct{}]
		request    *http.Request
		wantStatus int
		wantCode   string
	}{
		{
			name: "400 request shape",
			request: httptest.NewRequest(
				http.MethodGet,
				"/users?limit=1&limit=2",
				nil,
			),
			wantStatus: http.StatusBadRequest,
			wantCode:   "bad_request",
		},
		{
			name: "415 unsupported media type",
			operation: Operation[input, struct{}]{
				Method: http.MethodPost,
			},
			request: func() *http.Request {
				req := httptest.NewRequest(http.MethodPost, "/users", strings.NewReader("name=Ada"))
				req.Header.Set("Content-Type", "text/plain")
				return req
			}(),
			wantStatus: http.StatusUnsupportedMediaType,
			wantCode:   "unsupported_media_type",
		},
		{
			name: "422 invalid request",
			operation: Operation[input, struct{}]{
				Method: http.MethodPost,
			},
			request: func() *http.Request {
				req := httptest.NewRequest(http.MethodPost, "/users", strings.NewReader(`{}`))
				req.Header.Set("Content-Type", "application/json")
				return req
			}(),
			wantStatus: http.StatusUnprocessableEntity,
			wantCode:   "invalid_request",
		},
	}

	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			mapperCalls := 0
			countingMapper := func(error) *HTTPError {
				mapperCalls++
				return &HTTPError{Status: http.StatusTeapot, Code: "mapped", Message: "mapped"}
			}

			rt := New(WithErrorMapper(countingMapper))
			scope := rt.Scope(WithErrorMapper(countingMapper))
			op := tt.operation
			op.ErrorMappers = []ErrorMapper{countingMapper}

			handler := Handle(scope, op, func(_ context.Context, _ *input) (*struct{}, error) {
				t.Fatal("handler should not run")
				return nil, nil
			})

			rec := httptest.NewRecorder()
			handler.ServeHTTP(rec, tt.request)

			assertErrorEnvelopeCode(t, rec, tt.wantStatus, tt.wantCode)
			if mapperCalls != 0 {
				t.Fatalf("runtime public error should bypass mapper chain, got %d calls", mapperCalls)
			}
		})
	}
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

func TestScopeFailureObservationConfigInheritanceAndOverride(t *testing.T) {
	type input struct{}
	type output struct{}

	sentinel := errors.New("boom")
	var runtimeEvents []Event
	var scopeEvents []Event

	rt := New(
		WithObserver(func(event Event) {
			runtimeEvents = append(runtimeEvents, event)
		}),
		WithExtractor(func(*http.Request) RequestContext {
			return RequestContext{Target: "runtime"}
		}),
	)

	inherited := rt.Scope()
	overridden := rt.Scope(
		WithObserver(func(event Event) {
			scopeEvents = append(scopeEvents, event)
		}),
		WithExtractor(func(*http.Request) RequestContext {
			return RequestContext{Target: "scope"}
		}),
	)

	inheritedHandler := Handle(inherited, Operation[input, output]{}, func(_ context.Context, _ *input) (*output, error) {
		return nil, sentinel
	})
	overriddenHandler := Handle(overridden, Operation[input, output]{}, func(_ context.Context, _ *input) (*output, error) {
		return nil, sentinel
	})

	recInherited := httptest.NewRecorder()
	inheritedHandler.ServeHTTP(recInherited, httptest.NewRequest(http.MethodGet, "/users", nil))
	assertErrorEnvelopeCode(t, recInherited, http.StatusInternalServerError, "internal_error")

	if len(runtimeEvents) != 1 || len(scopeEvents) != 0 {
		t.Fatalf("expected inherited scope to reuse runtime observer, got runtime=%d scope=%d", len(runtimeEvents), len(scopeEvents))
	}
	if runtimeEvents[0].Target != "runtime" {
		t.Fatalf("expected inherited scope to reuse runtime extractor, got %+v", runtimeEvents[0])
	}

	recOverridden := httptest.NewRecorder()
	overriddenHandler.ServeHTTP(recOverridden, httptest.NewRequest(http.MethodGet, "/users", nil))
	assertErrorEnvelopeCode(t, recOverridden, http.StatusInternalServerError, "internal_error")

	if len(runtimeEvents) != 1 || len(scopeEvents) != 1 {
		t.Fatalf("expected overridden scope to use nearest observer, got runtime=%d scope=%d", len(runtimeEvents), len(scopeEvents))
	}
	if scopeEvents[0].Target != "scope" {
		t.Fatalf("expected overridden scope to use nearest extractor, got %+v", scopeEvents[0])
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

func TestObserverEmitsBoundaryFailureBeforeErrorWrite(t *testing.T) {
	type input struct{}
	type output struct{}

	sentinel := errors.New("boom")
	var events []Event

	rt := New(
		WithObserver(func(event Event) {
			events = append(events, event)
		}),
		WithErrorMapper(func(err error) *HTTPError {
			if errors.Is(err, sentinel) {
				return &HTTPError{Status: http.StatusBadGateway, Code: "upstream_failure", Message: "upstream failure"}
			}
			return nil
		}),
	)

	handler := Handle(rt, Operation[input, output]{}, func(_ context.Context, _ *input) (*output, error) {
		return nil, sentinel
	})

	writer := &recordingResponseWriter{}
	writer.onWrite = func() {
		if len(events) != 1 {
			t.Fatalf("expected boundary event before first error write, got %d events", len(events))
		}
		if !errors.Is(events[0].Error, sentinel) || events[0].Public == nil || events[0].Public.Code != "upstream_failure" {
			t.Fatalf("unexpected boundary event before write: %+v", events[0])
		}
		if writer.status != http.StatusBadGateway {
			t.Fatalf("expected public status to be selected before write, got %d", writer.status)
		}
	}

	handler.ServeHTTP(writer, httptest.NewRequest(http.MethodGet, "/users", nil))

	if len(events) != 1 {
		t.Fatalf("expected exactly one failure event, got %d", len(events))
	}
	assertErrorEnvelopePayload(t, writer.status, writer.body, http.StatusBadGateway, "upstream_failure")
}

func TestObserverEmitsInternalErrorWhenErrorResponseWriteFails(t *testing.T) {
	type input struct{}
	type output struct{}

	sentinel := errors.New("boom")
	writeErr := errors.New("write failed")
	var events []Event

	rt := New(
		WithObserver(func(event Event) {
			events = append(events, event)
		}),
		WithErrorMapper(func(err error) *HTTPError {
			if errors.Is(err, sentinel) {
				return &HTTPError{Status: http.StatusBadGateway, Code: "upstream_failure", Message: "upstream failure"}
			}
			return nil
		}),
	)

	handler := Handle(rt, Operation[input, output]{}, func(_ context.Context, _ *input) (*output, error) {
		return nil, sentinel
	})

	writer := &recordingResponseWriter{writeErr: writeErr}
	writer.onWrite = func() {
		if len(events) != 1 {
			t.Fatalf("expected boundary event before failing write, got %d events", len(events))
		}
	}

	handler.ServeHTTP(writer, httptest.NewRequest(http.MethodGet, "/users", nil))

	if writer.status != http.StatusBadGateway {
		t.Fatalf("expected public status write attempt before write failure, got %d", writer.status)
	}
	if len(events) != 2 {
		t.Fatalf("expected boundary event plus supplemental internal-error event, got %d", len(events))
	}
	if !errors.Is(events[0].Error, sentinel) || events[0].Public == nil || events[0].Public.Code != "upstream_failure" {
		t.Fatalf("unexpected primary failure event: %+v", events[0])
	}
	if !errors.Is(events[1].Error, writeErr) || events[1].Public == nil || events[1].Public.Code != "internal_error" {
		t.Fatalf("unexpected supplemental internal-error event: %+v", events[1])
	}
}

func assertErrorEnvelopeCode(t *testing.T, rec *httptest.ResponseRecorder, status int, code string) {
	t.Helper()

	assertErrorEnvelopePayload(t, rec.Code, rec.Body.Bytes(), status, code)
}

func assertErrorEnvelopePayload(t *testing.T, gotStatus int, body []byte, wantStatus int, wantCode string) {
	t.Helper()

	if gotStatus != wantStatus {
		t.Fatalf("expected %d, got %d", wantStatus, gotStatus)
	}

	var envelope struct {
		Error struct {
			Code       string          `json:"code"`
			Message    string          `json:"message"`
			DetailsRaw json.RawMessage `json:"details"`
		} `json:"error"`
	}
	if err := json.Unmarshal(body, &envelope); err != nil {
		t.Fatalf("decode error response: %v", err)
	}

	if envelope.Error.Code != wantCode {
		t.Fatalf("expected error code %q, got %+v", wantCode, envelope)
	}
	if string(envelope.Error.DetailsRaw) == "null" {
		t.Fatalf("expected details to be a JSON array, got %q", string(envelope.Error.DetailsRaw))
	}
}

func assertStringSlice(t *testing.T, got []string, want []string) {
	t.Helper()

	if len(got) != len(want) {
		t.Fatalf("expected %v, got %v", want, got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("expected %v, got %v", want, got)
		}
	}
}

type recordingResponseWriter struct {
	header      http.Header
	status      int
	wroteHeader bool
	body        []byte
	writeErr    error
	onWrite     func()
}

func (w *recordingResponseWriter) Header() http.Header {
	if w.header == nil {
		w.header = make(http.Header)
	}
	return w.header
}

func (w *recordingResponseWriter) WriteHeader(status int) {
	if w.wroteHeader {
		return
	}
	w.status = status
	w.wroteHeader = true
}

func (w *recordingResponseWriter) Write(p []byte) (int, error) {
	if !w.wroteHeader {
		w.WriteHeader(http.StatusOK)
	}
	if w.onWrite != nil {
		w.onWrite()
	}
	if w.writeErr != nil {
		return 0, w.writeErr
	}
	w.body = append(w.body, p...)
	return len(p), nil
}
