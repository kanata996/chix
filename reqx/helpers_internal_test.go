package reqx

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"reflect"
	"strings"
	"testing"

	ut "github.com/go-playground/universal-translator"
	"github.com/go-playground/validator/v10"
)

type failingReadCloser struct {
	err error
}

func (r failingReadCloser) Read(_ []byte) (int, error) {
	return 0, r.err
}

func (r failingReadCloser) Close() error {
	return nil
}

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
	actualTag  string
	namespace  string
	structNS   string
	field      string
	structName string
	value      any
	param      string
	typ        reflect.Type
}

func (f fakeFieldError) Tag() string             { return f.tag }
func (f fakeFieldError) ActualTag() string       { return f.actualTag }
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

func TestBindAndBindBodyRejectNilDestination(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/", nil)

	var dst struct{}
	if err := Bind[struct{}](nil, &dst); err == nil || err.Error() != "reqx: request must not be nil" {
		t.Fatalf("Bind(nil) error = %v, want request must not be nil", err)
	}

	if err := Bind[struct{}](req, nil); err == nil || err.Error() != "reqx: destination must not be nil" {
		t.Fatalf("Bind() error = %v, want destination must not be nil", err)
	}

	req = newJSONRequest(http.MethodPost, "/", `{"name":"kanata"}`)
	if err := BindBody[struct{}](req, nil); err == nil || err.Error() != "reqx: destination must not be nil" {
		t.Fatalf("BindBody() error = %v, want destination must not be nil", err)
	}
}

func TestBindTaggedValuesRejectsNonStructDestination(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	value := 1

	err := bindTaggedValues(req, &value, querySource, bindValuesConfig{})
	if err == nil || err.Error() != "reqx: destination must point to a struct" {
		t.Fatalf("bindTaggedValues() error = %v, want destination must point to a struct", err)
	}
}

func TestReadBodyBranches(t *testing.T) {
	data, err := readBody(nil, 10)
	if err != nil || data != nil {
		t.Fatalf("readBody(nil) = (%v, %v), want (nil, nil)", data, err)
	}

	wantErr := errors.New("read failed")
	_, err = readBody(failingReadCloser{err: wantErr}, 10)
	if !errors.Is(err, wantErr) {
		t.Fatalf("readBody() error = %v, want %v", err, wantErr)
	}
}

func TestBindJSONWithConfigRejectsEmptyBodyWhenNotAllowed(t *testing.T) {
	req := newJSONRequest(http.MethodPost, "/", "")

	var dst struct {
		Name string `json:"name"`
	}
	err := bindJSONWithConfig(req, &dst, bindBodyConfig{}, bodyBindMode{})
	_ = assertHTTPError(t, err, http.StatusBadRequest, CodeInvalidJSON, "request body must not be empty")
}

func TestBindJSONWithConfigRejectsInvalidContentTypeOnEmptyBody(t *testing.T) {
	req := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(""))
	req.Header.Set("Content-Type", "text/plain")

	var dst struct {
		Name string `json:"name"`
	}
	err := bindJSONWithConfig(req, &dst, bindBodyConfig{allowEmptyBody: true}, bodyBindMode{
		validateContentTypeOnEmpty: true,
	})
	_ = assertHTTPError(t, err, http.StatusUnsupportedMediaType, CodeUnsupportedMediaType, "Content-Type must be application/json")
}

func TestBindJSONWithConfigPropagatesReadError(t *testing.T) {
	wantErr := errors.New("read failed")
	req := httptest.NewRequest(http.MethodPost, "/", nil)
	req.Header.Set("Content-Type", "application/json")
	req.Body = failingReadCloser{err: wantErr}

	var dst struct {
		Name string `json:"name"`
	}
	err := bindJSONWithConfig(req, &dst, bindBodyConfig{}, bodyBindMode{})
	if !errors.Is(err, wantErr) {
		t.Fatalf("bindJSONWithConfig() error = %v, want %v", err, wantErr)
	}
}

