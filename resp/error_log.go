// Package resp 的本文件负责“错误请求日志注解”，而不是“统一错误日志输出”。
//
// 定位：
//   - 这里服务于 WriteError(...) 的内部流程。
//   - 它只负责把已收敛好的 HTTP 错误语义补充到当前 request log。
//   - 真正输出请求日志的仍然是外层 httplog，中间件层不重复实现 error -> HTTP 语义映射。
//
// 职责：
//   - 从 error / HTTPError 提取更适合排障的结构化字段。
//   - 在不泄露不可控内部对象的前提下，尽量保留原始错误文本、类型、错误链和安全细节。
//   - 仅在“错误响应自身写出失败”这类基础设施异常时，额外输出一条独立 error 日志。
//
// 要点：
//   - 普通 4xx / 5xx 不在这里额外打一条重复业务错误日志。
//   - 诊断字段优先围绕排障，而不是简单镜像对外响应 JSON。
//   - 诊断链优先从原始 cause 开始，避免 *HTTPError 包装层淹没真正根因。
package resp

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"reflect"
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/go-chi/httplog/v3"
	chixmw "github.com/kanata996/chix/middleware"
)

const (
	maxLoggedErrorChainDepth  = 8
	maxLoggedErrorStringBytes = 1024
	maxLoggedErrorDetailsSize = 4096
)

type errorChainInfo struct {
	message     string
	errorType   string
	rootMessage string
	rootType    string
	chain       []string
	typeChain   []string
	wrapped     bool
}

// annotateRequestErrorLog 把错误诊断字段挂到当前请求日志上下文。
// 这里不直接输出日志，只是补充 attrs，最终仍由外层 httplog 统一落日志。
func annotateRequestErrorLog(r *http.Request, err error, httpErr *HTTPError) {
	if r == nil || err == nil || httpErr == nil {
		return
	}

	attrs := requestErrorLogAttrs(r, err, httpErr)
	if len(attrs) == 0 {
		return
	}
	httplog.SetAttrs(r.Context(), attrs...)
}

// requestErrorLogAttrs 生成请求级错误日志字段。
// 4xx 仅保留外层请求日志，不再补内部错误诊断；5xx 才补充排障字段。
func requestErrorLogAttrs(r *http.Request, err error, httpErr *HTTPError) []slog.Attr {
	if err == nil || httpErr == nil {
		return nil
	}

	status := httpErr.Status()
	if status < http.StatusInternalServerError {
		return nil
	}

	chain := buildErrorChainInfo(errorForDiagnostics(err, httpErr))
	attrs := make([]slog.Attr, 0, 7)
	attrs = append(attrs, slog.String("error.code", httpErr.Code()))
	if chain.message != "" {
		attrs = append(attrs, slog.String("error.message", chain.message))
	}
	if chain.errorType != "" {
		attrs = append(attrs, slog.String("error.type", chain.errorType))
	}

	if chain.rootMessage != "" {
		attrs = append(attrs, slog.String("error.root_message", chain.rootMessage))
	}
	if chain.rootType != "" {
		attrs = append(attrs, slog.String("error.root_type", chain.rootType))
	}
	if errors.Is(err, context.Canceled) {
		attrs = append(attrs, slog.Bool("error.canceled", true))
	}
	if errors.Is(err, context.DeadlineExceeded) {
		attrs = append(attrs, slog.Bool("error.timeout", true))
	}
	if r != nil && !chixmw.HasBaseRequestLogAttrs(r.Context()) {
		attrs = append(attrs, chixmw.BaseRequestLogAttrs(r)...)
	}

	return attrs
}

// errorForDiagnostics 返回用于日志诊断的起始 error。
// 如果 HTTPError 已包住原始 cause，则优先从 cause 开始展开错误链，
// 避免把 *HTTPError 本身误当成主要错误类型。
func errorForDiagnostics(err error, httpErr *HTTPError) error {
	if httpErr != nil && httpErr.cause != nil {
		return httpErr.cause
	}
	return err
}

// logErrorResponseWriteFailure 只记录“错误响应自身写出失败”的异常。
// 这是基础设施级问题，不属于普通业务失败，因此需要单独打一条 error 日志。
func logErrorResponseWriteFailure(r *http.Request, httpErr *HTTPError, err error) {
	if err == nil || httpErr == nil {
		return
	}

	ctx := context.Background()
	logger := slog.Default()
	if r != nil {
		ctx = r.Context()
		if ctxLogger := chixmw.LoggerFromContext(ctx); ctxLogger != nil {
			logger = ctxLogger
		}
	}

	attrs := []any{
		slog.Int("http.response.status_code", httpErr.Status()),
		slog.String("error.code", httpErr.Code()),
		slog.Any("error", err),
	}

	var degraded *ErrorWriteDegraded
	if errors.As(err, &degraded) && degraded != nil {
		attrs = append(attrs,
			slog.Bool("resp.error_degraded", true),
			slog.Bool("resp.public_response_preserved", degraded.PreservedPublicResponse),
		)
	}

	logger.ErrorContext(ctx, "resp: failed to write error response", attrs...)
}

