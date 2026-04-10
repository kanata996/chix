package chix

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/go-chi/chi/v5"
	haherrx "github.com/kanata996/hah/errx"
	hahreqx "github.com/kanata996/hah/reqx"
)

// 测试清单：
// [✓] 根包 facade 会把 hah 的核心请求/响应能力稳定透传出来
// [✓] 根包 WriteError 会保留 chix 自己的 chi 侧 preset 行为
// [✓] README 中承诺的 create account handler 主路径有根包级端到端测试支撑

type rootPayloadMap map[string]any

type rootRequestValidatorBodyRequiredRequest struct {
	Name *string `json:"name"`
}

func (*rootRequestValidatorBodyRequiredRequest) ValidateRequest(r *http.Request) error {
	return RequireBody(r)
}

func TestBindAndValidateBody_DelegatesToHah(t *testing.T) {
	req := newJSONRequest(http.MethodPost, "/accounts", `{"name":"kanata"}`)

	var dst struct {
		Name string `json:"name" validate:"required"`
	}

	if err := BindAndValidateBody(req, &dst); err != nil {
		t.Fatalf("BindAndValidateBody() error = %v", err)
	}
	if dst.Name != "kanata" {
		t.Fatalf("name = %q, want kanata", dst.Name)
	}
}

func TestBind_DelegatesToHah(t *testing.T) {
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

func TestBindBody_DelegatesToHah(t *testing.T) {
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

func TestBindQueryParams_DelegatesToHah(t *testing.T) {
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

func TestBindPathValues_DelegatesToHah(t *testing.T) {
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

func TestBindHeaders_DelegatesToHah(t *testing.T) {
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

func TestBindAndValidate_DelegatesToHah(t *testing.T) {
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

func TestBindAndValidateBody_DelegatesRequestValidator(t *testing.T) {
	req := newJSONRequest(http.MethodPost, "/accounts", "")
	req.ContentLength = 0

	var dst rootRequestValidatorBodyRequiredRequest
	err := BindAndValidateBody(req, &dst)

	var httpErr *haherrx.HTTPError
	if !errors.As(err, &httpErr) || httpErr == nil {
		t.Fatalf("error type = %T, want *haherrx.HTTPError", err)
	}
	if httpErr.Status() != http.StatusUnprocessableEntity || httpErr.Code() != "invalid_request" || httpErr.Detail() != "request contains invalid fields" {
		t.Fatalf("http error = (%d, %q, %q)", httpErr.Status(), httpErr.Code(), httpErr.Detail())
	}
	if len(httpErr.Errors()) != 1 {
		t.Fatalf("errors len = %d, want 1", len(httpErr.Errors()))
	}
	violation, ok := httpErr.Errors()[0].(hahreqx.Violation)
	if !ok {
		t.Fatalf("detail type = %T, want hahreqx.Violation", httpErr.Errors()[0])
	}
	if violation.Field != "body" || violation.In != hahreqx.ViolationInBody || violation.Code != hahreqx.ViolationCodeRequired || violation.Detail != "is required" {
		t.Fatalf("violation = %#v", violation)
	}
}

func TestBindAndValidateQuery_DelegatesToHah(t *testing.T) {
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

func TestBindAndValidatePath_DelegatesToHah(t *testing.T) {
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

func TestBindAndValidateHeaders_DelegatesToHah(t *testing.T) {
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

func TestWriteError_DelegatesToHah(t *testing.T) {
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

func TestOK_DelegatesToHah(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/accounts", nil)
	rr := httptest.NewRecorder()

	if err := OK(rr, req, map[string]any{"id": "u_1"}); err != nil {
		t.Fatalf("OK() error = %v", err)
	}
	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rr.Code)
	}
	body := decodeRootPayload(t, rr.Body.Bytes())
	if got := body["id"]; got != "u_1" {
		t.Fatalf("id = %#v, want u_1", got)
	}
}

func TestCreated_DelegatesToHah(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/accounts", nil)
	rr := httptest.NewRecorder()

	if err := Created(rr, req, map[string]any{"id": "u_1"}); err != nil {
		t.Fatalf("Created() error = %v", err)
	}
	if rr.Code != http.StatusCreated {
		t.Fatalf("status = %d, want 201", rr.Code)
	}
}

func TestJSON_DelegatesToHah(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/accounts", nil)
	rr := httptest.NewRecorder()

	if err := JSON(rr, req, http.StatusAccepted, map[string]any{"id": "u_1"}); err != nil {
		t.Fatalf("JSON() error = %v", err)
	}
	if rr.Code != http.StatusAccepted {
		t.Fatalf("status = %d, want 202", rr.Code)
	}
}

func TestJSONBlob_DelegatesToHah(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/accounts", nil)
	rr := httptest.NewRecorder()

	if err := JSONBlob(rr, req, http.StatusOK, []byte(`{"id":"u_1"}`)); err != nil {
		t.Fatalf("JSONBlob() error = %v", err)
	}
	body := decodeRootPayload(t, rr.Body.Bytes())
	if got := body["id"]; got != "u_1" {
		t.Fatalf("id = %#v, want u_1", got)
	}
}

func TestNoContent_DelegatesToHah(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/accounts", nil)
	rr := httptest.NewRecorder()

	if err := NoContent(rr, req); err != nil {
		t.Fatalf("NoContent() error = %v", err)
	}
	if rr.Code != http.StatusNoContent {
		t.Fatalf("status = %d, want 204", rr.Code)
	}
}

func TestCreateAccountHandlerPath(t *testing.T) {
	type createAccountRequest struct {
		OrgID string `param:"org_id"`
		Name  string `json:"name" validate:"required,min=3"`
	}

	r := chi.NewRouter()
	r.Post("/orgs/{org_id}/accounts", func(w http.ResponseWriter, r *http.Request) {
		var req createAccountRequest
		if err := BindAndValidate(r, &req); err != nil {
			_ = WriteError(w, r, err)
			return
		}

		_ = Created(w, r, map[string]any{
			"id":     "acct_123",
			"org_id": req.OrgID,
			"name":   req.Name,
		})
	})

	req := newRouteJSONRequest(http.MethodPost, "/orgs/org_123/accounts", `{"name":"Acme"}`, "org_id", "org_123")
	rr := httptest.NewRecorder()
	r.ServeHTTP(rr, req)

	if rr.Code != http.StatusCreated {
		t.Fatalf("status = %d, want %d", rr.Code, http.StatusCreated)
	}

	body := decodeRootPayload(t, rr.Body.Bytes())
	if got := body["id"]; got != "acct_123" {
		t.Fatalf("id = %#v, want acct_123", got)
	}
	if got := body["org_id"]; got != "org_123" {
		t.Fatalf("org_id = %#v, want org_123", got)
	}
	if got := body["name"]; got != "Acme" {
		t.Fatalf("name = %#v, want Acme", got)
	}
}

func decodeRootPayload(t *testing.T, body []byte) rootPayloadMap {
	t.Helper()

	var payload rootPayloadMap
	if err := json.Unmarshal(body, &payload); err != nil {
		t.Fatalf("json.Unmarshal(%q) error = %v", strings.TrimSpace(string(body)), err)
	}
	return payload
}

func newJSONRequest(method, target, body string) *http.Request {
	req := httptest.NewRequest(method, target, strings.NewReader(body))
	if body != "" {
		req.Header.Set("Content-Type", "application/json")
	}
	return req
}

func newRouteRequest(method, target, param, value string) *http.Request {
	r := httptest.NewRequest(method, target, nil)
	pattern := strings.Replace(target, value, "{"+param+"}", 1)
	r.SetPathValue(param, value)
	r.Pattern = pattern
	return r
}

func newRouteJSONRequest(method, target, body, param, value string) *http.Request {
	r := newJSONRequest(method, target, body)
	pattern := strings.Replace(target, value, "{"+param+"}", 1)
	r.SetPathValue(param, value)
	r.Pattern = pattern
	return r
}
