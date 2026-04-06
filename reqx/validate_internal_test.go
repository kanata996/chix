package reqx

// 用例清单：
// - 标记说明：[✓] 已核对且已有真实覆盖；[x] 本轮审查发现缺口后补测。
// - [✓] `BindAndValidate*` 包装器的成功路径与绑定错误优先级。
// - [✓] `validate`、`validateStruct`、`validateTarget` 的边界分支与无效目标错误。
// - [✓] violation 规范化、字段解析、来源推断与标签优先级辅助分支。
// - [✓] 这组测试覆盖内部校验辅助逻辑，断言具体返回值和 panic 分支，不是假测试。

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"reflect"
	"testing"
	"time"

	"github.com/go-playground/validator/v10"
)

// 各个 BindAndValidate 包装器会优先返回绑定阶段错误。
func TestBindAndValidateWrappersReturnBindErrors(t *testing.T) {
	var dst struct{}

	if err := BindAndValidate[struct{}](nil, &dst); err == nil || err.Error() != "reqx: request must not be nil" {
		t.Fatalf("BindAndValidate() error = %v", err)
	}
	if err := BindAndValidateBody[struct{}](nil, &dst); err == nil || err.Error() != "reqx: request must not be nil" {
		t.Fatalf("BindAndValidateBody() error = %v", err)
	}
	if err := BindAndValidateQuery[struct{}](nil, &dst); err == nil || err.Error() != "reqx: request must not be nil" {
		t.Fatalf("BindAndValidateQuery() error = %v", err)
	}
	if err := BindAndValidatePath[struct{}](nil, &dst); err == nil || err.Error() != "reqx: request must not be nil" {
		t.Fatalf("BindAndValidatePath() error = %v", err)
	}
	if err := BindAndValidateHeaders[struct{}](nil, &dst); err == nil || err.Error() != "reqx: request must not be nil" {
		t.Fatalf("BindAndValidateHeaders() error = %v", err)
	}
}

// 各个 BindAndValidate 包装器在正常输入下都能顺利通过。
func TestBindAndValidateWrappersSuccessPaths(t *testing.T) {
	type bodyRequest struct {
		Name string `json:"name" validate:"required"`
	}
	bodyReq := newJSONRequest(http.MethodPost, "/", `{"name":"kanata"}`)
	var bodyDst bodyRequest
	if err := BindAndValidateBody(bodyReq, &bodyDst); err != nil {
		t.Fatalf("BindAndValidateBody() error = %v", err)
	}

	type requestRequest struct {
		ID string `param:"id" validate:"required"`
	}
	req := requestWithPathParams(map[string][]string{"id": {"route-id"}})
	req.Method = http.MethodGet
	req.URL.RawQuery = "ignored=1"
	var requestDst requestRequest
	if err := BindAndValidate(req, &requestDst); err != nil {
		t.Fatalf("BindAndValidate() error = %v", err)
	}

	type queryRequest struct {
		Cursor string `query:"cursor" validate:"required"`
	}
	queryReq := httptest.NewRequest(http.MethodGet, "/?cursor=abc", nil)
	var queryDst queryRequest
	if err := BindAndValidateQuery(queryReq, &queryDst); err != nil {
		t.Fatalf("BindAndValidateQuery() error = %v", err)
	}

	type pathRequest struct {
		UUID string `param:"uuid" validate:"required"`
	}
	pathReq := requestWithPathParams(map[string][]string{"uuid": {"u_1"}})
	var pathDst pathRequest
	if err := BindAndValidatePath(pathReq, &pathDst); err != nil {
		t.Fatalf("BindAndValidatePath() error = %v", err)
	}

	type headerRequest struct {
		RequestID string `header:"x-request-id" validate:"required"`
	}
	headerReq := httptest.NewRequest(http.MethodGet, "/", nil)
	headerReq.Header.Set("X-Request-Id", "req-1")
	var headerDst headerRequest
	if err := BindAndValidateHeaders(headerReq, &headerDst); err != nil {
		t.Fatalf("BindAndValidateHeaders() error = %v", err)
	}
}

// validate 会覆盖成功、typed nil 和非结构体目标分支。
func TestValidateBranches(t *testing.T) {
	type request struct {
		Name string `validate:"required"`
	}

	if err := validate(&request{Name: "ok"}, sourceBody); err != nil {
		t.Fatalf("validate(success) error = %v", err)
	}

	var nilTarget *request
	if err := validate(nilTarget, sourceBody); err == nil || err.Error() != "reqx: target must be a non-nil pointer to struct" {
		t.Fatalf("validate(typed nil) error = %v", err)
	}

	value := 1
	if err := validate(&value, sourceBody); err == nil || err.Error() != "reqx: target must be a non-nil pointer to struct" {
		t.Fatalf("validate(non-struct) error = %v", err)
	}
}