// buildErrorChainInfo 把错误链整理成适合日志输出的摘要结构。
// 它会同时保留当前错误、根错误以及整条错误链的 message / type 信息。
func buildErrorChainInfo(err error) errorChainInfo {
	chain := flattenErrorChain(err, maxLoggedErrorChainDepth)
	if len(chain) == 0 {
		return errorChainInfo{}
	}

	info := errorChainInfo{
		chain:     make([]string, 0, len(chain)),
		typeChain: make([]string, 0, len(chain)),
		wrapped:   len(chain) > 1,
	}
	for _, item := range chain {
		message := limitErrorLogString(item.Error())
		errType := errorTypeName(item)
		if message != "" {
			if info.message == "" {
				info.message = message
			}
			info.rootMessage = message
			info.chain = append(info.chain, message)
		}
		if errType != "" {
			if info.errorType == "" {
				info.errorType = errType
			}
			info.rootType = errType
			info.typeChain = append(info.typeChain, errType)
		}
	}

	return info
}

// flattenErrorChain 按 Unwrap 语义展开错误链，并限制深度、防止循环。
// 这样可以兼容标准单链 unwrap，也兼容 errors.Join 形成的多分支链路。
func flattenErrorChain(err error, limit int) []error {
	if err == nil || limit <= 0 {
		return nil
	}

	initialCap := limit
	if initialCap > 4 {
		initialCap = 4
	}
	seen := make(map[error]struct{}, initialCap)
	chain := make([]error, 0, initialCap)
	stack := make([]error, 1, initialCap)
	stack[0] = err

	for len(stack) > 0 && len(chain) < limit {
		last := len(stack) - 1
		current := stack[last]
		stack = stack[:last]
		if current == nil {
			continue
		}
		if _, ok := seen[current]; ok {
			continue
		}
		seen[current] = struct{}{}
		chain = append(chain, current)

		unwrapped := unwrapErrors(current)
		for i := len(unwrapped) - 1; i >= 0; i-- {
			if len(chain)+len(stack) >= limit && i == 0 {
				// Keep stack growth bounded when we've already hit the limit envelope.
				break
			}
			stack = append(stack, unwrapped[i])
		}
	}

	return chain
}

// unwrapErrors 统一兼容单个 Unwrap() error 和多个 Unwrap() []error 的错误类型。
func unwrapErrors(err error) []error {
	type multiUnwrapper interface {
		Unwrap() []error
	}

	switch current := err.(type) {
	case multiUnwrapper:
		return current.Unwrap()
	default:
		if next := errors.Unwrap(err); next != nil {
			return []error{next}
		}
		return nil
	}
}

// safeErrorLogDetails 尝试把响应 details 收敛为“可安全写日志”的结构化值。
// 只有当 details 可 JSON 编码且大小受控时才落日志，否则宁可丢弃并打 dropped 标记。
func safeErrorLogDetails(details []any) (any, bool) {
	if len(details) == 0 {
		return nil, false
	}

	budget := maxLoggedErrorDetailsSize
	safeDetails, ok, handled := sanitizeErrorLogSlice(details, &budget)
	if handled {
		if !ok {
			return nil, false
		}
		return safeDetails, true
	}

	return safeErrorLogDetailsFallback(details)
}

func safeErrorLogDetailsFallback(details []any) (any, bool) {
	body, err := json.Marshal(normalizeDetails(details))
	if err != nil || len(body) > maxLoggedErrorDetailsSize {
		return nil, false
	}

	var decoded any
	if err := json.Unmarshal(body, &decoded); err != nil {
		return nil, false
	}
	return decoded, true
}

func sanitizeErrorLogSlice(values []any, budget *int) ([]any, bool, bool) {
	if !consumeErrorLogBudget(budget, 2) {
		return nil, false, true
	}

	out := make([]any, 0, len(values))
	for i, value := range values {
		if i > 0 && !consumeErrorLogBudget(budget, 1) {
			return nil, false, true
		}
		safeValue, ok, handled := sanitizeErrorLogValue(value, budget)
		if !handled {
			return nil, false, false
		}
		if !ok {
			return nil, false, true
		}
		out = append(out, safeValue)
	}
	return out, true, true
}