func TestBindJSONWithConfigRejectsUnknownFieldWhenDisabled(t *testing.T) {
	req := newJSONRequest(http.MethodPost, "/", `{"name":"kanata","extra":1}`)

	var dst struct {
		Name string `json:"name"`
	}
	err := bindJSONWithConfig(req, &dst, bindBodyConfig{allowUnknownFields: false}, bodyBindMode{})
	violation := assertSingleViolation(t, err)
	if violation.Field != "extra" || violation.Code != ViolationCodeUnknown || violation.Message != "unknown field" {
		t.Fatalf("violation = %#v", violation)
	}
}

func TestValidateJSONContentTypeAllowsJSONSuffix(t *testing.T) {
	if err := validateJSONContentType("application/merge-patch+json"); err != nil {
		t.Fatalf("validateJSONContentType() error = %v", err)
	}
	if err := validateJSONContentType(""); err != nil {
		t.Fatalf("validateJSONContentType(empty) error = %v", err)
	}
}

func TestMapDecodeErrorBranches(t *testing.T) {
	t.Run("syntax", func(t *testing.T) {
		err := mapDecodeError(&json.SyntaxError{})
		_ = assertHTTPError(t, err, http.StatusBadRequest, CodeInvalidJSON, "request body must be valid JSON")
	})

	t.Run("type", func(t *testing.T) {
		err := mapDecodeError(&json.UnmarshalTypeError{
			Field: "age",
			Type:  reflect.TypeOf(0),
		})
		violation := assertSingleViolation(t, err)
		if violation.Field != "age" || violation.Code != ViolationCodeType || violation.Message != "must be number" {
			t.Fatalf("violation = %#v", violation)
		}
	})

	t.Run("invalid unmarshal", func(t *testing.T) {
		wantErr := &json.InvalidUnmarshalError{Type: reflect.TypeOf(0)}
		if got := mapDecodeError(wantErr); got != wantErr {
			t.Fatalf("mapDecodeError() = %v, want same error", got)
		}
	})

	t.Run("eof", func(t *testing.T) {
		err := mapDecodeError(io.EOF)
		_ = assertHTTPError(t, err, http.StatusBadRequest, CodeInvalidJSON, "request body must not be empty")
	})

	t.Run("unknown field", func(t *testing.T) {
		err := mapDecodeError(errors.New(`json: unknown field "extra"`))
		violation := assertSingleViolation(t, err)
		if violation.Field != "extra" || violation.Code != ViolationCodeUnknown || violation.Message != "unknown field" {
			t.Fatalf("violation = %#v", violation)
		}
	})

	t.Run("default", func(t *testing.T) {
		err := mapDecodeError(errors.New("boom"))
		_ = assertHTTPError(t, err, http.StatusBadRequest, CodeInvalidJSON, "request body must be valid JSON")
	})
}

func TestParseUnknownField(t *testing.T) {
	if field, ok := parseUnknownField(errors.New(`json: unknown field "extra"`)); !ok || field != "extra" {
		t.Fatalf("parseUnknownField() = (%q, %v), want (extra, true)", field, ok)
	}
	if field, ok := parseUnknownField(errors.New("boom")); ok || field != "" {
		t.Fatalf("parseUnknownField() = (%q, %v), want empty false", field, ok)
	}
}

func TestDescribeJSONType(t *testing.T) {
	testCases := []struct {
		name string
		typ  reflect.Type
		want string
	}{
		{name: "nil", typ: nil, want: "valid value"},
		{name: "bool", typ: reflect.TypeOf(true), want: "boolean"},
		{name: "int", typ: reflect.TypeOf(1), want: "number"},
		{name: "pointer int", typ: reflect.TypeOf(new(int)), want: "number"},
		{name: "uint", typ: reflect.TypeOf(uint(1)), want: "number"},
		{name: "float", typ: reflect.TypeOf(1.5), want: "number"},
		{name: "string", typ: reflect.TypeOf(""), want: "string"},
		{name: "array", typ: reflect.TypeOf([2]string{}), want: "array"},
		{name: "slice", typ: reflect.TypeOf([]string{}), want: "array"},
		{name: "map", typ: reflect.TypeOf(map[string]string{}), want: "object"},
		{name: "struct", typ: reflect.TypeOf(struct{}{}), want: "object"},
		{name: "default", typ: reflect.TypeOf(complex64(1)), want: "complex64"},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			if got := describeJSONType(tc.typ); got != tc.want {
				t.Fatalf("describeJSONType() = %q, want %q", got, tc.want)
			}
		})
	}
}