// validateStruct 返回的校验错误会被转换为 violation 列表。
func TestValidateStructValidationErrors(t *testing.T) {
	target := &struct {
		Name string `json:"name" validate:"required"`
	}{}

	violations, err := validateStruct(target, sourceBody)
	if err != nil {
		t.Fatalf("validateStruct() error = %v", err)
	}
	if len(violations) != 1 {
		t.Fatalf("violations len = %d, want 1", len(violations))
	}
	if violations[0].Field != "name" || violations[0].In != ViolationInBody || violations[0].Code != ViolationCodeRequired || violations[0].Detail != "is required" {
		t.Fatalf("violations[0] = %#v", violations[0])
	}
}

// 直接传入 nil 接口值时返回空目标错误。
func TestValidateTargetRejectsNilTarget(t *testing.T) {
	err := validateTarget(nil)
	if err == nil {
		t.Fatal("validateTarget() error = nil")
	}
	if got := err.Error(); got != "reqx: target must not be nil" {
		t.Fatalf("error = %q", got)
	}
}

// 非法的校验目标会透传 validator 的 InvalidValidationError。
func TestValidateStructReturnsInvalidValidationError(t *testing.T) {
	_, err := validateStruct(1, sourceBody)
	if err == nil {
		t.Fatal("validateStruct() error = nil")
	}

	var invalidErr *validator.InvalidValidationError
	if !errors.As(err, &invalidErr) {
		t.Fatalf("error = %T, want *validator.InvalidValidationError", err)
	}
}

// validator 拒绝 time.Time 时，validate 会直接透传该错误。
func TestValidateReturnsInvalidValidationError(t *testing.T) {
	now := time.Now()

	err := validate(&now, sourceBody)
	if err == nil {
		t.Fatal("validate() error = nil")
	}

	var invalidErr *validator.InvalidValidationError
	if !errors.As(err, &invalidErr) {
		t.Fatalf("error = %T, want *validator.InvalidValidationError", err)
	}
}

// 内部校验器和标签优先级 helper 对不支持的来源会 panic。
func TestValidatorHelpers_PanicOnUnsupportedSource(t *testing.T) {
	t.Run("validatorFor", func(t *testing.T) {
		defer func() {
			if recover() == nil {
				t.Fatal("validatorFor() did not panic")
			}
		}()

		_ = validatorFor(sourceKind("unsupported"))
	})

	t.Run("sourceTagPriority", func(t *testing.T) {
		defer func() {
			if recover() == nil {
				t.Fatal("sourceTagPriority() did not panic")
			}
		}()

		_ = sourceTagPriority(sourceKind("unsupported"))
	})
}

// body 来源的标签优先级顺序固定，用于字段别名解析。
func TestSourceTagPriority_UsesBodyPriority(t *testing.T) {
	got := sourceTagPriority(sourceBody)
	want := []string{"json", "query", "param", "header"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("sourceTagPriority(sourceBody) = %#v, want %#v", got, want)
	}
}

// normalizeViolation 会按错误码补齐默认错误信息。
func TestNormalizeViolationBranches(t *testing.T) {
	testCases := []struct {
		name string
		in   Violation
		want Violation
	}{
		{
			name: "required",
			in:   Violation{Field: "name", Code: ViolationCodeRequired},
			want: Violation{Field: "name", Code: ViolationCodeRequired, Detail: "is required"},
		},
		{
			name: "unknown",
			in:   Violation{Field: "name", Code: ViolationCodeUnknown},
			want: Violation{Field: "name", Code: ViolationCodeUnknown, Detail: "unknown field"},
		},
		{
			name: "type",
			in:   Violation{Field: "name", Code: ViolationCodeType},
			want: Violation{Field: "name", Code: ViolationCodeType, Detail: "has invalid type"},
		},
		{
			name: "multiple",
			in:   Violation{Field: "name", Code: ViolationCodeMultiple},
			want: Violation{Field: "name", Code: ViolationCodeMultiple, Detail: "must not be repeated"},
		},
		{
			name: "default",
			in:   Violation{Field: "name"},
			want: Violation{Field: "name", Code: ViolationCodeInvalid, Detail: "is invalid"},
		},
		{
			name: "explicit detail",
			in:   Violation{Field: "name", Code: ViolationCodeInvalid, Detail: "custom"},
			want: Violation{Field: "name", Code: ViolationCodeInvalid, Detail: "custom"},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			if got := normalizeViolation(tc.in); got != tc.want {
				t.Fatalf("normalizeViolation() = %#v, want %#v", got, tc.want)
			}
		})
	}
}

