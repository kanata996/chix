package bind

// 测试清单：
// - 标记说明：[✓] 已核对且已有真实覆盖；[x] 尚未完成，不得作为验收依据。
// - [✓] `BindBody` 在任意 body 与 Content-Type 输入下维持 JSON-only 契约：空 body no-op、非 JSON 拒绝、合法 JSON 写入目标、非法 JSON 返回 `invalid_json`。
// - [✓] 本文件提供单一 `FuzzBindBodyJSONContract` 入口，可直接配合仓库规范中的 `-fuzz=Fuzz` 执行。

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func FuzzBindBodyJSONContract(f *testing.F) {
	f.Add("", []byte(nil))
	f.Add("application/json", []byte(`{"name":"kanata","count":2}`))
	f.Add("application/json; charset=utf-8", []byte(`{"name":"kanata"}`))
	f.Add("application/xml", []byte(`{"name":"kanata"}`))
	f.Add("application/json", []byte(`{"name":`))

	f.Fuzz(func(t *testing.T, contentType string, body []byte) {
		type request struct {
			Name  string `json:"name"`
			Count int    `json:"count"`
		}

		req := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(string(body)))
		req.ContentLength = int64(len(body))
		req.Header.Set("Content-Type", contentType)

		initial := request{Name: "existing", Count: 7}
		dst := initial
		err := BindBody(req, &dst)

		if len(body) == 0 {
			if err != nil {
				t.Fatalf("BindBody() error = %v, want nil for empty body", err)
			}
			if dst != initial {
				t.Fatalf("dst = %#v, want unchanged target", dst)
			}
			return
		}

		base, _, _ := strings.Cut(contentType, ";")
		if strings.TrimSpace(base) != "application/json" {
			assertHTTPError(t, err, http.StatusUnsupportedMediaType, CodeUnsupportedMediaType, "Content-Type must be application/json")
			if dst != initial {
				t.Fatalf("dst = %#v, want unchanged target on media type rejection", dst)
			}
			return
		}

		if !json.Valid(body) {
			assertHTTPError(t, err, http.StatusBadRequest, CodeInvalidJSON, "request body must be valid JSON")
			return
		}

		want := initial
		if unmarshalErr := json.Unmarshal(body, &want); unmarshalErr != nil {
			assertHTTPError(t, err, http.StatusBadRequest, CodeInvalidJSON, "request body must be valid JSON")
			return
		}

		if err != nil {
			t.Fatalf("BindBody() error = %v", err)
		}
		if dst != want {
			t.Fatalf("dst = %#v, want %#v", dst, want)
		}
	})
}
