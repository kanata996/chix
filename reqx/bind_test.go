package reqx

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/kanata996/chix/resp"
)

// 统一绑定时遵循 path、query、body 的覆盖顺序。
func TestBind_FollowsEchoOrder(t *testing.T) {
	type request struct {
		ID   string `param:"id" query:"id" json:"id"`
		Name string `query:"name" json:"name"`
	}

	req := requestWithPathParams(map[string][]string{
		"id": {"route-id"},
	})
	req.Method = http.MethodGet
	req.URL.RawQuery = "id=query-id&name=query-name"
	req.Header.Set("Content-Type", "application/json")
	req.Body = io.NopCloser(strings.NewReader(`{"id":"body-id","name":"body-name"}`))

	var bound request
	if err := Bind(req, &bound); err != nil {
		t.Fatalf("Bind() error = %v", err)
	}
	if bound.ID != "body-id" {
		t.Fatalf("Bind() id = %q", bound.ID)
	}
	if bound.Name != "body-name" {
		t.Fatalf("Bind() name = %q", bound.Name)
	}
}

// POST 请求不会绑定 query 参数。
func TestBind_SkipsQueryOnPost(t *testing.T) {
	type request struct {
		ID    string `param:"id" json:"id"`
		Scope string `query:"scope"`
	}

	req := requestWithPathParams(map[string][]string{
		"id": {"route-id"},
	})
	req.Method = http.MethodPost
	req.URL.RawQuery = "scope=query-scope"
	req.Header.Set("Content-Type", "application/json")
	req.Body = io.NopCloser(strings.NewReader(`{"id":"body-id"}`))

	var bound request
	if err := Bind(req, &bound); err != nil {
		t.Fatalf("Bind() error = %v", err)
	}
	if bound.ID != "body-id" {
		t.Fatalf("Bind() id = %q", bound.ID)
	}
	if bound.Scope != "" {
		t.Fatalf("Bind() scope = %q", bound.Scope)
	}
}

// 空 body 场景下忽略不合法的 Content-Type。
func TestBind_IgnoresEmptyBodyContentType(t *testing.T) {
	type request struct {
		Page int `query:"page"`
	}

	req := httptest.NewRequest(http.MethodGet, "/items?page=2", nil)
	req.Header.Set("Content-Type", "text/plain")

	var bound request
	if err := Bind(req, &bound); err != nil {
		t.Fatalf("Bind() error = %v", err)
	}
	if bound.Page != 2 {
		t.Fatalf("Bind() page = %d", bound.Page)
	}
}

// DELETE 请求中 query 参数会覆盖 path 参数。
func TestBind_BindsQueryOverPathOnDelete(t *testing.T) {
	type request struct {
		ID string `param:"id" query:"id"`
	}

	req := requestWithPathParams(map[string][]string{
		"id": {"route-id"},
	})
	req.Method = http.MethodDelete
	req.URL.RawQuery = "id=query-id"

	var bound request
	if err := Bind(req, &bound); err != nil {
		t.Fatalf("Bind() error = %v", err)
	}
	if bound.ID != "query-id" {
		t.Fatalf("Bind() id = %q", bound.ID)
	}
}

// Header 绑定时会使用规范化后的请求头名称。
func TestBindHeaders_BindsCanonicalHeader(t *testing.T) {
	type request struct {
		RequestID string `header:"x-request-id"`
	}

	req := httptest.NewRequest(http.MethodGet, "/items", nil)
	req.Header.Set("X-Request-Id", "req-123")

	var bound request
	if err := BindHeaders(req, &bound); err != nil {
		t.Fatalf("BindHeaders() error = %v", err)
	}
	if bound.RequestID != "req-123" {
		t.Fatalf("BindHeaders() request_id = %q", bound.RequestID)
	}
}

