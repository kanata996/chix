package chix

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

type upsertWidgetInput struct {
	ID      string `path:"id"`
	Created bool   `query:"created"`
}

type upsertWidgetOutput struct {
	ID      string `json:"id"`
	Created bool   `json:"created"`
}

func (o *upsertWidgetOutput) ResponseStatus() int {
	if o != nil && o.Created {
		return http.StatusCreated
	}
	return http.StatusOK
}

func (o *upsertWidgetOutput) ResponseHeaders() http.Header {
	if o == nil || !o.Created {
		return nil
	}
	headers := http.Header{}
	headers.Set("Location", "/widgets/"+o.ID)
	return headers
}

func TestOpenAPIDocumentSupportsMultipleResponsesAndResponseHeaders(t *testing.T) {
	app := New(Config{})
	Register(app, Operation{
		Method: http.MethodPut,
		Path:   "/widgets/{id}",
		Responses: []OperationResponse{
			{Status: http.StatusOK, Description: "Widget updated"},
			{
				Status:      http.StatusCreated,
				Description: "Widget created",
				Headers: map[string]HeaderDoc{
					"Location": {
						Description: "Created resource URL",
						Schema:      &Schema{Type: "string", Format: "uri"},
					},
				},
			},
		},
	}, func(_ context.Context, input *upsertWidgetInput) (*upsertWidgetOutput, error) {
		return &upsertWidgetOutput{
			ID:      input.ID,
			Created: input.Created,
		}, nil
	})

	req := httptest.NewRequest(http.MethodPut, "/widgets/42?created=true", nil)
	rec := httptest.NewRecorder()
	app.ServeHTTP(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d", rec.Code)
	}
	if rec.Header().Get("Location") != "/widgets/42" {
		t.Fatalf("expected Location header, got %q", rec.Header().Get("Location"))
	}

	var got upsertWidgetOutput
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if got.ID != "42" || !got.Created {
		t.Fatalf("unexpected response body: %+v", got)
	}

	doc := app.OpenAPIDocument()
	path := doc.Paths["/widgets/{id}"]
	if path.Put == nil {
		t.Fatalf("expected PUT operation in spec")
	}
	if path.Put.Responses["200"].Description != "Widget updated" {
		t.Fatalf("expected 200 response doc, got %+v", path.Put.Responses["200"])
	}
	created := path.Put.Responses["201"]
	if created.Description != "Widget created" {
		t.Fatalf("expected 201 response doc, got %+v", created)
	}
	location := created.Headers["Location"]
	if location.Description != "Created resource URL" {
		t.Fatalf("expected Location header description, got %+v", location)
	}
	if location.Schema == nil || location.Schema.Type != "string" || location.Schema.Format != "uri" {
		t.Fatalf("expected Location header schema, got %+v", location.Schema)
	}
	responseSchema := resolvedSchema(doc, created.Content["application/json"].Schema)
	if responseSchema.Properties["id"] == nil {
		t.Fatalf("expected success response schema, got %+v", responseSchema)
	}
}

func TestRegisterSupportsExplicitNoContentResponse(t *testing.T) {
	type deleteWidgetInput struct {
		ID string `path:"id"`
	}

	app := New(Config{})
	Register(app, Operation{
		Method: http.MethodDelete,
		Path:   "/widgets/{id}",
		Responses: []OperationResponse{
			{Status: http.StatusNoContent, Description: "Widget deleted", NoBody: true},
		},
	}, func(_ context.Context, _ *deleteWidgetInput) (*struct{}, error) {
		return nil, nil
	})

	req := httptest.NewRequest(http.MethodDelete, "/widgets/42", nil)
	rec := httptest.NewRecorder()
	app.ServeHTTP(rec, req)

	if rec.Code != http.StatusNoContent {
		t.Fatalf("expected 204, got %d", rec.Code)
	}
	if rec.Body.Len() != 0 {
		t.Fatalf("expected empty body, got %q", rec.Body.String())
	}

	doc := app.OpenAPIDocument()
	path := doc.Paths["/widgets/{id}"]
	if path.Delete == nil {
		t.Fatalf("expected DELETE operation in spec")
	}
	if _, ok := path.Delete.Responses["200"]; ok {
		t.Fatalf("did not expect legacy 200 response when explicit 204 is configured")
	}
	if path.Delete.Responses["204"].Content != nil {
		t.Fatalf("did not expect 204 response content, got %+v", path.Delete.Responses["204"])
	}
}

