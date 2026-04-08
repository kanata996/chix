package reqx

import (
	"errors"
	"net/http"
	"reflect"
	"testing"

	ut "github.com/go-playground/universal-translator"
	"github.com/go-playground/validator/v10"
)

type fakeFieldLevel struct {
	field reflect.Value
}

func (f fakeFieldLevel) Top() reflect.Value      { return reflect.Value{} }
func (f fakeFieldLevel) Parent() reflect.Value   { return reflect.Value{} }
func (f fakeFieldLevel) Field() reflect.Value    { return f.field }
func (f fakeFieldLevel) FieldName() string       { return "" }
func (f fakeFieldLevel) StructFieldName() string { return "" }
func (f fakeFieldLevel) Param() string           { return "" }
func (f fakeFieldLevel) GetTag() string          { return "" }
func (f fakeFieldLevel) ExtractType(v reflect.Value) (reflect.Value, reflect.Kind, bool) {
	return v, v.Kind(), false
}
func (f fakeFieldLevel) GetStructFieldOK() (reflect.Value, reflect.Kind, bool) {
	return reflect.Value{}, reflect.Invalid, false
}
func (f fakeFieldLevel) GetStructFieldOKAdvanced(reflect.Value, string) (reflect.Value, reflect.Kind, bool) {
	return reflect.Value{}, reflect.Invalid, false
}
func (f fakeFieldLevel) GetStructFieldOK2() (reflect.Value, reflect.Kind, bool, bool) {
	return reflect.Value{}, reflect.Invalid, false, false
}
func (f fakeFieldLevel) GetStructFieldOKAdvanced2(reflect.Value, string) (reflect.Value, reflect.Kind, bool, bool) {
	return reflect.Value{}, reflect.Invalid, false, false
}

type fakeFieldError struct {
	tag        string
	namespace  string
	structNS   string
	field      string
	structName string
	value      any
	param      string
	typ        reflect.Type
}

func (f fakeFieldError) Tag() string             { return f.tag }
func (f fakeFieldError) ActualTag() string       { return f.tag }
func (f fakeFieldError) Namespace() string       { return f.namespace }
func (f fakeFieldError) StructNamespace() string { return f.structNS }
func (f fakeFieldError) Field() string           { return f.field }
func (f fakeFieldError) StructField() string     { return f.structName }
func (f fakeFieldError) Value() interface{}      { return f.value }
func (f fakeFieldError) Param() string           { return f.param }
func (f fakeFieldError) Kind() reflect.Kind {
	if f.typ == nil {
		return reflect.Invalid
	}
	return f.typ.Kind()
}
func (f fakeFieldError) Type() reflect.Type             { return f.typ }
func (f fakeFieldError) Translate(ut.Translator) string { return f.Error() }
func (f fakeFieldError) Error() string                  { return "fake field error" }

func TestRequireBodyNilRequest(t *testing.T) {
	if err := RequireBody(nil); err == nil || err.Error() != "reqx: request must not be nil" {
		t.Fatalf("RequireBody(nil) error = %v", err)
	}
}

func TestBindAndValidateWrappersRejectNilDestination(t *testing.T) {
	req := newJSONRequest(http.MethodPost, "/", `{}`)

	if err := BindAndValidate(req, nil); err == nil || err.Error() != "reqx: destination must not be nil" {
		t.Fatalf("BindAndValidate(nil target) error = %v", err)
	}
	if err := BindAndValidateBody(req, nil); err == nil || err.Error() != "reqx: destination must not be nil" {
		t.Fatalf("BindAndValidateBody(nil target) error = %v", err)
	}
	if err := BindAndValidateQuery(req, nil); err == nil || err.Error() != "reqx: destination must not be nil" {
		t.Fatalf("BindAndValidateQuery(nil target) error = %v", err)
	}
	if err := BindAndValidatePath(req, nil); err == nil || err.Error() != "reqx: destination must not be nil" {
		t.Fatalf("BindAndValidatePath(nil target) error = %v", err)
	}
	if err := BindAndValidateHeaders(req, nil); err == nil || err.Error() != "reqx: destination must not be nil" {
		t.Fatalf("BindAndValidateHeaders(nil target) error = %v", err)
	}
}

func TestBindAndValidateWrappersReturnBindingErrorsFromBindPackage(t *testing.T) {
	type bodyRequest struct {
		Name int `json:"name"`
	}
	bodyReq := newJSONRequest(http.MethodPost, "/", `{"name":"oops"}`)
	if err := BindAndValidateBody(bodyReq, &bodyRequest{}); err == nil {
		t.Fatal("BindAndValidateBody(bind error) = nil")
	}

	type requestQuery struct {
		Page int `query:"page"`
	}
	queryReq := newJSONRequest(http.MethodGet, "/?page=oops", "")
	queryReq.ContentLength = 0
	if err := BindAndValidateQuery(queryReq, &requestQuery{}); err == nil {
		t.Fatal("BindAndValidateQuery(bind error) = nil")
	}

	type requestPath struct {
		ID int `param:"id"`
	}
	pathReq := requestWithPathParams(map[string][]string{"id": {"oops"}})
	if err := BindAndValidatePath(pathReq, &requestPath{}); err == nil {
		t.Fatal("BindAndValidatePath(bind error) = nil")
	}

	type requestHeader struct {
		Retry int `header:"x-retry"`
	}
	headerReq := newJSONRequest(http.MethodGet, "/", "")
	headerReq.ContentLength = 0
	headerReq.Header.Set("X-Retry", "oops")
	if err := BindAndValidateHeaders(headerReq, &requestHeader{}); err == nil {
		t.Fatal("BindAndValidateHeaders(bind error) = nil")
	}

	type mixedRequest struct {
		ID int `param:"id"`
	}
	mixedReq := requestWithPathParams(map[string][]string{"id": {"oops"}})
	if err := BindAndValidate(mixedReq, &mixedRequest{}); err == nil {
		t.Fatal("BindAndValidate(bind error) = nil")
	}
}

