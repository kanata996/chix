package chix

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/go-chi/chi/v5"
)

// 测试清单：
// [✓] 根包 facade 会把 reqx 的绑定、校验与 path 参数 helper 稳定透传出来
// [✓] 根包 facade 会把 resp 的成功响应与错误响应 helper 稳定透传出来
// [✓] 根包公开 option 会继续委托到底层 reqx option，而不改变默认契约

type rootPayloadMap map[string]any

// BindAndValidateBody 会通过根包 facade 把 body 绑定和校验委托给 reqx。
func TestBindAndValidateBody_DelegatesToReqx(t *testing.T) {
	req := newJSONRequest(http.MethodPost, "/accounts", `{"name":"kanata"}`)

	var dst struct {
		Name string `json:"name" validate:"required"`
	}

	err := BindAndValidateBody(req, &dst)
	if err != nil {
		t.Fatalf("BindAndValidateBody() error = %v", err)
	}
	if dst.Name != "kanata" {
		t.Fatalf("name = %q, want kanata", dst.Name)
	}
}

// Bind 会通过根包 facade 复用 Echo 风格的 path/query/body 绑定顺序。
func TestBind_DelegatesToReqx(t *testing.T) {
	type request struct {
		ID   string `param:"id" query:"id" json:"id"`
		Name string `json:"name"`
	}

	req := newRouteRequest(http.MethodGet, "/accounts?id=query-id", "id", "route-id")

	var dst request
	if err := Bind(req, &dst); err != nil {
		t.Fatalf("Bind() error = %v", err)
	}
	if dst.ID != "query-id" {
		t.Fatalf("id = %q, want query-id", dst.ID)
	}
}

// BindBody 只从 JSON body 绑定数据。
func TestBindBody_DelegatesToReqx(t *testing.T) {
	req := newJSONRequest(http.MethodPost, "/accounts", `{"name":"kanata"}`)

	var dst struct {
		Name string `json:"name"`
	}

	if err := BindBody(req, &dst); err != nil {
		t.Fatalf("BindBody() error = %v", err)
	}
	if dst.Name != "kanata" {
		t.Fatalf("name = %q, want kanata", dst.Name)
	}
}

// BindQueryParams 只从 query 参数绑定数据。
func TestBindQueryParams_DelegatesToReqx(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/accounts?name=kanata", nil)

	var dst struct {
		Name string `query:"name"`
	}

	if err := BindQueryParams(req, &dst); err != nil {
		t.Fatalf("BindQueryParams() error = %v", err)
	}
	if dst.Name != "kanata" {
		t.Fatalf("name = %q, want kanata", dst.Name)
	}
}

// BindPathValues 只从 path 参数绑定数据。
func TestBindPathValues_DelegatesToReqx(t *testing.T) {
	req := newRouteRequest(http.MethodGet, "/accounts/u_1", "id", "u_1")

	var dst struct {
		ID string `param:"id"`
	}

	if err := BindPathValues(req, &dst); err != nil {
		t.Fatalf("BindPathValues() error = %v", err)
	}
	if dst.ID != "u_1" {
		t.Fatalf("id = %q, want u_1", dst.ID)
	}
}

// BindHeaders 会通过根包 facade 把 header 绑定委托给 reqx。
func TestBindHeaders_DelegatesToReqx(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/accounts", nil)
	req.Header.Set("X-Request-Id", "req-123")

	var dst struct {
		RequestID string `header:"x-request-id"`
	}

	if err := BindHeaders(req, &dst); err != nil {
		t.Fatalf("BindHeaders() error = %v", err)
	}
	if dst.RequestID != "req-123" {
		t.Fatalf("request_id = %q, want req-123", dst.RequestID)
	}
}

// BindAndValidate 会在根包 facade 中同时复用绑定和校验能力。
func TestBindAndValidate_DelegatesToReqx(t *testing.T) {
	type request struct {
		OrgID string `param:"org_id" validate:"required"`
		Name  string `json:"name" validate:"required"`
	}

	req := newRouteJSONRequest(http.MethodPost, "/orgs/o_1/accounts", `{"name":"kanata"}`, "org_id", "o_1")

	var dst request
	if err := BindAndValidate(req, &dst); err != nil {
		t.Fatalf("BindAndValidate() error = %v", err)
	}
	if dst.OrgID != "o_1" {
		t.Fatalf("org_id = %q, want o_1", dst.OrgID)
	}
	if dst.Name != "kanata" {
		t.Fatalf("name = %q, want kanata", dst.Name)
	}
}

// BindAndValidateQuery 会从 query 参数绑定后再执行校验。
func TestBindAndValidateQuery_DelegatesToReqx(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/accounts?name=kanata", nil)

	var dst struct {
		Name string `query:"name" validate:"required"`
	}

	if err := BindAndValidateQuery(req, &dst); err != nil {
		t.Fatalf("BindAndValidateQuery() error = %v", err)
	}
	if dst.Name != "kanata" {
		t.Fatalf("name = %q, want kanata", dst.Name)
	}
}