func TestEmptyBodyError(t *testing.T) {
	err := emptyBodyError()
	_ = assertHTTPError(t, err, http.StatusBadRequest, CodeInvalidJSON, "request body must not be empty")
}

func TestPathValuesBranches(t *testing.T) {
	if got := pathValues(nil); len(got) != 0 {
		t.Fatalf("pathValues(nil) = %#v, want empty", got)
	}

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	if got := pathValues(req); len(got) != 0 {
		t.Fatalf("pathValues(no route ctx) = %#v, want empty", got)
	}
}

func TestRequiredPathParamValuesRejectsSingleEmptyValue(t *testing.T) {
	req := requestWithPathParams(map[string][]string{
		"id": {"   "},
	})

	_, err := requiredPathParamValues(req, "id")
	violation := assertSingleViolation(t, err)
	if violation.Field != "id" || violation.Code != ViolationCodeRequired || violation.Message != "is required" {
		t.Fatalf("violation = %#v", violation)
	}
}

func TestLoadValueDecodePlanUsesCache(t *testing.T) {
	type request struct {
		Page int `query:"page"`
	}

	plan1, err := loadValueDecodePlan(reflect.TypeOf(request{}), querySource)
	if err != nil {
		t.Fatalf("loadValueDecodePlan() error = %v", err)
	}
	plan2, err := loadValueDecodePlan(reflect.TypeOf(request{}), querySource)
	if err != nil {
		t.Fatalf("loadValueDecodePlan() second error = %v", err)
	}
	if plan1 != plan2 {
		t.Fatal("loadValueDecodePlan() did not reuse cached plan")
	}
}

func TestValueFieldNameAndTagValue(t *testing.T) {
	type request struct {
		Page int `query:" page ,omitempty "`
		Skip int `query:"-"`
	}

	pageField, _ := reflect.TypeOf(request{}).FieldByName("Page")
	skipField, _ := reflect.TypeOf(request{}).FieldByName("Skip")

	if name, ok := valueFieldName(pageField, querySource); !ok || name != "page" {
		t.Fatalf("valueFieldName(page) = (%q, %v), want (page, true)", name, ok)
	}
	if name, ok := valueFieldName(skipField, querySource); ok || name != "" {
		t.Fatalf("valueFieldName(skip) = (%q, %v), want empty false", name, ok)
	}

	if got := tagValue(pageField, "query"); got != "page" {
		t.Fatalf("tagValue() = %q, want page", got)
	}
	if got := tagValue(skipField, "query"); got != "" {
		t.Fatalf("tagValue(skip) = %q, want empty", got)
	}
}

func TestValidateValueFieldTypeAndSupportsTextUnmarshaler(t *testing.T) {
	if err := validateValueFieldType(reflect.TypeOf(new(int)), "Age", "query"); err != nil {
		t.Fatalf("validateValueFieldType(pointer) error = %v", err)
	}
	if err := validateValueFieldType(reflect.TypeOf([]uint{}), "IDs", "query"); err != nil {
		t.Fatalf("validateValueFieldType(slice) error = %v", err)
	}
	if !supportsTextUnmarshaler(reflect.TypeOf(queryState(""))) {
		t.Fatal("supportsTextUnmarshaler(queryState) = false, want true")
	}
	if !supportsTextUnmarshaler(reflect.TypeOf((*queryState)(nil))) {
		t.Fatal("supportsTextUnmarshaler(*queryState) = false, want true")
	}
	if supportsTextUnmarshaler(reflect.TypeOf("")) {
		t.Fatal("supportsTextUnmarshaler(string) = true, want false")
	}
	if supportsTextUnmarshaler(reflect.TypeOf((*string)(nil))) {
		t.Fatal("supportsTextUnmarshaler(*string) = true, want false")
	}
}