// 非规范 header key 仍会被规范化后参与绑定。
func TestBindHeaders_NormalizesNonCanonicalHeaderKey(t *testing.T) {
	type request struct {
		RequestID string `header:"x-request-id"`
	}

	req := httptest.NewRequest(http.MethodGet, "/items", nil)
	req.Header = http.Header{
		" x-request-id ": {"req-123"},
	}

	var bound request
	if err := BindHeaders(req, &bound); err != nil {
		t.Fatalf("BindHeaders() error = %v", err)
	}
	if bound.RequestID != "req-123" {
		t.Fatalf("BindHeaders() request_id = %q", bound.RequestID)
	}
}

// 请求级校验错误优先使用请求标签中的字段名。
func TestBindAndValidate_UsesRequestTagNames(t *testing.T) {
	type request struct {
		UUID string `param:"uuid" validate:"required"`
	}

	req := httptest.NewRequest(http.MethodGet, "/items", nil)

	var bound request
	err := BindAndValidate(req, &bound)
	if err == nil {
		t.Fatal("BindAndValidate() error = nil")
	}

	httpErr, ok := err.(*resp.HTTPError)
	if !ok {
		t.Fatalf("BindAndValidate() error type = %T", err)
	}
	details := httpErr.Details()
	if len(details) != 1 {
		t.Fatalf("BindAndValidate() details len = %d", len(details))
	}

	violation, ok := details[0].(Violation)
	if !ok {
		t.Fatalf("BindAndValidate() detail type = %T", details[0])
	}
	if violation.Field != "uuid" || violation.Code != ViolationCodeRequired {
		t.Fatalf("BindAndValidate() violation = %#v", violation)
	}
}

// GET 请求默认忽略未声明的 query 字段。
func TestBind_IgnoresUnknownQueryFieldOnGet(t *testing.T) {
	type request struct {
		Page int `query:"page"`
	}

	req := httptest.NewRequest(http.MethodGet, "/items?page=2&extra=1", nil)

	var bound request
	if err := Bind(req, &bound); err != nil {
		t.Fatalf("Bind() error = %v", err)
	}
	if bound.Page != 2 {
		t.Fatalf("Bind() page = %d", bound.Page)
	}
}

// body 存在且 Content-Type 非 JSON 时会返回媒体类型错误。
func TestBind_RejectsUnsupportedBodyContentTypeWhenBodyPresent(t *testing.T) {
	type request struct {
		Name string `json:"name"`
	}

	req := httptest.NewRequest(http.MethodPost, "/items", strings.NewReader(`{"name":"kanata"}`))
	req.Header.Set("Content-Type", "text/plain")

	var bound request
	err := Bind(req, &bound)
	_ = assertHTTPError(t, err, http.StatusUnsupportedMediaType, CodeUnsupportedMediaType, "Content-Type must be application/json or application/*+json")
}

// GET 请求中的 query 绑定错误会直接向上返回。
func TestBind_PropagatesQueryBindingErrorOnGet(t *testing.T) {
	type request struct {
		Page int `query:"page"`
	}

	req := httptest.NewRequest(http.MethodGet, "/items?page=oops", nil)

	var bound request
	err := Bind(req, &bound)
	violation := assertSingleViolation(t, err)
	if violation.Field != "page" || violation.Code != ViolationCodeType || violation.Message != "must be number" {
		t.Fatalf("violation = %#v", violation)
	}
}

// POST 请求无 body 时保留已绑定的 path 参数。
func TestBind_UsesPathValueWhenPostHasNoBody(t *testing.T) {
	type request struct {
		ID string `param:"id" json:"id"`
	}

	req := requestWithPathParams(map[string][]string{
		"id": {"route-id"},
	})
	req.Method = http.MethodPost

	var bound request
	if err := Bind(req, &bound); err != nil {
		t.Fatalf("Bind() error = %v", err)
	}
	if bound.ID != "route-id" {
		t.Fatalf("Bind() id = %q", bound.ID)
	}
}
