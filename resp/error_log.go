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
	"strings"

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
// 这些字段以排障为主，兼顾响应语义和可检索性，例如错误链、根因、route、request id 等。
func requestErrorLogAttrs(r *http.Request, err error, httpErr *HTTPError) []slog.Attr {
	if err == nil || httpErr == nil {
		return nil
	}

	details := httpErr.Details()
	chain := buildErrorChainInfo(errorForDiagnostics(err, httpErr))
	attrs := []slog.Attr{
		slog.String("error.code", httpErr.Code()),
		slog.String("error.category", errorLogCategory(httpErr.Status())),
		slog.String("error.message", chain.message),
		slog.String("error.public_message", httpErr.Message()),
		slog.String("error.type", chain.errorType),
		slog.Bool("error.expected", httpErr.Status() < http.StatusInternalServerError),
		slog.Int("error.details_count", len(details)),
	}

	if chain.rootMessage != "" {
		attrs = append(attrs, slog.String("error.root_message", chain.rootMessage))
	}
	if chain.rootType != "" {
		attrs = append(attrs, slog.String("error.root_type", chain.rootType))
	}
	if chain.wrapped {
		attrs = append(attrs, slog.Bool("error.wrapped", true))
	}
	if len(chain.chain) > 1 {
		attrs = append(attrs, slog.Any("error.chain", chain.chain))
	}
	if len(chain.typeChain) > 1 {
		attrs = append(attrs, slog.Any("error.chain_types", chain.typeChain))
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
	if safeDetails, ok := safeErrorLogDetails(details); ok {
		attrs = append(attrs, slog.Any("error.details", safeDetails))
	} else if len(details) > 0 {
		attrs = append(attrs, slog.Bool("error.details_dropped", true))
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

// errorLogCategory 按 HTTP 状态码把错误收敛为 client / server 两类，
// 便于日志筛选和告警维度聚合。
func errorLogCategory(status int) string {
	if status >= http.StatusInternalServerError {
		return "server"
	}
	return "client"
}

// logErrorResponseWriteFailure 只记录“错误响应自身写出失败”的异常。
// 这是基础设施级问题，不属于普通业务失败，因此需要单独打一条 error 日志。
func logErrorResponseWriteFailure(r *http.Request, httpErr *HTTPError, err error) {
	if err == nil || httpErr == nil {
		return
	}

	ctx := context.Background()
	if r != nil {
		ctx = r.Context()
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

	slog.ErrorContext(ctx, "resp: failed to write error response", attrs...)
}

// buildErrorChainInfo 把错误链整理成适合日志输出的摘要结构。
// 它会同时保留当前错误、根错误以及整条错误链的 message / type 信息。
func buildErrorChainInfo(err error) errorChainInfo {
	chain := flattenErrorChain(err, maxLoggedErrorChainDepth)
	if len(chain) == 0 {
		return errorChainInfo{}
	}

	info := errorChainInfo{
		wrapped: len(chain) > 1,
	}
	for _, item := range chain {
		message := limitErrorLogString(item.Error())
		errType := errorTypeName(item)
		if message != "" {
			info.chain = append(info.chain, message)
		}
		if errType != "" {
			info.typeChain = append(info.typeChain, errType)
		}
	}

	info.message = firstNonEmpty(info.chain...)
	info.errorType = firstNonEmpty(info.typeChain...)
	info.rootMessage = firstNonEmpty(reverseStrings(info.chain)...)
	info.rootType = firstNonEmpty(reverseStrings(info.typeChain)...)
	return info
}

// flattenErrorChain 按 Unwrap 语义展开错误链，并限制深度、防止循环。
// 这样可以兼容标准单链 unwrap，也兼容 errors.Join 形成的多分支链路。
func flattenErrorChain(err error, limit int) []error {
	if err == nil || limit <= 0 {
		return nil
	}

	seen := map[error]struct{}{}
	var chain []error
	var walk func(error)
	walk = func(current error) {
		if current == nil || len(chain) >= limit {
			return
		}
		if _, ok := seen[current]; ok {
			return
		}
		seen[current] = struct{}{}
		chain = append(chain, current)

		switch unwrapped := unwrapErrors(current); len(unwrapped) {
		case 0:
			return
		default:
			for _, next := range unwrapped {
				walk(next)
				if len(chain) >= limit {
					return
				}
			}
		}
	}

	walk(err)
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

// firstNonEmpty 返回第一个非空字符串，常用于挑选“当前值/根值”的候选项。
func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return value
		}
	}
	return ""
}

// reverseStrings 返回字符串切片的反转副本，避免原地修改调用方数据。
func reverseStrings(values []string) []string {
	if len(values) == 0 {
		return nil
	}
	out := make([]string, len(values))
	copy(out, values)
	for i := 0; i < len(out)/2; i++ {
		j := len(out) - 1 - i
		out[i], out[j] = out[j], out[i]
	}
	return out
}