func TestDecodeValuesIntoAndDecodeQueryFieldBranches(t *testing.T) {
	type request struct {
		Page int `query:"page"`
	}

	dst := reflect.ValueOf(&request{}).Elem()
	plan := &valueDecodePlan{
		fields:      []valueFieldSpec{{index: 0, name: "page"}},
		knownFields: map[string]struct{}{"page": {}},
	}

	violations, err := decodeValuesInto(dst, url.Values{"extra": {"1"}}, plan, bindValuesConfig{})
	if err != nil {
		t.Fatalf("decodeValuesInto() error = %v", err)
	}
	if len(violations) != 1 || violations[0].Field != "extra" || violations[0].Code != ViolationCodeUnknown {
		t.Fatalf("violations = %#v", violations)
	}

	violations, err = decodeValuesInto(dst, url.Values{"extra": {"1"}}, plan, bindValuesConfig{allowUnknownFields: true})
	if err != nil {
		t.Fatalf("decodeValuesInto(allow unknown) error = %v", err)
	}
	if len(violations) != 0 {
		t.Fatalf("violations = %#v, want empty", violations)
	}

	var value string
	violation, err := decodeQueryField(reflect.ValueOf(&value).Elem(), nil, "name")
	if err != nil || violation != nil {
		t.Fatalf("decodeQueryField(empty) = (%v, %v), want (nil, nil)", violation, err)
	}

	var state queryState
	violation, err = decodeQueryField(reflect.ValueOf(&state).Elem(), []string{"active", "disabled"}, "state")
	if err != nil {
		t.Fatalf("decodeQueryField(text repeated) error = %v", err)
	}
	if violation == nil || violation.Code != ViolationCodeMultiple {
		t.Fatalf("violation = %#v, want multiple", violation)
	}

	var ids []int
	violation, err = decodeQueryField(reflect.ValueOf(&ids).Elem(), []string{"1", "oops"}, "id")
	if err != nil {
		t.Fatalf("decodeQueryField(slice item) error = %v", err)
	}
	if violation == nil || violation.Field != "id" || violation.Code != ViolationCodeType {
		t.Fatalf("violation = %#v, want type violation for id", violation)
	}

	var page *int
	violation, err = decodeQueryField(reflect.ValueOf(&page).Elem(), []string{"oops"}, "page")
	if err != nil {
		t.Fatalf("decodeQueryField(pointer invalid) error = %v", err)
	}
	if violation == nil || violation.Field != "page" || violation.Code != ViolationCodeType {
		t.Fatalf("violation = %#v, want type violation for page", violation)
	}
}

func TestDecodeScalarValueBranches(t *testing.T) {
	var boolValue bool
	if violation, err := decodeScalarValue(reflect.ValueOf(&boolValue).Elem(), "maybe", "enabled"); err != nil {
		t.Fatalf("decodeScalarValue(invalid bool) error = %v", err)
	} else if violation == nil || violation.Field != "enabled" || violation.Code != ViolationCodeType {
		t.Fatalf("decodeScalarValue(invalid bool) violation = %#v", violation)
	}

	var uintValue uint
	if violation, err := decodeScalarValue(reflect.ValueOf(&uintValue).Elem(), "42", "id"); err != nil || violation != nil || uintValue != 42 {
		t.Fatalf("decodeScalarValue(uint) = (%v, %v), value=%d", violation, err, uintValue)
	}

	if violation, err := decodeScalarValue(reflect.ValueOf(&uintValue).Elem(), "-1", "id"); err != nil {
		t.Fatalf("decodeScalarValue(invalid uint) error = %v", err)
	} else if violation == nil || violation.Field != "id" || violation.Code != ViolationCodeType {
		t.Fatalf("decodeScalarValue(invalid uint) violation = %#v", violation)
	}

	var score float64
	if violation, err := decodeScalarValue(reflect.ValueOf(&score).Elem(), "oops", "score"); err != nil {
		t.Fatalf("decodeScalarValue(invalid float) error = %v", err)
	} else if violation == nil || violation.Field != "score" || violation.Code != ViolationCodeType {
		t.Fatalf("decodeScalarValue(invalid float) violation = %#v", violation)
	}

	var unsupported struct{}
	_, err := decodeScalarValue(reflect.ValueOf(&unsupported).Elem(), "x", "field")
	if err == nil || !strings.Contains(err.Error(), "unsupported destination type") {
		t.Fatalf("decodeScalarValue(unsupported) error = %v", err)
	}
}

