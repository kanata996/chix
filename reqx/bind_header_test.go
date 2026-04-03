package reqx

// 用例清单：
// - 标记说明：[✓] 已核对且已有真实覆盖；[x] 本轮审查发现缺口后补测。
// - [✓] `BindHeaders` 会绑定常见标量 header，并对空请求参数做明确校验。
// - [✓] `BindAndValidateHeaders` 会在校验前执行 `Normalize()`。
// - [✓] `BindAndValidateHeaders` 的校验错误字段名使用规范化后的 header tag 名称。

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// Header 绑定应支持常见的标量字段类型。
func TestBindHeaders_BindsSupportedScalarTypes(t *testing.T) {
	type request struct {
		RequestID string `header:"x-request-id"`
		Retry     int    `header:"x-retry"`
		Enabled   bool   `header:"x-enabled"`
	}

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("X-Request-Id", "req-123")
	req.Header.Set("X-Retry", "2")
	req.Header.Set("X-Enabled", "true")

	var dst request
	if err := BindHeaders(req, &dst); err != nil {
		t.Fatalf("BindHeaders() error = %v", err)
	}
	if dst.RequestID != "req-123" || dst.Retry != 2 || !dst.Enabled {
		t.Fatalf("dst = %#v, want bound scalar header values", dst)
	}
}

// 空输入会在进入 header 绑定前被直接拒绝。
func TestBindHeaders_RejectsNilInputs(t *testing.T) {
	t.Run("nil request", func(t *testing.T) {
		var dst struct{}
		err := BindHeaders[struct{}](nil, &dst)
		if err == nil {
			t.Fatal("BindHeaders() error = nil")
		}
		if got := err.Error(); got != "reqx: request must not be nil" {
			t.Fatalf("error = %q", got)
		}
	})

	t.Run("nil destination", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/", nil)

		err := BindHeaders[struct{}](req, nil)
		if err == nil {
			t.Fatal("BindHeaders() error = nil")
		}
		if got := err.Error(); got != "reqx: destination must not be nil" {
			t.Fatalf("error = %q", got)
		}
	})
}

// 绑定后会先做标准化再进入 header 校验。
func TestBindAndValidateHeaders_NormalizesBeforeValidation(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("X-Request-Id", " req-123 ")

	var dst normalizedHeaderRequest
	if err := BindAndValidateHeaders(req, &dst); err != nil {
		t.Fatalf("BindAndValidateHeaders() error = %v", err)
	}
	if dst.RequestID != "req-123" {
		t.Fatalf("request id = %q, want req-123", dst.RequestID)
	}
}

// Header 校验错误字段名应使用规范化后的 header tag 名称。
func TestBindAndValidateHeaders_UsesCanonicalHeaderTagName(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/", nil)

	var dst struct {
		RequestID string `header:"x-request-id" validate:"required"`
	}
	err := BindAndValidateHeaders(req, &dst)
	violation := assertSingleViolation(t, err)
	if violation.Field != "X-Request-Id" || violation.In != ViolationInHeader || violation.Code != ViolationCodeRequired || violation.Detail != "is required" {
		t.Fatalf("violation = %#v", violation)
	}
}

type normalizedHeaderRequest struct {
	RequestID string `header:"x-request-id" validate:"required,nospace"`
}

func (r *normalizedHeaderRequest) Normalize() {
	r.RequestID = strings.TrimSpace(r.RequestID)
}