// BindAndValidatePath 会从 path 参数绑定后再执行校验。
func TestBindAndValidatePath_DelegatesToReqx(t *testing.T) {
	req := newRouteRequest(http.MethodGet, "/accounts/u_1", "id", "u_1")

	var dst struct {
		ID string `param:"id" validate:"required"`
	}

	if err := BindAndValidatePath(req, &dst); err != nil {
		t.Fatalf("BindAndValidatePath() error = %v", err)
	}
	if dst.ID != "u_1" {
		t.Fatalf("id = %q, want u_1", dst.ID)
	}
}

// BindAndValidateHeaders 会从 header 绑定后再执行校验。
func TestBindAndValidateHeaders_DelegatesToReqx(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/accounts", nil)
	req.Header.Set("X-Request-Id", "req-123")

	var dst struct {
		RequestID string `header:"x-request-id" validate:"required"`
	}

	if err := BindAndValidateHeaders(req, &dst); err != nil {
		t.Fatalf("BindAndValidateHeaders() error = %v", err)
	}
	if dst.RequestID != "req-123" {
		t.Fatalf("request_id = %q, want req-123", dst.RequestID)
	}
}

// ParamString 会通过根包 facade 读取并裁剪必填 path 字符串参数。
func TestParamString_DelegatesToReqx(t *testing.T) {
	req := newRouteRequest(http.MethodGet, "/accounts", "id", "  acct_123  ")

	got, err := ParamString(req, "id")
	if err != nil {
		t.Fatalf("ParamString() error = %v", err)
	}
	if got != "acct_123" {
		t.Fatalf("id = %q, want acct_123", got)
	}
}

// ParamInt 会通过根包 facade 把 path 参数解析为整数。
func TestParamInt_DelegatesToReqx(t *testing.T) {
	req := newRouteRequest(http.MethodGet, "/accounts", "page", "42")

	got, err := ParamInt(req, "page")
	if err != nil {
		t.Fatalf("ParamInt() error = %v", err)
	}
	if got != 42 {
		t.Fatalf("page = %d, want 42", got)
	}
}

// ParamUUID 会通过根包 facade 复用 UUID 规范化逻辑。
func TestParamUUID_DelegatesToReqx(t *testing.T) {
	req := newRouteRequest(http.MethodGet, "/accounts", "uuid", "550E8400-E29B-41D4-A716-446655440000")

	got, err := ParamUUID(req, "uuid")
	if err != nil {
		t.Fatalf("ParamUUID() error = %v", err)
	}
	if got != "550e8400-e29b-41d4-a716-446655440000" {
		t.Fatalf("uuid = %q", got)
	}
}

// WriteError 会通过根包 facade 写出统一的公开错误包络。
func TestWriteError_DelegatesToResp(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/accounts", nil)
	rr := httptest.NewRecorder()

	if err := WriteError(rr, req, context.DeadlineExceeded); err != nil {
		t.Fatalf("WriteError() error = %v", err)
	}
	if rr.Code != http.StatusGatewayTimeout {
		t.Fatalf("status = %d, want %d", rr.Code, http.StatusGatewayTimeout)
	}

	body := decodeRootPayload(t, rr.Body.Bytes())
	if got := body["code"]; got != "timeout" {
		t.Fatalf("code = %#v, want timeout", got)
	}
	if got := body["title"]; got != http.StatusText(http.StatusGatewayTimeout) {
		t.Fatalf("title = %#v, want %q", got, http.StatusText(http.StatusGatewayTimeout))
	}
}

// OK 会通过根包 facade 写回标准 200 JSON 响应。
func TestOK_DelegatesToResp(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/accounts", nil)
	rr := httptest.NewRecorder()

	if err := OK(rr, req, map[string]any{"id": "u_1"}); err != nil {
		t.Fatalf("OK() error = %v", err)
	}
	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rr.Code)
	}

	payload := decodeRootPayload(t, rr.Body.Bytes())
	if payload["id"] != "u_1" {
		t.Fatalf("id = %#v, want u_1", payload["id"])
	}
}

// JSON 会通过根包 facade 复用 pretty query 的响应格式。
func TestJSON_DelegatesToResp(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/accounts?pretty", nil)
	rr := httptest.NewRecorder()

	if err := JSON(rr, req, http.StatusAccepted, map[string]any{"id": "u_1"}); err != nil {
		t.Fatalf("JSON() error = %v", err)
	}
	if rr.Code != http.StatusAccepted {
		t.Fatalf("status = %d, want 202", rr.Code)
	}
	if body := rr.Body.String(); body != "{\n  \"id\": \"u_1\"\n}\n" {
		t.Fatalf("body = %q, want pretty JSON", body)
	}
}