func TestOpenAPIDocumentSupportsExplicitErrorResponses(t *testing.T) {
	type createLockInput struct {
		ID string `path:"id"`
	}

	app := New(Config{})
	Register(app, Operation{
		Method: http.MethodPost,
		Path:   "/locks/{id}",
		Responses: []OperationResponse{
			{
				Status:      http.StatusConflict,
				Description: "Lock version conflict",
				Headers: map[string]HeaderDoc{
					"Retry-After": {
						Description: "Seconds to wait before retrying",
						Schema:      &Schema{Type: "integer"},
					},
				},
			},
		},
	}, func(_ context.Context, _ *createLockInput) (*struct{}, error) {
		return nil, StatusError(http.StatusConflict, "lock version conflict").WithHeader("Retry-After", "120")
	})

	req := httptest.NewRequest(http.MethodPost, "/locks/42", nil)
	rec := httptest.NewRecorder()
	app.ServeHTTP(rec, req)

	if rec.Code != http.StatusConflict {
		t.Fatalf("expected 409, got %d", rec.Code)
	}
	if rec.Header().Get("Retry-After") != "120" {
		t.Fatalf("expected Retry-After header, got %q", rec.Header().Get("Retry-After"))
	}

	doc := app.OpenAPIDocument()
	path := doc.Paths["/locks/{id}"]
	if path.Post == nil {
		t.Fatalf("expected POST operation in spec")
	}
	conflict := path.Post.Responses["409"]
	if conflict.Description != "Lock version conflict" {
		t.Fatalf("expected explicit 409 response doc, got %+v", conflict)
	}
	retryAfter := conflict.Headers["Retry-After"]
	if retryAfter.Description != "Seconds to wait before retrying" {
		t.Fatalf("expected Retry-After header description, got %+v", retryAfter)
	}
	if retryAfter.Schema == nil || retryAfter.Schema.Type != "integer" {
		t.Fatalf("expected Retry-After header schema, got %+v", retryAfter.Schema)
	}
	problem := resolvedSchema(doc, conflict.Content["application/problem+json"].Schema)
	if problem.Properties["status"] == nil {
		t.Fatalf("expected explicit error response to use problem schema, got %+v", problem)
	}
}

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
	if path.Get.Responses["400"].Description != "Invalid request" {
		t.Fatalf("expected explicit 400 response, got %+v", path.Get.Responses)
	}
	if _, ok := path.Get.Responses["422"]; ok {
		t.Fatalf("did not expect 422 response for input without validate rules")
	}

	response := path.Get.Responses["200"]
	responseSchema := resolvedSchema(doc, response.Content["application/json"].Schema)
	itemsSchema := resolvedSchema(doc, responseSchema.Properties["items"].Items)
	if itemsSchema.Properties["id"] == nil {
		t.Fatalf("expected nested response schema in OpenAPI")
	}
}

func TestOpenAPIDocumentIncludesValidationConstraints(t *testing.T) {
	type profileInput struct {
		ZIP string `json:"zip" validate:"len=6"`
	}

	type createUserInput struct {
		Role    string        `query:"role" validate:"required,oneof=admin member"`
		Email   string        `json:"email,omitempty" validate:"required,email"`
		Name    string        `json:"name" validate:"min=3,max=20"`
		Code    string        `json:"code,omitempty" validate:"len=8"`
		Tags    []string      `json:"tags,omitempty" validate:"max=3"`
		Profile *profileInput `json:"profile,omitempty"`
	}

	type createUserOutput struct {
		Status string `json:"status" validate:"oneof=ok fail"`
	}

	app := New(Config{})
	Register(app, Operation{
		Method: http.MethodPost,
		Path:   "/users",
	}, func(_ context.Context, input *createUserInput) (*createUserOutput, error) {
		return &createUserOutput{}, nil
	})

	doc := app.OpenAPIDocument()
	path := doc.Paths["/users"]
	if path.Post == nil {
		t.Fatalf("expected POST operation in spec")
	}

	if path.Post.RequestBody == nil || !path.Post.RequestBody.Required {
		t.Fatalf("expected request body to be marked required")
	}
	if path.Post.Responses["400"].Description != "Invalid request" {
		t.Fatalf("expected explicit 400 response, got %+v", path.Post.Responses)
	}
	if path.Post.Responses["422"].Description != "Request validation failed" {
		t.Fatalf("expected explicit 422 response, got %+v", path.Post.Responses)
	}
	validationProblem := resolvedSchema(doc, path.Post.Responses["422"].Content["application/problem+json"].Schema)
	if validationProblem.Properties["violations"] == nil {
		t.Fatalf("expected 422 problem schema to expose violations")
	}

	var roleParam *Parameter
	for i := range path.Post.Parameters {
		if path.Post.Parameters[i].Name == "role" {
			roleParam = &path.Post.Parameters[i]
			break
		}
	}
	if roleParam == nil {
		t.Fatalf("expected role query parameter")
	}
	if !roleParam.Required {
		t.Fatalf("expected role parameter to be required")
	}
	if len(roleParam.Schema.Enum) != 2 || roleParam.Schema.Enum[0] != "admin" || roleParam.Schema.Enum[1] != "member" {
		t.Fatalf("expected role enum, got %+v", roleParam.Schema.Enum)
	}

	bodySchema := resolvedSchema(doc, path.Post.RequestBody.Content["application/json"].Schema)
	required := map[string]bool{}
	for _, name := range bodySchema.Required {
		required[name] = true
	}
	if !required["email"] || !required["name"] {
		t.Fatalf("expected required body fields, got %+v", bodySchema.Required)
	}

	email := bodySchema.Properties["email"]
	if email.Format != "email" {
		t.Fatalf("expected email format, got %+v", email)
	}

	name := bodySchema.Properties["name"]
	if name.MinLength == nil || *name.MinLength != 3 {
		t.Fatalf("expected name minLength=3, got %+v", name)
	}
	if name.MaxLength == nil || *name.MaxLength != 20 {
		t.Fatalf("expected name maxLength=20, got %+v", name)
	}

	code := bodySchema.Properties["code"]
	if code.MinLength == nil || *code.MinLength != 8 || code.MaxLength == nil || *code.MaxLength != 8 {
		t.Fatalf("expected code len=8, got %+v", code)
	}

	tags := bodySchema.Properties["tags"]
	if tags.MaxItems == nil || *tags.MaxItems != 3 {
		t.Fatalf("expected tags maxItems=3, got %+v", tags)
	}

	profile := bodySchema.Properties["profile"]
	if !profile.Nullable {
		t.Fatalf("expected profile to remain nullable")
	}
	zip := resolvedSchema(doc, profile).Properties["zip"]
	if zip.MinLength == nil || *zip.MinLength != 6 || zip.MaxLength == nil || *zip.MaxLength != 6 {
		t.Fatalf("expected nested zip len=6, got %+v", zip)
	}

	responseStatus := resolvedSchema(doc, path.Post.Responses["201"].Content["application/json"].Schema).Properties["status"]
	if len(responseStatus.Enum) != 0 {
		t.Fatalf("did not expect response schema validation enum, got %+v", responseStatus.Enum)
	}
}