func TestViolationInputHelpers(t *testing.T) {
	if got := violationInForValueSource(valueSource{tag: querySource.tag}); got != ViolationInQuery {
		t.Fatalf("violationInForValueSource(query) = %q", got)
	}
	if got := violationInForValueSource(valueSource{tag: pathSource.tag}); got != ViolationInPath {
		t.Fatalf("violationInForValueSource(path) = %q", got)
	}
	if got := violationInForValueSource(valueSource{tag: headerSource.tag}); got != ViolationInHeader {
		t.Fatalf("violationInForValueSource(header) = %q", got)
	}
	if got := violationInForValueSource(valueSource{tag: "unknown"}); got != ViolationInRequest {
		t.Fatalf("violationInForValueSource(default) = %q", got)
	}

	testTags := map[string]string{
		"json":    ViolationInBody,
		"query":   ViolationInQuery,
		"param":   ViolationInPath,
		"header":  ViolationInHeader,
		"unknown": ViolationInRequest,
	}
	for tag, want := range testTags {
		if got := violationInForTag(tag); got != want {
			t.Fatalf("violationInForTag(%q) = %q, want %q", tag, got, want)
		}
	}

	testSources := map[sourceKind]string{
		sourceBody:      ViolationInBody,
		sourceQuery:     ViolationInQuery,
		sourcePath:      ViolationInPath,
		sourceHeader:    ViolationInHeader,
		sourceRequest:   ViolationInRequest,
		sourceKind("x"): ViolationInRequest,
	}
	for source, want := range testSources {
		if got := violationInForSource(source); got != want {
			t.Fatalf("violationInForSource(%q) = %q, want %q", source, got, want)
		}
	}
}

func TestResolveValidationFieldAndNamespaceParsing(t *testing.T) {
	type nestedItem struct {
		Value string `header:"x-value"`
	}
	type request struct {
		Nested []*nestedItem `json:"nested"`
		Plain  string        `json:"plain"`
	}

	field, ok := resolveValidationField(&request{}, "request.Nested[0].Value")
	if !ok {
		t.Fatal("resolveValidationField() = false, want true")
	}
	if field.Name != "Value" {
		t.Fatalf("field.Name = %q, want Value", field.Name)
	}

	testCases := []struct {
		name      string
		target    any
		namespace string
	}{
		{name: "nil target", target: nil, namespace: "request.Name"},
		{name: "empty namespace", target: &request{}, namespace: ""},
		{name: "root only", target: &request{}, namespace: "request"},
		{name: "missing field", target: &request{}, namespace: "request.Missing"},
		{name: "missing intermediate field", target: &request{}, namespace: "request.Nested[0].Missing.Value"},
		{name: "non-struct intermediate", target: &request{}, namespace: "request.Plain.Value"},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			if _, ok := resolveValidationField(tc.target, tc.namespace); ok {
				t.Fatalf("resolveValidationField(%#v, %q) = true, want false", tc.target, tc.namespace)
			}
		})
	}

	if got := parseStructNamespace(" request.Nested[0].Value "); !reflect.DeepEqual(got, []string{"Nested", "Value"}) {
		t.Fatalf("parseStructNamespace() = %#v", got)
	}
	if got := parseStructNamespace(""); got != nil {
		t.Fatalf("parseStructNamespace(empty) = %#v, want nil", got)
	}
	if got := parseStructNamespace("request"); got != nil {
		t.Fatalf("parseStructNamespace(root) = %#v, want nil", got)
	}
	if got := parseStructNamespace("request..Nested[0]..Value"); !reflect.DeepEqual(got, []string{"Nested", "Value"}) {
		t.Fatalf("parseStructNamespace(skip empty) = %#v", got)
	}
}

func TestValidationInputForRequestUsesTagPriorityAndFallback(t *testing.T) {
	type request struct {
		ID      string `param:"id" validate:"required"`
		Cursor  string `query:"cursor" validate:"required"`
		Name    string `json:"name" validate:"required"`
		TraceID string `header:"x-trace-id" validate:"required"`
		Plain   string `validate:"required"`
	}

	target := &request{}
	err := validatorFor(sourceRequest).Struct(target)
	if err == nil {
		t.Fatal("validatorFor(sourceRequest).Struct() error = nil")
	}

	validationErrs := err.(validator.ValidationErrors)
	violations := violationsFromValidation(sourceRequest, target, validationErrs)
	got := map[string]string{}
	for _, violation := range violations {
		got[violation.Field] = violation.In
	}

	want := map[string]string{
		"id":         ViolationInPath,
		"cursor":     ViolationInQuery,
		"name":       ViolationInBody,
		"X-Trace-Id": ViolationInHeader,
		"Plain":      ViolationInRequest,
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("request violations = %#v, want %#v", got, want)
	}

	if got := validationInput(sourceRequest, 1, validationErrs[0]); got != ViolationInRequest {
		t.Fatalf("validationInput(fallback) = %q, want request", got)
	}
}