func sanitizeErrorLogMap(values map[string]any, budget *int) (map[string]any, bool, bool) {
	if !consumeErrorLogBudget(budget, 2) {
		return nil, false, true
	}

	out := make(map[string]any, len(values))
	index := 0
	for key, value := range values {
		if index > 0 && !consumeErrorLogBudget(budget, 1) {
			return nil, false, true
		}
		index++
		if !consumeErrorLogBudget(budget, jsonStringSize(key)+1) {
			return nil, false, true
		}

		safeValue, ok, handled := sanitizeErrorLogValue(value, budget)
		if !handled {
			return nil, false, false
		}
		if !ok {
			return nil, false, true
		}
		out[key] = safeValue
	}
	return out, true, true
}

func sanitizeErrorLogValue(value any, budget *int) (any, bool, bool) {
	switch current := value.(type) {
	case nil:
		return nil, consumeErrorLogBudget(budget, 4), true
	case string:
		return current, consumeErrorLogBudget(budget, jsonStringSize(current)), true
	case bool:
		if current {
			return current, consumeErrorLogBudget(budget, 4), true
		}
		return current, consumeErrorLogBudget(budget, 5), true
	case int:
		return current, consumeErrorLogBudget(budget, len(strconv.FormatInt(int64(current), 10))), true
	case int8:
		return current, consumeErrorLogBudget(budget, len(strconv.FormatInt(int64(current), 10))), true
	case int16:
		return current, consumeErrorLogBudget(budget, len(strconv.FormatInt(int64(current), 10))), true
	case int32:
		return current, consumeErrorLogBudget(budget, len(strconv.FormatInt(int64(current), 10))), true
	case int64:
		return current, consumeErrorLogBudget(budget, len(strconv.FormatInt(current, 10))), true
	case uint:
		return current, consumeErrorLogBudget(budget, len(strconv.FormatUint(uint64(current), 10))), true
	case uint8:
		return current, consumeErrorLogBudget(budget, len(strconv.FormatUint(uint64(current), 10))), true
	case uint16:
		return current, consumeErrorLogBudget(budget, len(strconv.FormatUint(uint64(current), 10))), true
	case uint32:
		return current, consumeErrorLogBudget(budget, len(strconv.FormatUint(uint64(current), 10))), true
	case uint64:
		return current, consumeErrorLogBudget(budget, len(strconv.FormatUint(current, 10))), true
	case float32:
		return current, consumeErrorLogBudget(budget, len(strconv.FormatFloat(float64(current), 'g', -1, 32))), true
	case float64:
		return current, consumeErrorLogBudget(budget, len(strconv.FormatFloat(current, 'g', -1, 64))), true
	case json.Number:
		return current, consumeErrorLogBudget(budget, len(current.String())), true
	case []any:
		safeSlice, ok, handled := sanitizeErrorLogSlice(current, budget)
		return safeSlice, ok, handled
	case map[string]any:
		safeMap, ok, handled := sanitizeErrorLogMap(current, budget)
		return safeMap, ok, handled
	default:
		return nil, false, false
	}
}

func consumeErrorLogBudget(budget *int, cost int) bool {
	if budget == nil || cost < 0 {
		return false
	}
	if *budget < cost {
		return false
	}
	*budget -= cost
	return true
}

func jsonStringSize(value string) int {
	size := 2
	for len(value) > 0 {
		if value[0] < utf8.RuneSelf {
			switch value[0] {
			case '\\', '"', '\b', '\f', '\n', '\r', '\t':
				size += 2
			case '<', '>', '&':
				size += 6
			default:
				if value[0] < 0x20 {
					size += 6
				} else {
					size++
				}
			}
			value = value[1:]
			continue
		}

		r, width := utf8.DecodeRuneInString(value)
		if r == utf8.RuneError && width == 1 {
			size += 6
			value = value[1:]
			continue
		}
		if r == '\u2028' || r == '\u2029' {
			size += 6
		} else {
			size += width
		}
		value = value[width:]
	}
	return size
}

// errorTypeName 返回 error 的 Go 运行时类型名，便于按类型聚合和检索。
func errorTypeName(err error) string {
	if err == nil {
		return ""
	}
	t := reflect.TypeOf(err)
	if t == nil {
		return ""
	}
	return t.String()
}

// limitErrorLogString 对错误文本做长度限制，避免单条日志过大。
func limitErrorLogString(value string) string {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return ""
	}
	if len(trimmed) <= maxLoggedErrorStringBytes {
		return trimmed
	}
	return trimmed[:maxLoggedErrorStringBytes] + "...(truncated)"
}