func TestOpenAPIDocumentUsesComponentsAndRefs(t *testing.T) {
	type user struct {
		ID string `json:"id"`
	}

	type getUserOutput struct {
		Item user `json:"item"`
	}

	type listUsersOutput struct {
		Items []user `json:"items"`
	}

	app := New(Config{})
	Register(app, Operation{
		Method: http.MethodGet,
		Path:   "/user",
	}, func(_ context.Context, _ *struct{}) (*getUserOutput, error) {
		return &getUserOutput{}, nil
	})
	Register(app, Operation{
		Method: http.MethodGet,
		Path:   "/users",
	}, func(_ context.Context, _ *struct{}) (*listUsersOutput, error) {
		return &listUsersOutput{}, nil
	})

	doc := app.OpenAPIDocument()
	if doc.Components == nil || len(doc.Components.Schemas) == 0 {
		t.Fatalf("expected OpenAPI components to be populated")
	}

	getSchema := resolvedSchema(doc, doc.Paths["/user"].Get.Responses["200"].Content["application/json"].Schema)
	listSchema := resolvedSchema(doc, doc.Paths["/users"].Get.Responses["200"].Content["application/json"].Schema)

	itemSchema := getSchema.Properties["item"]
	itemsSchema := listSchema.Properties["items"]
	if itemSchema.Ref == "" {
		t.Fatalf("expected object property to use $ref")
	}
	if itemsSchema.Items == nil || itemsSchema.Items.Ref == "" {
		t.Fatalf("expected array items to use $ref")
	}
	if itemSchema.Ref != itemsSchema.Items.Ref {
		t.Fatalf("expected repeated user schema to reuse one component, got %q and %q", itemSchema.Ref, itemsSchema.Items.Ref)
	}
}