func TestReqxValidationHelperBranches(t *testing.T) {
	if !validateNoSpace(fakeFieldLevel{field: reflect.ValueOf("kanata")}) {
		t.Fatal("validateNoSpace(string without space) = false, want true")
	}
	if validateNoSpace(fakeFieldLevel{field: reflect.ValueOf("kana ta")}) {
		t.Fatal("validateNoSpace(string with space) = true, want false")
	}
	if validateNoSpace(fakeFieldLevel{field: reflect.ValueOf(1)}) {
		t.Fatal("validateNoSpace(non-string) = true, want false")
	}

	defer func() {
		if recover() == nil {
			t.Fatal("mustRegisterValidation() did not panic")
		}
	}()
	mustRegisterValidation(validator.New(), "", validateNoSpace)
}

func TestViolationsFromValidationAndFieldPathBranches(t *testing.T) {
	if got := violationsFromValidation(sourceBody, nil, nil); got != nil {
		t.Fatalf("violationsFromValidation(nil) = %#v, want nil", got)
	}

	errs := validator.ValidationErrors{
		fakeFieldError{tag: "required", namespace: "Req.z", field: "z", typ: reflect.TypeOf("")},
		fakeFieldError{tag: "min", namespace: "Req.a", field: "a", typ: reflect.TypeOf("")},
		fakeFieldError{tag: "required", namespace: "Req.a", field: "a", typ: reflect.TypeOf("")},
	}
	violations := violationsFromValidation(sourceRequest, nil, errs)
	if len(violations) != 2 {
		t.Fatalf("violations len = %d, want 2", len(violations))
	}
	if violations[0].Field != "a" || violations[0].Code != ViolationCodeInvalid {
		t.Fatalf("violations[0] = %#v", violations[0])
	}
	if violations[1].Field != "z" || violations[1].Code != ViolationCodeRequired {
		t.Fatalf("violations[1] = %#v", violations[1])
	}

	if got := validationFieldPath(sourceBody, fakeFieldError{namespace: " Req.body.name ", typ: reflect.TypeOf("")}); got != "body.name" {
		t.Fatalf("validationFieldPath(namespace) = %q, want body.name", got)
	}
	if got := validationFieldPath(sourceBody, fakeFieldError{field: "display_name", typ: reflect.TypeOf("")}); got != "display_name" {
		t.Fatalf("validationFieldPath(field) = %q, want display_name", got)
	}
	if got := validationFieldPath(sourceBody, fakeFieldError{}); got != "body" {
		t.Fatalf("validationFieldPath(body fallback) = %q, want body", got)
	}
	if got := validationFieldPath(sourceRequest, fakeFieldError{}); got != "request" {
		t.Fatalf("validationFieldPath(request fallback) = %q, want request", got)
	}
}

func TestTagValueAdditionalBranches(t *testing.T) {
	type request struct {
		NoTag    string
		BlankTag string `json:"   "`
		SkipTag  string `json:"-"`
	}

	noTagField, _ := reflect.TypeOf(request{}).FieldByName("NoTag")
	blankTagField, _ := reflect.TypeOf(request{}).FieldByName("BlankTag")
	skipTagField, _ := reflect.TypeOf(request{}).FieldByName("SkipTag")

	if got := tagValue(noTagField, "json"); got != "" {
		t.Fatalf("tagValue(no tag) = %q, want empty", got)
	}
	if got := tagValue(blankTagField, "json"); got != "" {
		t.Fatalf("tagValue(blank tag) = %q, want empty", got)
	}
	if got := tagValue(skipTagField, "json"); got != "" {
		t.Fatalf("tagValue(skip tag) = %q, want empty", got)
	}
}

func TestPostBindValidateRejectsInvalidTarget(t *testing.T) {
	if err := postBindValidate(newJSONRequest(http.MethodPost, "/", `{}`), 1, sourceBody); err == nil || err.Error() != "reqx: target must be a non-nil pointer to struct" {
		t.Fatalf("postBindValidate(non-struct) error = %v", err)
	}
}

func TestApplyRequestValidationNoValidator(t *testing.T) {
	if err := applyRequestValidation(newJSONRequest(http.MethodGet, "/", ""), struct{}{}); err != nil {
		t.Fatalf("applyRequestValidation(no validator) error = %v", err)
	}
}

func TestValidateStructInvalidValidationErrorExtra(t *testing.T) {
	_, err := validateStruct(1, sourceBody)
	var invalidValidationErr *validator.InvalidValidationError
	if !errors.As(err, &invalidValidationErr) {
		t.Fatalf("validateStruct() error = %T, want *validator.InvalidValidationError", err)
	}
}
