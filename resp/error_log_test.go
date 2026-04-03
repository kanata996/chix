package resp

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/go-chi/chi/v5"
)

type cycleTestError struct{}

type multiUnwrapTestError struct {
	errs []error
}

func (e *cycleTestError) Error() string {
	return "cycle"
}

func (e *cycleTestError) Unwrap() error {
	return e
}

func (e *multiUnwrapTestError) Error() string {
	return "multi"
}

func (e *multiUnwrapTestError) Unwrap() []error {
	return e.errs
}

// 错误链摘要会在 nil 输入时返回零值，并对多层包装提取首尾信息。
func TestBuildErrorChainInfo(t *testing.T) {
	if got := buildErrorChainInfo(nil); got.message != "" || got.errorType != "" || got.rootMessage != "" || got.rootType != "" || len(got.chain) != 0 || len(got.typeChain) != 0 || got.wrapped {
		t.Fatalf("buildErrorChainInfo(nil) = %#v, want zero value fields", got)
	}

	root := errors.New("db timeout")
	info := buildErrorChainInfo(fmt.Errorf("load user: %w", root))
	if got := info.message; got != "load user: db timeout" {
		t.Fatalf("message = %q, want wrapped message", got)
	}
	if got := info.rootMessage; got != "db timeout" {
		t.Fatalf("rootMessage = %q, want db timeout", got)
	}
	if got := info.rootType; got != "*errors.errorString" {
		t.Fatalf("rootType = %q, want *errors.errorString", got)
	}
	if !info.wrapped {
		t.Fatal("wrapped = false, want true")
	}
	if len(info.chain) != 2 {
		t.Fatalf("chain = %#v, want len 2", info.chain)
	}
}

// 错误链展开会兼容 nil、深度限制、errors.Join 和循环引用。
func TestFlattenErrorChain(t *testing.T) {
	if got := flattenErrorChain(nil, maxLoggedErrorChainDepth); got != nil {
		t.Fatalf("flattenErrorChain(nil) = %#v, want nil", got)
	}
	if got := flattenErrorChain(errors.New("x"), 0); got != nil {
		t.Fatalf("flattenErrorChain(limit=0) = %#v, want nil", got)
	}

	joined := errors.Join(errors.New("a"), fmt.Errorf("wrap: %w", errors.New("b")))
	if got := flattenErrorChain(joined, 2); len(got) != 2 {
		t.Fatalf("flattenErrorChain(joined, 2) len = %d, want 2", len(got))
	}

	cycle := &cycleTestError{}
	if got := flattenErrorChain(cycle, maxLoggedErrorChainDepth); len(got) != 1 {
		t.Fatalf("flattenErrorChain(cycle) len = %d, want 1", len(got))
	}

	multi := &multiUnwrapTestError{errs: []error{nil, errors.New("left"), errors.New("right")}}
	if got := flattenErrorChain(multi, maxLoggedErrorChainDepth); len(got) != 3 {
		t.Fatalf("flattenErrorChain(multi) len = %d, want 3", len(got))
	}
}

// unwrapErrors 会统一兼容单个 unwrap、多分支 join 和无下级错误三种场景。
func TestUnwrapErrors(t *testing.T) {
	if got := unwrapErrors(nil); got != nil {
		t.Fatalf("unwrapErrors(nil) = %#v, want nil", got)
	}

	root := errors.New("root")
	single := fmt.Errorf("wrap: %w", root)
	gotSingle := unwrapErrors(single)
	if len(gotSingle) != 1 || !errors.Is(gotSingle[0], root) {
		t.Fatalf("unwrapErrors(single) = %#v, want [root]", gotSingle)
	}

	left := errors.New("left")
	right := errors.New("right")
	gotMulti := unwrapErrors(errors.Join(left, right))
	if len(gotMulti) != 2 || !errors.Is(gotMulti[0], left) || !errors.Is(gotMulti[1], right) {
		t.Fatalf("unwrapErrors(join) = %#v, want [left right]", gotMulti)
	}
}

// 请求日志属性提取会在 nil 输入时返回空，并对 5xx 错误补充诊断字段。
func TestRequestErrorLogAttrs(t *testing.T) {
	if got := requestErrorLogAttrs(nil, nil, nil); got != nil {
		t.Fatalf("requestErrorLogAttrs(nil, nil, nil) = %#v, want nil", got)
	}

	req := newRequestWithRoute(t, http.MethodGet, "/users/{id}", "/users/u_1")
	httpErr := wrapError(http.StatusInternalServerError, "internal_error", "", errors.New("db timeout"))
	attrs := requestErrorLogAttrs(req, httpErr, httpErr)
	if len(attrs) == 0 {
		t.Fatal("requestErrorLogAttrs() = nil, want attrs")
	}

	values := attrsToMap(attrs)
	if got := values["error.code"]; got != "internal_error" {
		t.Fatalf("error.code = %#v, want internal_error", got)
	}
	if got := values["error.root_message"]; got != "db timeout" {
		t.Fatalf("error.root_message = %#v, want db timeout", got)
	}

	clientErr := BadRequest("bad_request", "bad request")
	if got := requestErrorLogAttrs(req, clientErr, clientErr); got != nil {
		t.Fatalf("requestErrorLogAttrs(4xx) = %#v, want nil", got)
	}
}

// 错误类型名和错误文本裁剪都会对 nil、空白和超长输入给出稳定结果。
func TestErrorTypeNameAndLimitErrorLogString(t *testing.T) {
	if got := errorTypeName(nil); got != "" {
		t.Fatalf("errorTypeName(nil) = %q, want empty", got)
	}

	var typedNil error = (*rawTestError)(nil)
	if got := errorTypeName(typedNil); got != "*resp.rawTestError" {
		t.Fatalf("errorTypeName(typed nil) = %q, want *resp.rawTestError", got)
	}

	if got := limitErrorLogString("   "); got != "" {
		t.Fatalf("limitErrorLogString(blank) = %q, want empty", got)
	}
	if got := limitErrorLogString("  keep me  "); got != "keep me" {
		t.Fatalf("limitErrorLogString(trim) = %q, want keep me", got)
	}

	long := strings.Repeat("a", maxLoggedErrorStringBytes+1)
	got := limitErrorLogString(long)
	if !strings.HasSuffix(got, "...(truncated)") {
		t.Fatalf("limitErrorLogString(long) = %q, want truncated suffix", got)
	}
}

func attrsToMap(attrs []slog.Attr) map[string]any {
	out := make(map[string]any, len(attrs))
	for _, attr := range attrs {
		out[attr.Key] = attr.Value.Any()
	}
	return out
}

func newRequestWithRoute(t *testing.T, method, routePattern, target string) *http.Request {
	t.Helper()

	req := httptest.NewRequest(method, target, nil)
	routeCtx := chi.NewRouteContext()
	routeCtx.RoutePatterns = []string{routePattern}
	return req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, routeCtx))
}
