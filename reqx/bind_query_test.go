package reqx

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

type queryState string

func (s *queryState) UnmarshalText(text []byte) error {
	switch string(text) {
	case "active", "disabled":
		*s = queryState(text)
		return nil
	default:
		return errors.New("invalid state")
	}
}

// 查询参数可以绑定到支持的标量、指针、切片和 TextUnmarshaler 类型。
func TestBindQueryParams_BindsSupportedTypes(t *testing.T) {
	type request struct {
		Name    string       `query:"name"`
		Enabled bool         `query:"enabled"`
		Age     *int         `query:"age"`
		Score   float64      `query:"score"`
		Tags    []string     `query:"tag"`
		State   queryState   `query:"state"`
		States  []queryState `query:"states"`
	}

	req := httptest.NewRequest(
		http.MethodGet,
		"/?name=kanata&enabled=true&age=17&score=9.5&tag=a&tag=b&state=active&states=active&states=disabled",
		nil,
	)

	var dst request
	if err := BindQueryParams(req, &dst); err != nil {
		t.Fatalf("BindQueryParams() error = %v", err)
	}
	if dst.Name != "kanata" {
		t.Fatalf("name = %q, want kanata", dst.Name)
	}
	if !dst.Enabled {
		t.Fatal("enabled = false, want true")
	}
	if dst.Age == nil || *dst.Age != 17 {
		t.Fatalf("age = %#v, want 17", dst.Age)
	}
	if dst.Score != 9.5 {
		t.Fatalf("score = %v, want 9.5", dst.Score)
	}
	if strings.Join(dst.Tags, ",") != "a,b" {
		t.Fatalf("tags = %#v, want [a b]", dst.Tags)
	}
	if dst.State != "active" {
		t.Fatalf("state = %q, want active", dst.State)
	}
	if len(dst.States) != 2 || dst.States[0] != "active" || dst.States[1] != "disabled" {
		t.Fatalf("states = %#v, want [active disabled]", dst.States)
	}
}

// 默认配置下，未知查询参数会被忽略。
func TestBindQueryParams_IgnoresUnknownFieldsByDefault(t *testing.T) {
	type request struct {
		Page int `query:"page"`
	}

	req := httptest.NewRequest(http.MethodGet, "/?page=2&extra=1", nil)

	var dst request
	if err := BindQueryParams(req, &dst); err != nil {
		t.Fatalf("BindQueryParams() error = %v", err)
	}
	if dst.Page != 2 {
		t.Fatalf("page = %d, want 2", dst.Page)
	}
}

// 标量查询参数重复出现时返回重复值错误。
func TestBindQueryParams_RejectsRepeatedScalar(t *testing.T) {
	type request struct {
		ID string `query:"id"`
	}

	req := httptest.NewRequest(http.MethodGet, "/?id=1&id=2", nil)

	var dst request
	err := BindQueryParams(req, &dst)
	violation := assertSingleViolation(t, err)
	if violation.Field != "id" || violation.Code != ViolationCodeMultiple || violation.Message != "must not be repeated" {
		t.Fatalf("violation = %#v", violation)
	}
}

// 查询参数类型不匹配时返回类型错误。
func TestBindQueryParams_RejectsTypeMismatch(t *testing.T) {
	type request struct {
		Page int `query:"page"`
	}

	req := httptest.NewRequest(http.MethodGet, "/?page=nope", nil)

	var dst request
	err := BindQueryParams(req, &dst)
	violation := assertSingleViolation(t, err)
	if violation.Field != "page" || violation.Code != ViolationCodeType || violation.Message != "must be number" {
		t.Fatalf("violation = %#v", violation)
	}
}

// TextUnmarshaler 返回错误时转换为非法值错误。
func TestBindQueryParams_RejectsInvalidTextUnmarshalerValue(t *testing.T) {
	type request struct {
		State queryState `query:"state"`
	}

	req := httptest.NewRequest(http.MethodGet, "/?state=unknown", nil)

	var dst request
	err := BindQueryParams(req, &dst)
	violation := assertSingleViolation(t, err)
	if violation.Field != "state" || violation.Code != ViolationCodeInvalid || violation.Message != "is invalid" {
		t.Fatalf("violation = %#v", violation)
	}
}

// 不支持的查询字段类型会在构建解码计划时被拒绝。
func TestBindQueryParams_RejectsUnsupportedFieldType(t *testing.T) {
	type nested struct {
		Name string
	}
	type request struct {
		Nested nested `query:"nested"`
	}

	req := httptest.NewRequest(http.MethodGet, "/?nested=x", nil)

	var dst request
	err := BindQueryParams(req, &dst)
	if err == nil {
		t.Fatal("BindQueryParams() error = nil")
	}
	if got := err.Error(); !strings.Contains(got, `unsupported query type`) {
		t.Fatalf("error = %q, want unsupported query type", got)
	}
}

// 重复声明同名 query 标签会返回重复字段错误。
func TestBindQueryParams_RejectsDuplicateTaggedFields(t *testing.T) {
	type request struct {
		Page  int `query:"page"`
		Limit int `query:"page"`
	}

	req := httptest.NewRequest(http.MethodGet, "/?page=1", nil)

	var dst request
	err := BindQueryParams(req, &dst)
	if err == nil {
		t.Fatal("BindQueryParams() error = nil")
	}
	if got := err.Error(); got != `reqx: duplicate query field "page" on Page and Limit` {
		t.Fatalf("error = %q", got)
	}
}

