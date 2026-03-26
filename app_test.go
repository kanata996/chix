package chix

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestRegisterBindsInputAndWritesJSON(t *testing.T) {
	type createUserInput struct {
		ID      string `path:"id" doc:"User identifier"`
		Verbose bool   `query:"verbose"`
		TraceID string `header:"X-Trace-ID"`
		Name    string `json:"name"`
	}

	type createUserOutput struct {
		ID      string `json:"id"`
		Name    string `json:"name"`
		Verbose bool   `json:"verbose"`
		TraceID string `json:"traceID"`
	}

	app := New(Config{Title: "Test API", Version: "1.0.0"})
	Register(app, Operation{
		Method:      http.MethodPost,
		Path:        "/users/{id}",
		OperationID: "createUser",
		Summary:     "Create a user",
	}, func(_ context.Context, input *createUserInput) (*createUserOutput, error) {
		return &createUserOutput{
			ID:      input.ID,
			Name:    input.Name,
			Verbose: input.Verbose,
			TraceID: input.TraceID,
		}, nil
	})

	req := httptest.NewRequest(http.MethodPost, "/users/42?verbose=true", strings.NewReader(`{"name":"Ada"}`))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Trace-ID", "trace-123")

	rec := httptest.NewRecorder()
	app.ServeHTTP(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d", rec.Code)
	}

	var got createUserOutput
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatalf("decode response: %v", err)
	}

	if got.ID != "42" || got.Name != "Ada" || !got.Verbose || got.TraceID != "trace-123" {
		t.Fatalf("unexpected response: %+v", got)
	}
}

func TestOpenAPIDocumentIncludesParametersAndBody(t *testing.T) {
	type listUsersInput struct {
		AccountID string `path:"accountID" doc:"Account identifier"`
		Limit     int    `query:"limit"`
	}

	type user struct {
		ID   string `json:"id"`
		Name string `json:"name"`
	}

	type listUsersOutput struct {
		Items []user `json:"items"`
	}

	app := New(Config{Title: "Catalog API", Version: "2026.03.27"})
	Register(app, Operation{
		Method:      http.MethodGet,
		Path:        "/accounts/{accountID}/users",
		OperationID: "listUsers",
		Tags:        []string{"users"},
	}, func(_ context.Context, input *listUsersInput) (*listUsersOutput, error) {
		return &listUsersOutput{}, nil
	})

	req := httptest.NewRequest(http.MethodGet, "/openapi.json", nil)
	rec := httptest.NewRecorder()
	app.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}

	var doc Document
	if err := json.Unmarshal(rec.Body.Bytes(), &doc); err != nil {
		t.Fatalf("decode openapi: %v", err)
	}

	path := doc.Paths["/accounts/{accountID}/users"]
	if path.Get == nil {
		t.Fatalf("expected GET operation in spec")
	}
	if path.Get.OperationID != "listUsers" {
		t.Fatalf("unexpected operation id: %s", path.Get.OperationID)
	}
	if len(path.Get.Parameters) != 2 {
		t.Fatalf("expected 2 parameters, got %d", len(path.Get.Parameters))
	}
	if path.Get.RequestBody != nil {
		t.Fatalf("did not expect request body for parameter-only input")
	}

	response := path.Get.Responses["200"]
	if response.Content["application/json"].Schema.Properties["items"].Items.Properties["id"] == nil {
		t.Fatalf("expected nested response schema in OpenAPI")
	}
}

func TestWriteErrorReturnsProblemDocument(t *testing.T) {
	app := New(Config{})
	Register(app, Operation{
		Method: http.MethodGet,
		Path:   "/boom",
	}, func(_ context.Context, _ *struct{}) (*struct{}, error) {
		return nil, StatusError(http.StatusBadRequest, "broken request")
	})

	req := httptest.NewRequest(http.MethodGet, "/boom", nil)
	rec := httptest.NewRecorder()
	app.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", rec.Code)
	}

	var problem Problem
	if err := json.Unmarshal(rec.Body.Bytes(), &problem); err != nil {
		t.Fatalf("decode problem: %v", err)
	}
	if problem.Detail != "broken request" {
		t.Fatalf("unexpected problem detail: %+v", problem)
	}
	if problem.RequestID == "" {
		t.Fatalf("expected request id to be propagated in problem response")
	}
}

func TestMissingRequiredBodyReturnsBadRequest(t *testing.T) {
	type createWidgetInput struct {
		Name string `json:"name"`
	}

	app := New(Config{})
	Register(app, Operation{
		Method: http.MethodPost,
		Path:   "/widgets",
	}, func(_ context.Context, input *createWidgetInput) (*struct{}, error) {
		return &struct{}{}, nil
	})

	req := httptest.NewRequest(http.MethodPost, "/widgets", nil)
	rec := httptest.NewRecorder()
	app.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", rec.Code)
	}
}

func TestDocsRouteServesSwaggerUI(t *testing.T) {
	app := New(Config{})

	req := httptest.NewRequest(http.MethodGet, "/docs", nil)
	rec := httptest.NewRecorder()
	app.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), "SwaggerUIBundle") {
		t.Fatalf("expected swagger ui page, got %q", rec.Body.String())
	}
}
