package chix

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/go-chi/chi/v5"
	hah "github.com/kanata996/hah"
)

func TestSuccessResponseWritersMatchHah(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/accounts", nil)

	tests := []struct {
		name      string
		chixWrite func(http.ResponseWriter, *http.Request) error
		hahWrite  func(http.ResponseWriter, *http.Request) error
	}{
		{
			name: "JSON",
			chixWrite: func(w http.ResponseWriter, r *http.Request) error {
				return JSON(w, r, http.StatusAccepted, map[string]any{"ok": true})
			},
			hahWrite: func(w http.ResponseWriter, r *http.Request) error {
				return hah.JSON(w, http.StatusAccepted, map[string]any{"ok": true})
			},
		},
		{
			name: "JSONBlob",
			chixWrite: func(w http.ResponseWriter, r *http.Request) error {
				return JSONBlob(w, r, http.StatusAccepted, []byte(`{"ok":true}`))
			},
			hahWrite: func(w http.ResponseWriter, r *http.Request) error {
				return hah.JSONBlob(w, http.StatusAccepted, []byte(`{"ok":true}`))
			},
		},
		{
			name: "OK",
			chixWrite: func(w http.ResponseWriter, r *http.Request) error {
				return OK(w, r, map[string]any{"id": "acct_123"})
			},
			hahWrite: func(w http.ResponseWriter, r *http.Request) error {
				return hah.OK(w, map[string]any{"id": "acct_123"})
			},
		},
		{
			name: "Created",
			chixWrite: func(w http.ResponseWriter, r *http.Request) error {
				return Created(w, r, map[string]any{"id": "acct_123"})
			},
			hahWrite: func(w http.ResponseWriter, r *http.Request) error {
				return hah.Created(w, map[string]any{"id": "acct_123"})
			},
		},
		{
			name: "NoContent",
			chixWrite: func(w http.ResponseWriter, r *http.Request) error {
				return NoContent(w, r)
			},
			hahWrite: func(w http.ResponseWriter, r *http.Request) error {
				return hah.NoContent(w)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			chixRecorder := httptest.NewRecorder()
			if err := tt.chixWrite(chixRecorder, req); err != nil {
				t.Fatalf("chix writer error = %v", err)
			}

			hahRecorder := httptest.NewRecorder()
			if err := tt.hahWrite(hahRecorder, req); err != nil {
				t.Fatalf("hah writer error = %v", err)
			}

			if chixRecorder.Code != hahRecorder.Code {
				t.Fatalf("status = %d, want %d", chixRecorder.Code, hahRecorder.Code)
			}
			if chixRecorder.Header().Get("Content-Type") != hahRecorder.Header().Get("Content-Type") {
				t.Fatalf("content-type = %q, want %q", chixRecorder.Header().Get("Content-Type"), hahRecorder.Header().Get("Content-Type"))
			}
			if chixRecorder.Body.String() != hahRecorder.Body.String() {
				t.Fatalf("body = %q, want %q", chixRecorder.Body.String(), hahRecorder.Body.String())
			}
		})
	}
}

// 测试清单：
// [✓] 推荐用法可以用 hah 处理绑定，再用 chix 统一写成功/错误响应。
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
