package reqx

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/yogorobot/app-mall/chix/resp"
)

func TestBindAndValidatePath_BindsTrimmedParams(t *testing.T) {
	type pathRequest struct {
		UUID string `param:"uuid" validate:"required,uuid"`
	}

	req := requestWithPathParams(map[string][]string{
		"uuid": {" 550e8400-e29b-41d4-a716-446655440000 "},
	})

	var path pathRequest
	if err := BindAndValidatePath(req, &path); err != nil {
		t.Fatalf("BindAndValidatePath() error = %v", err)
	}
	if path.UUID != "550e8400-e29b-41d4-a716-446655440000" {
		t.Fatalf("BindAndValidatePath() uuid = %q", path.UUID)
	}
}

func TestBindPathValues_TypeMismatchReturnsViolation(t *testing.T) {
	type pathRequest struct {
		ID int `param:"id"`
	}

	req := requestWithPathParams(map[string][]string{
		"id": {"abc"},
	})

	var path pathRequest
	err := BindPathValues(req, &path)
	if err == nil {
		t.Fatal("BindPathValues() error = nil")
	}

	httpErr, ok := err.(*resp.HTTPError)
	if !ok {
		t.Fatalf("BindPathValues() error type = %T", err)
	}
	if httpErr.Status() != http.StatusUnprocessableEntity {
		t.Fatalf("BindPathValues() status = %d", httpErr.Status())
	}

	details := httpErr.Details()
	if len(details) != 1 {
		t.Fatalf("BindPathValues() details len = %d", len(details))
	}

	violation, ok := details[0].(Violation)
	if !ok {
		t.Fatalf("BindPathValues() detail type = %T", details[0])
	}
	if violation.Field != "id" || violation.Code != ViolationCodeType || violation.Message != "must be number" {
		t.Fatalf("BindPathValues() violation = %#v", violation)
	}
}

func TestBindPathValues_RepeatedParamsReturnViolation(t *testing.T) {
	type pathRequest struct {
		ID string `param:"id"`
	}

	req := requestWithPathParams(map[string][]string{
		"id": {"1", "2"},
	})

	var path pathRequest
	err := BindPathValues(req, &path)
	if err == nil {
		t.Fatal("BindPathValues() error = nil")
	}

	httpErr, ok := err.(*resp.HTTPError)
	if !ok {
		t.Fatalf("BindPathValues() error type = %T", err)
	}

	details := httpErr.Details()
	if len(details) != 1 {
		t.Fatalf("BindPathValues() details len = %d", len(details))
	}

	violation, ok := details[0].(Violation)
	if !ok {
		t.Fatalf("BindPathValues() detail type = %T", details[0])
	}
	if violation.Field != "id" || violation.Code != ViolationCodeMultiple || violation.Message != "must not be repeated" {
		t.Fatalf("BindPathValues() violation = %#v", violation)
	}
}

func TestBindPathValues_RejectsUnsupportedFieldType(t *testing.T) {
	type nested struct {
		Name string
	}
	type pathRequest struct {
		Nested nested `param:"nested"`
	}

	req := requestWithPathParams(map[string][]string{
		"nested": {"x"},
	})

	var path pathRequest
	err := BindPathValues(req, &path)
	if err == nil {
		t.Fatal("BindPathValues() error = nil")
	}
	if got := err.Error(); got != `reqx: field "Nested" has unsupported path type reqx.nested` {
		t.Fatalf("error = %q", got)
	}
}

func TestBindPathValues_RejectsUnexportedTaggedField(t *testing.T) {
	type pathRequest struct {
		id string `param:"id"`
	}

	req := requestWithPathParams(map[string][]string{
		"id": {"1"},
	})

	var path pathRequest
	err := BindPathValues(req, &path)
	if err == nil {
		t.Fatal("BindPathValues() error = nil")
	}
	if got := err.Error(); got != `reqx: path field "id" must be exported` {
		t.Fatalf("error = %q", got)
	}
}

func TestBindPathValues_RejectsDuplicateTaggedFields(t *testing.T) {
	type pathRequest struct {
		ID   string `param:"id"`
		UUID string `param:"id"`
	}

	req := requestWithPathParams(map[string][]string{
		"id": {"1"},
	})

	var path pathRequest
	err := BindPathValues(req, &path)
	if err == nil {
		t.Fatal("BindPathValues() error = nil")
	}
	if got := err.Error(); got != `reqx: duplicate path field "id" on ID and UUID` {
		t.Fatalf("error = %q", got)
	}
}

func TestParamString_TrimsValue(t *testing.T) {
	req := requestWithPathParams(map[string][]string{
		"name": {"  kanata  "},
	})

	value, err := ParamString(req, "name")
	if err != nil {
		t.Fatalf("ParamString() error = %v", err)
	}
	if value != "kanata" {
		t.Fatalf("ParamString() value = %q", value)
	}
}