// JSONPretty 会通过根包 facade 按指定缩进写回 pretty JSON。
func TestJSONPretty_DelegatesToResp(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/accounts", nil)
	rr := httptest.NewRecorder()

	if err := JSONPretty(rr, req, http.StatusOK, map[string]any{"id": "u_1"}, "    "); err != nil {
		t.Fatalf("JSONPretty() error = %v", err)
	}
	if body := rr.Body.String(); body != "{\n    \"id\": \"u_1\"\n}\n" {
		t.Fatalf("body = %q, want indented JSON", body)
	}
}

// JSONBlob 会通过根包 facade 直接写回原始 JSON 字节。
func TestJSONBlob_DelegatesToResp(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/accounts", nil)
	rr := httptest.NewRecorder()

	if err := JSONBlob(rr, req, http.StatusAccepted, []byte(`{"id":"u_1"}`)); err != nil {
		t.Fatalf("JSONBlob() error = %v", err)
	}
	if rr.Code != http.StatusAccepted {
		t.Fatalf("status = %d, want %d", rr.Code, http.StatusAccepted)
	}
	if body := rr.Body.String(); body != `{"id":"u_1"}` {
		t.Fatalf("body = %q, want raw JSON", body)
	}
}

// Created 会通过根包 facade 写回标准 201 JSON 响应。
func TestCreated_DelegatesToResp(t *testing.T) {
	req := httptest.NewRequest(http.MethodPost, "/accounts", nil)
	rr := httptest.NewRecorder()

	if err := Created(rr, req, map[string]any{"id": "u_1"}); err != nil {
		t.Fatalf("Created() error = %v", err)
	}
	if rr.Code != http.StatusCreated {
		t.Fatalf("status = %d, want %d", rr.Code, http.StatusCreated)
	}

	payload := decodeRootPayload(t, rr.Body.Bytes())
	if payload["id"] != "u_1" {
		t.Fatalf("id = %#v, want u_1", payload["id"])
	}
}

// NoContent 会通过根包 facade 写回标准 204 响应。
func TestNoContent_DelegatesToResp(t *testing.T) {
	req := httptest.NewRequest(http.MethodDelete, "/accounts/u_1", nil)
	rr := httptest.NewRecorder()

	if err := NoContent(rr, req); err != nil {
		t.Fatalf("NoContent() error = %v", err)
	}
	if rr.Code != http.StatusNoContent {
		t.Fatalf("status = %d, want %d", rr.Code, http.StatusNoContent)
	}
	if rr.Body.Len() != 0 {
		t.Fatalf("body = %q, want empty", rr.Body.String())
	}
}

// WithMaxBodyBytes 会通过根包 facade 把 body 大小限制透传给 reqx。
func TestWithMaxBodyBytes_DelegatesToReqx(t *testing.T) {
	req := newJSONRequest(http.MethodPost, "/accounts", `{"name":"kanata"}`)

	var dst struct {
		Name string `json:"name"`
	}

	err := Bind(req, &dst, WithMaxBodyBytes(4))
	_ = assertRootHTTPError(t, err, http.StatusRequestEntityTooLarge, "request_too_large", "request body is too large")
}

func newJSONRequest(method, target, body string) *http.Request {
	req := httptest.NewRequest(method, target, strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	return req
}

func newRouteRequest(method, target, name, value string) *http.Request {
	req := httptest.NewRequest(method, target, nil)
	return withRouteParam(req, name, value)
}

func newRouteJSONRequest(method, target, body, name, value string) *http.Request {
	req := newJSONRequest(method, target, body)
	return withRouteParam(req, name, value)
}

func withRouteParam(req *http.Request, name, value string) *http.Request {
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add(name, value)
	return req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))
}

func decodeRootPayload(t *testing.T, body []byte) rootPayloadMap {
	t.Helper()

	var payload rootPayloadMap
	if err := json.Unmarshal(body, &payload); err != nil {
		t.Fatalf("json.Unmarshal() error = %v", err)
	}
	return payload
}

func assertRootHTTPError(t *testing.T, err error, wantStatus int, wantCode, wantMessage string) *HTTPError {
	t.Helper()

	httpErr, ok := err.(*HTTPError)
	if !ok {
		t.Fatalf("error type = %T, want *HTTPError", err)
	}
	if got := httpErr.Status(); got != wantStatus {
		t.Fatalf("Status() = %d, want %d", got, wantStatus)
	}
	if got := httpErr.Code(); got != wantCode {
		t.Fatalf("Code() = %q, want %q", got, wantCode)
	}
	if got := httpErr.Message(); got != wantMessage {
		t.Fatalf("Message() = %q, want %q", got, wantMessage)
	}
	return httpErr
}
