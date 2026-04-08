package bind

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"reflect"
	"testing"

	echo "github.com/labstack/echo/v5"
)

// 测试清单：
// - 标记说明：[✓] 已核对且已有真实覆盖；[x] 尚未完成，不得作为验收依据。
// - [✓] `sharedEcho` / `newContext` 会复用共享 Echo 实例，并在 nil request 下构造可工作的兜底上下文。
// - [✓] `pathValuesFromRequest` / `pathWildcardNames` 能按 pattern 顺序提取 path 名称，覆盖普通占位、正则后缀、ellipsis 和空 pattern。
// - [✓] `validateRequestAndTarget` 对 request/target 的 nil、非指针、nil pointer 场景做稳定校验。
// - [✓] `mapEchoError` 能处理 nil、Echo `BindingError`、Echo `HTTPError(415)` 和普通 passthrough 错误。
// - [✓] `errorsf` 统一为内部错误文本加上 `bind:` 前缀。
func TestSharedEchoReturnsSingleton(t *testing.T) {
	first := sharedEcho()
	second := sharedEcho()
	if first != second {
		t.Fatal("sharedEcho() returned different instances")
	}
}

func TestNewContextWithNilRequestUsesFallbackRequest(t *testing.T) {
	ctx := newContext(nil)

	if ctx.Request() == nil {
		t.Fatal("ctx.Request() = nil")
	}
	if ctx.Request().Method != http.MethodGet {
		t.Fatalf("method = %q, want %q", ctx.Request().Method, http.MethodGet)
	}
	if ctx.Request().URL == nil || ctx.Request().URL.Path != "/" {
		t.Fatalf("url path = %v, want /", ctx.Request().URL)
	}
	if got := ctx.PathValues(); len(got) != 0 {
		t.Fatalf("path values = %#v, want empty", got)
	}
}

func TestPathValuesFromRequestUsesPatternOrderAndNames(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/teams/acme/docs/42", nil)
	req.Pattern = "/teams/{org_id}/{slug...}/{id:[0-9]+}/{$}"
	req.SetPathValue("org_id", "acme")
	req.SetPathValue("slug", "docs")
	req.SetPathValue("id", "42")

	got := pathValuesFromRequest(req)
	if len(got) != 3 {
		t.Fatalf("len(got) = %d, want 3", len(got))
	}
	wantNames := []string{"org_id", "slug", "id"}
	wantValues := []string{"acme", "docs", "42"}
	for i := range got {
		if got[i].Name != wantNames[i] || got[i].Value != wantValues[i] {
			t.Fatalf("got[%d] = (%q, %q), want (%q, %q)", i, got[i].Name, got[i].Value, wantNames[i], wantValues[i])
		}
	}
}

func TestPathValuesFromRequestWithoutPatternReturnsEmpty(t *testing.T) {
	if got := pathValuesFromRequest(nil); len(got) != 0 {
		t.Fatalf("pathValuesFromRequest(nil) = %#v, want empty", got)
	}

	req := httptest.NewRequest(http.MethodGet, "/teams/acme", nil)
	if got := pathValuesFromRequest(req); len(got) != 0 {
		t.Fatalf("pathValuesFromRequest(no pattern) = %#v, want empty", got)
	}
}

func TestPathWildcardNames(t *testing.T) {
	tests := []struct {
		name    string
		pattern string
		want    []string
	}{
		{name: "empty", pattern: "", want: nil},
		{name: "whitespace", pattern: "   ", want: nil},
		{name: "malformed", pattern: "/items/{id", want: []string{}},
		{name: "plain regex and ellipsis", pattern: "/teams/{org_id}/{id:[0-9]+}/{slug...}/{$}", want: []string{"org_id", "id", "slug"}},
		{name: "trim spaces in token", pattern: "/{ id }/{ slug:[a-z]+ }", want: []string{"id", "slug"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := pathWildcardNames(tt.pattern); !reflect.DeepEqual(got, tt.want) {
				t.Fatalf("got = %#v, want %#v", got, tt.want)
			}
		})
	}
}

func TestValidateRequestAndTarget(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/", nil)

	var nilPtr *struct{}
	tests := []struct {
		name    string
		request *http.Request
		target  any
		want    string
	}{
		{name: "nil request", request: nil, target: &struct{}{}, want: "bind: request must not be nil"},
		{name: "nil target", request: req, target: nil, want: "bind: target must not be nil"},
		{name: "nil pointer", request: req, target: nilPtr, want: "bind: target must be a non-nil pointer"},
		{name: "non pointer", request: req, target: struct{}{}, want: "bind: target must be a non-nil pointer"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateRequestAndTarget(tt.request, tt.target)
			if err == nil || err.Error() != tt.want {
				t.Fatalf("error = %v, want %q", err, tt.want)
			}
		})
	}

	if err := validateRequestAndTarget(req, &struct{}{}); err != nil {
		t.Fatalf("validateRequestAndTarget() error = %v, want nil", err)
	}
}

func TestMapEchoError(t *testing.T) {
	t.Run("nil", func(t *testing.T) {
		if err := mapEchoError(nil); err != nil {
			t.Fatalf("mapEchoError(nil) = %v, want nil", err)
		}
	})

	t.Run("unsupported media type from runtime echo error", func(t *testing.T) {
		cause := errors.New("content type mismatch")
		mapped := mapEchoError(echo.ErrUnsupportedMediaType.Wrap(cause))
		assertHTTPError(t, mapped, http.StatusUnsupportedMediaType, CodeUnsupportedMediaType, "Unsupported Media Type")
		if !errors.Is(mapped, cause) {
			t.Fatalf("errors.Is(mapped, cause) = false, want true")
		}
	})

	t.Run("binding error", func(t *testing.T) {
		cause := errors.New("invalid syntax")
		mapped := mapEchoError(echo.NewBindingError("page", []string{"oops"}, "query param", cause))

		assertBindingError(t, mapped, "page", []string{"oops"}, "query param")
		if !errors.Is(mapped, cause) {
			t.Fatalf("errors.Is(mapped, cause) = false, want true")
		}
	})

	t.Run("passthrough", func(t *testing.T) {
		cause := errors.New("plain error")
		if got := mapEchoError(cause); got != cause {
			t.Fatalf("got = %v, want same error", got)
		}
	})
}

func TestErrorsfPrefixesMessages(t *testing.T) {
	err := errorsf("bad %s %d", "input", 3)
	if err == nil || err.Error() != "bind: bad input 3" {
		t.Fatalf("error = %v, want %q", err, "bind: bad input 3")
	}
}
