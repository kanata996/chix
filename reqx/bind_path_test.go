package reqx

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/kanata996/chix/resp"
)

// 绑定后会先裁剪 path 参数再做校验。
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

// 请求对象不能为空。
func TestBindPathValues_RequestMustNotBeNil(t *testing.T) {
	var dst struct{}

	err := BindPathValues[struct{}](nil, &dst)
	if err == nil {
		t.Fatal("BindPathValues() error = nil")
	}
	if got := err.Error(); got != "reqx: request must not be nil" {
		t.Fatalf("error = %q", got)
	}
}

// 目标对象不能为空。
func TestBindPathValues_DestinationMustNotBeNil(t *testing.T) {
	req := requestWithPathParams(map[string][]string{
		"id": {"1"},
	})

	err := BindPathValues[struct{}](req, nil)
	if err == nil {
		t.Fatal("BindPathValues() error = nil")
	}
	if got := err.Error(); got != "reqx: destination must not be nil" {
		t.Fatalf("error = %q", got)
	}
}

// 默认配置下，未知路径参数会被忽略。
func TestBindPathValues_IgnoresUnknownFieldsByDefault(t *testing.T) {
	type pathRequest struct {
		ID string `param:"id"`
	}

	req := requestWithPathParams(map[string][]string{
		"id":    {"42"},
		"extra": {"ignored"},
	})

	var path pathRequest
	if err := BindPathValues(req, &path); err != nil {
		t.Fatalf("BindPathValues() error = %v", err)
	}
	if path.ID != "42" {
		t.Fatalf("BindPathValues() id = %q", path.ID)
	}
}

// 缺失路径参数时，绑定阶段保持字段零值。
func TestBindPathValues_MissingTaggedParamLeavesZeroValue(t *testing.T) {
	type pathRequest struct {
		ID int `param:"id"`
	}

	req := requestWithPathParams(map[string][]string{
		"other": {"1"},
	})

	var path pathRequest
	if err := BindPathValues(req, &path); err != nil {
		t.Fatalf("BindPathValues() error = %v", err)
	}
	if path.ID != 0 {
		t.Fatalf("BindPathValues() id = %d, want 0", path.ID)
	}
}

// path 参数类型不匹配时返回类型错误。
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

// 标量 path 参数重复出现时返回重复值错误。
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

// 不支持的 path 字段类型会在构建解码计划时被拒绝。
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

// 未导出的 path 字段不允许参与绑定。
func TestBindPathValues_RejectsUnexportedTaggedField(t *testing.T) {
	type pathRequest struct {
		id string `param:"id"`
	}

	req := requestWithPathParams(map[string][]string{
		"id": {"1"},
	})

	var path pathRequest
	_ = path.id
	err := BindPathValues(req, &path)
	if err == nil {
		t.Fatal("BindPathValues() error = nil")
	}
	if got := err.Error(); got != `reqx: path field "id" must be exported` {
		t.Fatalf("error = %q", got)
	}
}

// 重复声明同名 path 标签会返回重复字段错误。
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

// ParamString 会裁剪前后空白。
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

// ParamInt 会把字符串解析为整数。
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

// ParamUUID 会校验并规范化 UUID 格式。
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

// ParamInt 解析失败时返回类型错误。
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

// 缺失的整数 path 参数会返回必填错误。
func TestParamInt_MissingValueReturnsRequired(t *testing.T) {
	req := requestWithPathParams(nil)

	_, err := ParamInt(req, "id")
	violation := assertSingleViolation(t, err)
	if violation.Field != "id" || violation.Code != ViolationCodeRequired || violation.Message != "is required" {
		t.Fatalf("violation = %#v", violation)
	}
}

// ParamString 遇到重复值时返回重复字段错误。
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

// 仅包含空白字符的路径参数会被视为缺失。
func TestParamString_WhitespaceOnlyValueReturnsRequired(t *testing.T) {
	req := requestWithPathParams(map[string][]string{
		"id": {"   "},
	})

	_, err := ParamString(req, "id")
	violation := assertSingleViolation(t, err)
	if violation.Field != "id" || violation.Code != ViolationCodeRequired || violation.Message != "is required" {
		t.Fatalf("violation = %#v", violation)
	}
}

// 缺失的字符串 path 参数会返回必填错误。
func TestParamString_MissingValueReturnsRequired(t *testing.T) {
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

// 空白参数名会直接返回参数名错误。
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

// 非法 UUID 会返回非法值错误。
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

// 重复的 UUID 路径参数会直接返回重复值错误。
func TestParamUUID_RepeatedValueReturnsViolation(t *testing.T) {
	req := requestWithPathParams(map[string][]string{
		"uuid": {
			"550e8400-e29b-41d4-a716-446655440000",
			"f47ac10b-58cc-4372-a567-0e02b2c3d479",
		},
	})

	_, err := ParamUUID(req, "uuid")
	violation := assertSingleViolation(t, err)
	if violation.Field != "uuid" || violation.Code != ViolationCodeMultiple || violation.Message != "must not be repeated" {
		t.Fatalf("violation = %#v", violation)
	}
}

// 缺失的 UUID path 参数会返回必填错误。
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
