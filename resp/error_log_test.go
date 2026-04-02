package resp

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/go-chi/chi/v5"
)

type logDetailStruct struct {
	Field string `json:"field"`
	Code  string `json:"code"`
}

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

// 已经是安全 map 结构的 details 会走快路径并保留原有键值。
func TestSafeErrorLogDetailsFastPath(t *testing.T) {
	safeDetails, ok := safeErrorLogDetails([]any{
		map[string]any{
			"field": "name",
			"code":  "required",
			"count": 2,
		},
	})
	if !ok {
		t.Fatal("safeErrorLogDetails() ok = false, want true")
	}

	items, ok := safeDetails.([]any)
	if !ok || len(items) != 1 {
		t.Fatalf("safeDetails = %#v, want one []any item", safeDetails)
	}

	detail, ok := items[0].(map[string]any)
	if !ok {
		t.Fatalf("detail = %#v, want map[string]any", items[0])
	}
	if detail["field"] != "name" || detail["code"] != "required" || detail["count"] != 2 {
		t.Fatalf("detail = %#v, want field/code/count preserved", detail)
	}
}

// 空 details 不会参与日志记录。
func TestSafeErrorLogDetailsRejectsEmptySlice(t *testing.T) {
	safeDetails, ok := safeErrorLogDetails(nil)
	if ok || safeDetails != nil {
		t.Fatalf("safeErrorLogDetails(nil) = (%#v, %v), want (nil, false)", safeDetails, ok)
	}
}

// 快路径失败时，可序列化的结构体 details 仍会被回退为安全日志对象。
func TestSafeErrorLogDetailsFallbackPreservesMarshalableStruct(t *testing.T) {
	safeDetails, ok := safeErrorLogDetails([]any{
		logDetailStruct{
			Field: "name",
			Code:  "required",
		},
	})
	if !ok {
		t.Fatal("safeErrorLogDetails() ok = false, want true")
	}

	items, ok := safeDetails.([]any)
	if !ok || len(items) != 1 {
		t.Fatalf("safeDetails = %#v, want one []any item", safeDetails)
	}

	detail, ok := items[0].(map[string]any)
	if !ok {
		t.Fatalf("detail = %#v, want map[string]any", items[0])
	}
	if detail["field"] != "name" || detail["code"] != "required" {
		t.Fatalf("detail = %#v, want field/code preserved", detail)
	}
}

// 超出日志预算的 details 会被整体丢弃，避免错误日志失控。
func TestSafeErrorLogDetailsDropsOversizedPayload(t *testing.T) {
	safeDetails, ok := safeErrorLogDetails([]any{
		map[string]any{
			"message": strings.Repeat("a", maxLoggedErrorDetailsSize),
		},
	})
	if ok || safeDetails != nil {
		t.Fatalf("safeErrorLogDetails() = (%#v, %v), want (nil, false)", safeDetails, ok)
	}
}

// 结构化 details 中的常见标量、切片和 map 会被收敛为可安全落日志的值。
func TestSafeErrorLogDetailsSanitizesStructuredValues(t *testing.T) {
	safeDetails, ok := safeErrorLogDetails([]any{
		map[string]any{
			"text":   "value",
			"flag":   true,
			"count":  json.Number("42"),
			"nested": []any{"x", false, map[string]any{"k": "v"}},
		},
	})
	if !ok {
		t.Fatal("safeErrorLogDetails() ok = false, want true")
	}

	items, ok := safeDetails.([]any)
	if !ok || len(items) != 1 {
		t.Fatalf("safeDetails = %#v, want one []any item", safeDetails)
	}

	detail, ok := items[0].(map[string]any)
	if !ok {
		t.Fatalf("detail = %#v, want map[string]any", items[0])
	}
	if got := detail["text"]; got != "value" {
		t.Fatalf("text = %#v, want value", got)
	}
	if got := detail["flag"]; got != true {
		t.Fatalf("flag = %#v, want true", got)
	}
	if got := detail["count"]; got != json.Number("42") {
		t.Fatalf("count = %#v, want json.Number(\"42\")", got)
	}

	nested, ok := detail["nested"].([]any)
	if !ok || len(nested) != 3 {
		t.Fatalf("nested = %#v, want []any len 3", detail["nested"])
	}
	if got := nested[0]; got != "x" {
		t.Fatalf("nested[0] = %#v, want x", got)
	}
	if got := nested[1]; got != false {
		t.Fatalf("nested[1] = %#v, want false", got)
	}
	if nestedMap, ok := nested[2].(map[string]any); !ok || nestedMap["k"] != "v" {
		t.Fatalf("nested[2] = %#v, want map[k:v]", nested[2])
	}
}