// 未导出的 query 字段不允许参与绑定。
func TestBindQueryParams_RejectsUnexportedTaggedField(t *testing.T) {
	type request struct {
		name string `query:"name"`
	}

	req := httptest.NewRequest(http.MethodGet, "/?name=kanata", nil)

	var dst request
	_ = dst.name
	err := BindQueryParams(req, &dst)
	if err == nil {
		t.Fatal("BindQueryParams() error = nil")
	}
	if got := err.Error(); !strings.Contains(got, `must be exported`) {
		t.Fatalf("error = %q, want must be exported", got)
	}
}

// 请求对象不能为空。
func TestBindQueryParams_RequestMustNotBeNil(t *testing.T) {
	var dst struct{}
	err := BindQueryParams[struct{}](nil, &dst)
	if err == nil {
		t.Fatal("BindQueryParams() error = nil")
	}
	if got := err.Error(); got != "reqx: request must not be nil" {
		t.Fatalf("error = %q", got)
	}
}

// 目标对象不能为空。
func TestBindQueryParams_DestinationMustNotBeNil(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/?page=1", nil)

	err := BindQueryParams[struct{}](req, nil)
	if err == nil {
		t.Fatal("BindQueryParams() error = nil")
	}
	if got := err.Error(); got != "reqx: destination must not be nil" {
		t.Fatalf("error = %q", got)
	}
}

// 缺失查询参数时，字段保持零值。
func TestBindQueryParams_MissingParamsLeaveZeroValue(t *testing.T) {
	type request struct {
		Page int  `query:"page"`
		Age  *int `query:"age"`
	}

	req := httptest.NewRequest(http.MethodGet, "/?other=1", nil)

	var dst request
	if err := BindQueryParams(req, &dst); err != nil {
		t.Fatalf("BindQueryParams() error = %v", err)
	}
	if dst.Page != 0 {
		t.Fatalf("page = %d, want 0", dst.Page)
	}
	if dst.Age != nil {
		t.Fatalf("age = %#v, want nil", dst.Age)
	}
}

// nil URL 会按空查询串处理。
func TestBindQueryParams_NilURLTreatsQueryAsEmpty(t *testing.T) {
	type request struct {
		Page int `query:"page"`
	}

	req := &http.Request{Header: make(http.Header)}

	var dst request
	if err := BindQueryParams(req, &dst); err != nil {
		t.Fatalf("BindQueryParams() error = %v", err)
	}
	if dst.Page != 0 {
		t.Fatalf("page = %d, want 0", dst.Page)
	}
}

// 重复的 TextUnmarshaler 参数会返回重复值错误。
func TestBindQueryParams_RejectsRepeatedTextUnmarshaler(t *testing.T) {
	type request struct {
		State queryState `query:"state"`
	}

	req := httptest.NewRequest(http.MethodGet, "/?state=active&state=disabled", nil)

	var dst request
	err := BindQueryParams(req, &dst)
	violation := assertSingleViolation(t, err)
	if violation.Field != "state" || violation.Code != ViolationCodeMultiple || violation.Message != "must not be repeated" {
		t.Fatalf("violation = %#v", violation)
	}
}

// 指针类型解析失败时返回类型错误。
func TestBindQueryParams_RejectsPointerTypeMismatch(t *testing.T) {
	type request struct {
		Page *int `query:"page"`
	}

	req := httptest.NewRequest(http.MethodGet, "/?page=oops", nil)

	var dst request
	err := BindQueryParams(req, &dst)
	violation := assertSingleViolation(t, err)
	if violation.Field != "page" || violation.Code != ViolationCodeType || violation.Message != "must be number" {
		t.Fatalf("violation = %#v", violation)
	}
	if dst.Page != nil {
		t.Fatalf("page = %#v, want nil", dst.Page)
	}
}

// 查询校验错误字段名使用 query tag 名称。
func TestBindAndValidateQuery_UsesQueryTagName(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/", nil)

	var dst struct {
		Cursor string `query:"cursor" validate:"required"`
	}
	err := BindAndValidateQuery(req, &dst)
	violation := assertSingleViolation(t, err)
	if violation.Field != "cursor" || violation.Code != ViolationCodeRequired || violation.Message != "is required" {
		t.Fatalf("violation = %#v", violation)
	}
}

// 绑定后会先标准化再校验。
func TestBindAndValidateQuery_NormalizesBeforeValidation(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/?name=%20kanata%20", nil)

	var dst normalizedQueryRequest
	if err := BindAndValidateQuery(req, &dst); err != nil {
		t.Fatalf("BindAndValidateQuery() error = %v", err)
	}
	if dst.Name != "kanata" {
		t.Fatalf("name = %q, want kanata", dst.Name)
	}
}

// 不支持的值来源会触发 panic。
func TestSourceValues_PanicsOnUnsupportedSource(t *testing.T) {
	defer func() {
		if recover() == nil {
			t.Fatal("sourceValues() did not panic")
		}
	}()

	_ = sourceValues(httptest.NewRequest(http.MethodGet, "/", nil), valueSource{tag: "unsupported"})
}

type normalizedQueryRequest struct {
	Name string `query:"name" validate:"required,nospace"`
}

func (r *normalizedQueryRequest) Normalize() {
	r.Name = strings.TrimSpace(r.Name)
}