func TestOpenAPIDocumentSupportsCustomSchemaNames(t *testing.T) {
	type profileInput struct {
		ID string `json:"id"`
	}

	type user struct {
		ID string `json:"id"`
	}

	type team struct {
		ID string `json:"id"`
	}

	type createUserInput struct {
		Profile profileInput `json:"profile"`
	}

	type createUserOutput struct {
		User user `json:"user"`
	}

	type getTeamOutput struct {
		Team team `json:"team"`
	}

	app := New(Config{
		OpenAPISchemaNamer: func(ctx OpenAPISchemaNameContext) string {
			switch ctx.Type.Name() {
			case "profileInput":
				if ctx.Request {
					return "ProfilePayload"
				}
			case "createUserOutput":
				return "CreateUserView"
			case "user", "team":
				return "Actor"
			}
			return ""
		},
	})

	Register(app, Operation{
		Method: http.MethodPost,
		Path:   "/users",
	}, func(_ context.Context, _ *createUserInput) (*createUserOutput, error) {
		return &createUserOutput{}, nil
	})
	Register(app, Operation{
		Method: http.MethodGet,
		Path:   "/teams/current",
	}, func(_ context.Context, _ *struct{}) (*getTeamOutput, error) {
		return &getTeamOutput{}, nil
	})

	doc := app.OpenAPIDocument()
	if doc.Components == nil || doc.Components.Schemas == nil {
		t.Fatalf("expected components schemas")
	}
	if doc.Components.Schemas["ProfilePayload"] == nil {
		t.Fatalf("expected custom request schema name")
	}
	if doc.Components.Schemas["CreateUserView"] == nil {
		t.Fatalf("expected custom response schema name")
	}
	if doc.Components.Schemas["Actor"] == nil || doc.Components.Schemas["Actor2"] == nil {
		t.Fatalf("expected colliding custom names to be deduplicated, got %+v", doc.Components.Schemas)
	}

	post := doc.Paths["/users"].Post
	if post == nil {
		t.Fatalf("expected POST operation")
	}
	bodySchema := resolvedSchema(doc, post.RequestBody.Content["application/json"].Schema)
	if bodySchema.Properties["profile"].Ref != "#/components/schemas/ProfilePayload" {
		t.Fatalf("expected nested request schema to use custom name, got %+v", bodySchema.Properties["profile"])
	}
	if post.Responses["201"].Content["application/json"].Schema.Ref != "#/components/schemas/CreateUserView" {
		t.Fatalf("expected response root schema to use custom name, got %+v", post.Responses["201"].Content["application/json"].Schema)
	}

	teamResponse := resolvedSchema(doc, doc.Paths["/teams/current"].Get.Responses["200"].Content["application/json"].Schema)
	if teamResponse.Properties["team"].Ref != "#/components/schemas/Actor2" {
		t.Fatalf("expected colliding custom schema name to be suffixed, got %+v", teamResponse.Properties["team"])
	}
}

func TestOpenAPIDocumentOmitsRequestErrorResponsesForEmptyInput(t *testing.T) {
	app := New(Config{})
	Register(app, Operation{
		Method: http.MethodGet,
		Path:   "/health",
	}, func(_ context.Context, _ *struct{}) (*struct {
		OK bool `json:"ok"`
	}, error) {
		return &struct {
			OK bool `json:"ok"`
		}{OK: true}, nil
	})

	doc := app.OpenAPIDocument()
	path := doc.Paths["/health"]
	if path.Get == nil {
		t.Fatalf("expected GET operation in spec")
	}
	if _, ok := path.Get.Responses["400"]; ok {
		t.Fatalf("did not expect 400 response for empty input")
	}
	if _, ok := path.Get.Responses["422"]; ok {
		t.Fatalf("did not expect 422 response for empty input")
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

func TestValidationFailureReturnsProblemDocument(t *testing.T) {
	type createUserInput struct {
		Role  string `query:"role" validate:"oneof=admin member"`
		Email string `json:"email" validate:"required,email"`
	}

	app := New(Config{})
	Register(app, Operation{
		Method: http.MethodPost,
		Path:   "/users",
	}, func(_ context.Context, input *createUserInput) (*struct{}, error) {
		return &struct{}{}, nil
	})

	req := httptest.NewRequest(http.MethodPost, "/users?role=guest", strings.NewReader(`{"email":"nope"}`))
	req.Header.Set("Content-Type", "application/json")

	rec := httptest.NewRecorder()
	app.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("expected 422, got %d", rec.Code)
	}

	var problem Problem
	if err := json.Unmarshal(rec.Body.Bytes(), &problem); err != nil {
		t.Fatalf("decode problem: %v", err)
	}
	if problem.Detail != "request validation failed" {
		t.Fatalf("unexpected problem detail: %+v", problem)
	}
	if len(problem.Violations) != 2 {
		t.Fatalf("expected 2 violations, got %d", len(problem.Violations))
	}

	got := map[string]string{}
	for _, violation := range problem.Violations {
		got[violation.Source+":"+violation.Field] = violation.Rule
	}

	if got["query:role"] != "oneof" {
		t.Fatalf("expected query role violation, got %+v", problem.Violations)
	}
	if got["body:email"] != "email" {
		t.Fatalf("expected body email violation, got %+v", problem.Violations)
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

func resolvedSchema(doc Document, schema *Schema) *Schema {
	if schema == nil {
		return nil
	}
	if schema.Ref == "" {
		return schema
	}
	const prefix = "#/components/schemas/"
	name := strings.TrimPrefix(schema.Ref, prefix)
	if doc.Components == nil || doc.Components.Schemas == nil {
		return nil
	}
	return doc.Components.Schemas[name]
}