// 不支持的嵌套 details 会在快路径和回退路径都失败后被整体丢弃。
func TestSafeErrorLogDetailsRejectsUnsupportedNestedValue(t *testing.T) {
	safeDetails, ok := safeErrorLogDetails([]any{
		map[string]any{
			"field": "name",
			"bad":   make(chan int),
		},
	})
	if ok || safeDetails != nil {
		t.Fatalf("safeErrorLogDetails() = (%#v, %v), want (nil, false)", safeDetails, ok)
	}
}

// fallback 路径在 details 可 JSON 往返时也会返回安全的结构化值。
func TestSafeErrorLogDetailsFallbackRoundTripsJSON(t *testing.T) {
	safeDetails, ok := safeErrorLogDetailsFallback([]any{
		logDetailStruct{
			Field: "email",
			Code:  "invalid",
		},
	})
	if !ok {
		t.Fatal("safeErrorLogDetailsFallback() ok = false, want true")
	}

	items, ok := safeDetails.([]any)
	if !ok || len(items) != 1 {
		t.Fatalf("safeDetails = %#v, want one []any item", safeDetails)
	}
	detail, ok := items[0].(map[string]any)
	if !ok {
		t.Fatalf("detail = %#v, want map[string]any", items[0])
	}
	if detail["field"] != "email" || detail["code"] != "invalid" {
		t.Fatalf("detail = %#v, want field/code preserved", detail)
	}
}

// fallback 路径在 details 不可 JSON 编码时会返回 false。
func TestSafeErrorLogDetailsFallbackRejectsUnsupportedValue(t *testing.T) {
	safeDetails, ok := safeErrorLogDetailsFallback([]any{make(chan int)})
	if ok || safeDetails != nil {
		t.Fatalf("safeErrorLogDetailsFallback() = (%#v, %v), want (nil, false)", safeDetails, ok)
	}
}

// fallback 路径在 JSON 体超出大小预算时也会直接丢弃 details。
func TestSafeErrorLogDetailsFallbackRejectsOversizedPayload(t *testing.T) {
	safeDetails, ok := safeErrorLogDetailsFallback([]any{
		map[string]any{
			"message": strings.Repeat("a", maxLoggedErrorDetailsSize),
		},
	})
	if ok || safeDetails != nil {
		t.Fatalf("safeErrorLogDetailsFallback() = (%#v, %v), want (nil, false)", safeDetails, ok)
	}
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

// details 清洗会覆盖各种标量、容器和不支持类型分支。
func TestSanitizeErrorLogValueBranches(t *testing.T) {
	testCases := []struct {
		name        string
		value       any
		wantOK      bool
		wantHandled bool
	}{
		{name: "nil", value: nil, wantOK: true, wantHandled: true},
		{name: "string", value: "hello", wantOK: true, wantHandled: true},
		{name: "bool true", value: true, wantOK: true, wantHandled: true},
		{name: "bool false", value: false, wantOK: true, wantHandled: true},
		{name: "int", value: int(1), wantOK: true, wantHandled: true},
		{name: "int8", value: int8(1), wantOK: true, wantHandled: true},
		{name: "int16", value: int16(1), wantOK: true, wantHandled: true},
		{name: "int32", value: int32(1), wantOK: true, wantHandled: true},
		{name: "int64", value: int64(1), wantOK: true, wantHandled: true},
		{name: "uint", value: uint(1), wantOK: true, wantHandled: true},
		{name: "uint8", value: uint8(1), wantOK: true, wantHandled: true},
		{name: "uint16", value: uint16(1), wantOK: true, wantHandled: true},
		{name: "uint32", value: uint32(1), wantOK: true, wantHandled: true},
		{name: "uint64", value: uint64(1), wantOK: true, wantHandled: true},
		{name: "float32", value: float32(1.5), wantOK: true, wantHandled: true},
		{name: "float64", value: float64(1.5), wantOK: true, wantHandled: true},
		{name: "json number", value: json.Number("42"), wantOK: true, wantHandled: true},
		{name: "slice", value: []any{"x", 1}, wantOK: true, wantHandled: true},
		{name: "map", value: map[string]any{"k": "v"}, wantOK: true, wantHandled: true},
		{name: "unsupported", value: make(chan int), wantOK: false, wantHandled: false},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			budget := maxLoggedErrorDetailsSize
			_, ok, handled := sanitizeErrorLogValue(tc.value, &budget)
			if ok != tc.wantOK || handled != tc.wantHandled {
				t.Fatalf("sanitizeErrorLogValue(%T) = (_, %v, %v), want (_, %v, %v)", tc.value, ok, handled, tc.wantOK, tc.wantHandled)
			}
		})
	}
}