func TestParamInt_ParsesValue(t *testing.T) {
	req := requestWithPathParams(map[string][]string{
		"id": {"42"},
	})

	value, err := ParamInt(req, "id")
	if err != nil {
		t.Fatalf("ParamInt() error = %v", err)
	}
	if value != 42 {
		t.Fatalf("ParamInt() value = %d", value)
	}
}

func TestParamUUID_NormalizesValue(t *testing.T) {
	req := requestWithPathParams(map[string][]string{
		"uuid": {" 550E8400-E29B-41D4-A716-446655440000 "},
	})

	value, err := ParamUUID(req, "uuid")
	if err != nil {
		t.Fatalf("ParamUUID() error = %v", err)
	}
	if value != "550e8400-e29b-41d4-a716-446655440000" {
		t.Fatalf("ParamUUID() value = %q", value)
	}
}

func TestParamInt_InvalidValueReturnsViolation(t *testing.T) {
	req := requestWithPathParams(map[string][]string{
		"id": {"oops"},
	})

	_, err := ParamInt(req, "id")
	violation := assertSingleViolation(t, err)
	if violation.Field != "id" || violation.Code != ViolationCodeType || violation.Message != "must be number" {
		t.Fatalf("violation = %#v", violation)
	}
}

func TestParamInt_MissingValueReturnsRequired(t *testing.T) {
	req := requestWithPathParams(nil)

	_, err := ParamInt(req, "id")
	violation := assertSingleViolation(t, err)
	if violation.Field != "id" || violation.Code != ViolationCodeRequired || violation.Message != "is required" {
		t.Fatalf("violation = %#v", violation)
	}
}

func TestParamString_RepeatedValueReturnsViolation(t *testing.T) {
	req := requestWithPathParams(map[string][]string{
		"id": {"1", "2"},
	})

	_, err := ParamString(req, "id")
	violation := assertSingleViolation(t, err)
	if violation.Field != "id" || violation.Code != ViolationCodeMultiple || violation.Message != "must not be repeated" {
		t.Fatalf("violation = %#v", violation)
	}
}

func TestParamHelpers_MissingValueReturnsRequired(t *testing.T) {
	req := requestWithPathParams(nil)

	_, err := ParamString(req, "uuid")
	if err == nil {
		t.Fatal("ParamString() error = nil")
	}

	httpErr, ok := err.(*resp.HTTPError)
	if !ok {
		t.Fatalf("ParamString() error type = %T", err)
	}
	details := httpErr.Details()
	if len(details) != 1 {
		t.Fatalf("ParamString() details len = %d", len(details))
	}
	violation, ok := details[0].(Violation)
	if !ok {
		t.Fatalf("ParamString() detail type = %T", details[0])
	}
	if violation.Field != "uuid" || violation.Code != ViolationCodeRequired || violation.Message != "is required" {
		t.Fatalf("ParamString() violation = %#v", violation)
	}
}

func TestParamString_EmptyNameReturnsError(t *testing.T) {
	req := requestWithPathParams(map[string][]string{
		"id": {"1"},
	})

	_, err := ParamString(req, "   ")
	if err == nil {
		t.Fatal("ParamString() error = nil")
	}
	if got := err.Error(); got != "reqx: path param name must not be empty" {
		t.Fatalf("error = %q", got)
	}
}

func TestParamUUID_InvalidValueReturnsViolation(t *testing.T) {
	req := requestWithPathParams(map[string][]string{
		"uuid": {"not-a-uuid"},
	})

	_, err := ParamUUID(req, "uuid")
	if err == nil {
		t.Fatal("ParamUUID() error = nil")
	}

	httpErr, ok := err.(*resp.HTTPError)
	if !ok {
		t.Fatalf("ParamUUID() error type = %T", err)
	}
	details := httpErr.Details()
	if len(details) != 1 {
		t.Fatalf("ParamUUID() details len = %d", len(details))
	}
	violation, ok := details[0].(Violation)
	if !ok {
		t.Fatalf("ParamUUID() detail type = %T", details[0])
	}
	if violation.Field != "uuid" || violation.Code != ViolationCodeInvalid || violation.Message != "is invalid" {
		t.Fatalf("ParamUUID() violation = %#v", violation)
	}
}

func TestParamUUID_MissingValueReturnsRequired(t *testing.T) {
	req := requestWithPathParams(nil)

	_, err := ParamUUID(req, "uuid")
	violation := assertSingleViolation(t, err)
	if violation.Field != "uuid" || violation.Code != ViolationCodeRequired || violation.Message != "is required" {
		t.Fatalf("violation = %#v", violation)
	}
}

func requestWithPathParams(params map[string][]string) *http.Request {
	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	rctx := chi.NewRouteContext()
	for key, values := range params {
		for _, value := range values {
			rctx.URLParams.Add(key, value)
		}
	}
	return req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))
}
