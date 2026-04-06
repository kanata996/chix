package reqx

// 用例清单：
// - 标记说明：[✓] 已核对且已有真实覆盖；[x] 本轮审查发现缺口后补测。
// - [✓] `Bind` 的阶段顺序、覆盖优先级与按 HTTP 方法启用/跳过规则。
// - [✓] `Bind` 在空 body 时会把 body 阶段视为 no-op，并忽略该场景的 Content-Type。
// - [✓] `BindHeaders` 会规范化请求头键名，并拒绝重复的标量 header。
// - [✓] `Bind`/`BindAndValidate` 会透传阶段错误，并使用请求级字段名。

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// 顶层 Bind 只负责来源顺序和按方法启用/跳过各阶段。
func TestBind_StageOrderAndMethodRules(t *testing.T) {
	t.Run("get applies path query then body", func(t *testing.T) {
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
		if bound.ID != "body-id" || bound.Name != "body-name" {
			t.Fatalf("bound = %#v, want body values to win", bound)
		}
	})

	t.Run("delete binds query over path", func(t *testing.T) {
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
	})

	t.Run("post skips query but still binds body", func(t *testing.T) {
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
		if bound.ID != "body-id" || bound.Scope != "" {
			t.Fatalf("bound = %#v, want body id and skipped query scope", bound)
		}
	})
}

// 顶层 Bind 在空 body 场景下会把 body 阶段视为 no-op，因此不会校验该阶段的 Content-Type。
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

// Header 绑定会兼容规范和非规范请求头键名。
func TestBindHeaders_NormalizesHeaderKeys(t *testing.T) {
	type request struct {
		RequestID string `header:"x-request-id"`
	}

	testCases := []struct {
		name   string
		header http.Header
	}{
		{
			name: "canonical key",
			header: http.Header{
				"X-Request-Id": {"req-123"},
			},
		},
		{
			name: "non canonical key",
			header: http.Header{
				" x-request-id ": {"req-123"},
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "/items", nil)
			req.Header = tc.header

			var bound request
			if err := BindHeaders(req, &bound); err != nil {
				t.Fatalf("BindHeaders() error = %v", err)
			}
			if bound.RequestID != "req-123" {
				t.Fatalf("BindHeaders() request_id = %q", bound.RequestID)
			}
		})
	}
}

// Header 标量字段重复出现时返回重复值错误。
func TestBindHeaders_RejectsRepeatedScalar(t *testing.T) {
	type request struct {
		RequestID string `header:"x-request-id"`
	}

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Add("X-Request-Id", "req-1")
	req.Header.Add("X-Request-Id", "req-2")

	var dst request
	violation := assertSingleViolation(t, BindHeaders(req, &dst))
	if violation.Field != "X-Request-Id" || violation.In != ViolationInHeader || violation.Code != ViolationCodeMultiple || violation.Detail != "must not be repeated" {
		t.Fatalf("violation = %#v", violation)
	}
}

// 请求级校验错误优先使用请求标签中的字段名。
func TestBindAndValidate_UsesRequestTagNames(t *testing.T) {
	type request struct {
		UUID string `param:"uuid" validate:"required"`
	}

	req := httptest.NewRequest(http.MethodGet, "/items", nil)

	var bound request
	violation := assertSingleViolation(t, BindAndValidate(req, &bound))
	if violation.Field != "uuid" || violation.Code != ViolationCodeRequired {
		t.Fatalf("BindAndValidate() violation = %#v", violation)
	}
}

// 顶层 Bind 会直接返回各阶段产生的错误，而不会吞掉或改写来源语义。
func TestBind_PropagatesStageErrors(t *testing.T) {
	t.Run("query binding error on get", func(t *testing.T) {
		type request struct {
			Page int `query:"page"`
		}

		req := httptest.NewRequest(http.MethodGet, "/items?page=oops", nil)

		var bound request
		violation := assertSingleViolation(t, Bind(req, &bound))
		if violation.Field != "page" || violation.Code != ViolationCodeType || violation.Detail != "must be number" {
			t.Fatalf("violation = %#v", violation)
		}
	})

	t.Run("unsupported body content type when body present", func(t *testing.T) {
		type request struct {
			Name string `json:"name"`
		}

		req := httptest.NewRequest(http.MethodPost, "/items", strings.NewReader(`{"name":"kanata"}`))
		req.Header.Set("Content-Type", "text/plain")

		var bound request
		err := Bind(req, &bound)
		_ = assertHTTPError(t, err, http.StatusUnsupportedMediaType, CodeUnsupportedMediaType, "Content-Type must be application/json or application/*+json")
	})
}
