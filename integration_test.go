package chix

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/go-chi/chi/v5"
	hah "github.com/kanata996/hah"
)

// 测试清单：
// [✓] 推荐用法可以用 hah 处理绑定与成功响应，再用 chix.WriteError 处理 chi 侧错误输出。
// [✓] README 中承诺的 create account handler 主路径有根包级端到端测试支撑。

func TestCreateAccountHandlerPath(t *testing.T) {
	type createAccountRequest struct {
		OrgID string `param:"org_id"`
		Name  string `json:"name" validate:"required,min=3"`
	}

	r := chi.NewRouter()
	r.Post("/orgs/{org_id}/accounts", func(w http.ResponseWriter, r *http.Request) {
		var req createAccountRequest
		if err := hah.BindAndValidate(r, &req); err != nil {
			_ = WriteError(w, r, err)
			return
		}

		_ = hah.Created(w, r, map[string]any{
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

func newJSONRequest(method, target, body string) *http.Request {
	req := httptest.NewRequest(method, target, strings.NewReader(body))
	if body != "" {
		req.Header.Set("Content-Type", "application/json")
	}
	return req
}

func newRouteJSONRequest(method, target, body, param, value string) *http.Request {
	r := newJSONRequest(method, target, body)
	pattern := strings.Replace(target, value, "{"+param+"}", 1)
	r.SetPathValue(param, value)
	r.Pattern = pattern
	return r
}