func TestValidateNoSpace(t *testing.T) {
	if !validateNoSpace(fakeFieldLevel{field: reflect.ValueOf("kanata")}) {
		t.Fatal("validateNoSpace(string without space) = false, want true")
	}
	if validateNoSpace(fakeFieldLevel{field: reflect.ValueOf("kana ta")}) {
		t.Fatal("validateNoSpace(string with space) = true, want false")
	}
	if validateNoSpace(fakeFieldLevel{field: reflect.ValueOf(1)}) {
		t.Fatal("validateNoSpace(non-string) = true, want false")
	}
}

func TestValidateTargetAndErrorsf(t *testing.T) {
	if err := validateTarget(nil); err == nil || err.Error() != "reqx: target must not be nil" {
		t.Fatalf("validateTarget(nil) error = %v", err)
	}

	var nilStruct *struct{}
	if err := validateTarget(nilStruct); err == nil || err.Error() != "reqx: target must be a non-nil pointer to struct" {
		t.Fatalf("validateTarget(nil struct ptr) error = %v", err)
	}
	if err := validateTarget(struct{}{}); err == nil || err.Error() != "reqx: target must be a non-nil pointer to struct" {
		t.Fatalf("validateTarget(non-pointer) error = %v", err)
	}
	if err := errorsf("boom %d", 1); err.Error() != "reqx: boom 1" {
		t.Fatalf("errorsf() = %v", err)
	}
}

func TestValidateStructInvalidValidationError(t *testing.T) {
	_, err := validateStruct(1, sourceBody)
	var invalidValidationErr *validator.InvalidValidationError
	if !errors.As(err, &invalidValidationErr) {
		t.Fatalf("validateStruct() error = %T, want *validator.InvalidValidationError", err)
	}
}

func TestMustRegisterValidationPanicsOnInvalidRegistration(t *testing.T) {
	defer func() {
		if recover() == nil {
			t.Fatal("mustRegisterValidation() did not panic")
		}
	}()

	mustRegisterValidation(validator.New(), "", validateNoSpace)
}

func TestValidationHelpers(t *testing.T) {
	if got := violationsFromValidation(sourceBody, nil); got != nil {
		t.Fatalf("violationsFromValidation(nil) = %#v, want nil", got)
	}

	errs := validator.ValidationErrors{
		fakeFieldError{tag: "required", namespace: "Req.z", field: "z", typ: reflect.TypeOf("")},
		fakeFieldError{tag: "min", namespace: "Req.a", field: "a", typ: reflect.TypeOf("")},
		fakeFieldError{tag: "required", namespace: "Req.a", field: "a", typ: reflect.TypeOf("")},
	}
	violations := violationsFromValidation(sourceRequest, errs)
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

func TestFieldAlias(t *testing.T) {
	type request struct {
		RequestID string `header:"x-request-id"`
		Name      string
	}

	requestIDField, _ := reflect.TypeOf(request{}).FieldByName("RequestID")
	nameField, _ := reflect.TypeOf(request{}).FieldByName("Name")

	if got := fieldAlias(requestIDField, sourceHeader); got != "X-Request-Id" {
		t.Fatalf("fieldAlias(header) = %q, want X-Request-Id", got)
	}
	if got := fieldAlias(nameField, sourceBody); got != "Name" {
		t.Fatalf("fieldAlias(fallback) = %q, want Name", got)
	}
}