// details 容器清洗在预算不足时会明确失败，并在预算足够时保留结构。
func TestSanitizeErrorLogContainersRespectBudget(t *testing.T) {
	budget := maxLoggedErrorDetailsSize
	safeSlice, ok, handled := sanitizeErrorLogSlice([]any{"x", 1}, &budget)
	if !handled || !ok || len(safeSlice) != 2 {
		t.Fatalf("sanitizeErrorLogSlice() = (%#v, %v, %v), want success", safeSlice, ok, handled)
	}

	budget = maxLoggedErrorDetailsSize
	safeMap, ok, handled := sanitizeErrorLogMap(map[string]any{"k": "v"}, &budget)
	if !handled || !ok || safeMap["k"] != "v" {
		t.Fatalf("sanitizeErrorLogMap() = (%#v, %v, %v), want success", safeMap, ok, handled)
	}

	budget = 1
	if _, ok, handled := sanitizeErrorLogSlice([]any{"x"}, &budget); !handled || ok {
		t.Fatalf("sanitizeErrorLogSlice(low budget) = (_, %v, %v), want (_, false, true)", ok, handled)
	}

	budget = 1
	if _, ok, handled := sanitizeErrorLogMap(map[string]any{"k": "v"}, &budget); !handled || ok {
		t.Fatalf("sanitizeErrorLogMap(low budget) = (_, %v, %v), want (_, false, true)", ok, handled)
	}

	budget = 5
	if _, ok, handled := sanitizeErrorLogSlice([]any{"x", "y"}, &budget); !handled || ok {
		t.Fatalf("sanitizeErrorLogSlice(second item over budget) = (_, %v, %v), want (_, false, true)", ok, handled)
	}

	budget = 9
	if _, ok, handled := sanitizeErrorLogMap(map[string]any{"a": "x", "b": "y"}, &budget); !handled || ok {
		t.Fatalf("sanitizeErrorLogMap(second item over budget) = (_, %v, %v), want (_, false, true)", ok, handled)
	}

	budget = 3
	if _, ok, handled := sanitizeErrorLogMap(map[string]any{"a": "x"}, &budget); !handled || ok {
		t.Fatalf("sanitizeErrorLogMap(key over budget) = (_, %v, %v), want (_, false, true)", ok, handled)
	}
}

// 容器清洗遇到不支持的嵌套值时，会显式返回 handled=false 交给 fallback 处理。
func TestSanitizeErrorLogContainersReturnUnhandledForUnsupportedNestedValue(t *testing.T) {
	budget := maxLoggedErrorDetailsSize
	if _, ok, handled := sanitizeErrorLogSlice([]any{make(chan int)}, &budget); handled || ok {
		t.Fatalf("sanitizeErrorLogSlice(unsupported) = (_, %v, %v), want (_, false, false)", ok, handled)
	}

	budget = maxLoggedErrorDetailsSize
	if _, ok, handled := sanitizeErrorLogMap(map[string]any{"bad": make(chan int)}, &budget); handled || ok {
		t.Fatalf("sanitizeErrorLogMap(unsupported) = (_, %v, %v), want (_, false, false)", ok, handled)
	}
}

// 预算消耗辅助函数会拒绝 nil、负数和超额消耗，并在成功时扣减预算。
func TestConsumeErrorLogBudget(t *testing.T) {
	if consumeErrorLogBudget(nil, 1) {
		t.Fatal("consumeErrorLogBudget(nil, 1) = true, want false")
	}

	budget := 3
	if consumeErrorLogBudget(&budget, -1) {
		t.Fatal("consumeErrorLogBudget(negative cost) = true, want false")
	}
	if budget != 3 {
		t.Fatalf("budget = %d, want 3", budget)
	}

	if consumeErrorLogBudget(&budget, 4) {
		t.Fatal("consumeErrorLogBudget(too much) = true, want false")
	}
	if budget != 3 {
		t.Fatalf("budget = %d, want 3", budget)
	}

	if !consumeErrorLogBudget(&budget, 2) {
		t.Fatal("consumeErrorLogBudget(valid) = false, want true")
	}
	if budget != 1 {
		t.Fatalf("budget = %d, want 1", budget)
	}
}

// jsonStringSize 会与 encoding/json 对字符串转义后的字节长度保持一致。
func TestJSONStringSize(t *testing.T) {
	testCases := []string{
		"plain",
		"quote\"slash\\",
		"line\nbreak\t",
		"\x01",
		"<tag>&",
		"你",
		"\u2028\u2029",
		string([]byte{0xff}),
	}

	for _, tc := range testCases {
		t.Run(tc, func(t *testing.T) {
			body, err := json.Marshal(tc)
			if err != nil {
				t.Fatalf("json.Marshal(%q) error = %v", tc, err)
			}
			if got, want := jsonStringSize(tc), len(body); got != want {
				t.Fatalf("jsonStringSize(%q) = %d, want %d", tc, got, want)
			}
		})
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
