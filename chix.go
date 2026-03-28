// 本文件职责：作为根包对外暴露 runtime 的公开 API。
// 定位：根包只保留稳定入口；具体执行、failure、observation 实现都下沉到 internal/runtime。
package chix

import (
	"log/slog"
	"net/http"

	"github.com/kanata996/chix/internal/runtime"
)

type Runtime = runtime.Runtime

type Operation[I any, O any] = runtime.Operation[I, O]
type Handler[I any, O any] = runtime.Handler[I, O]

type Event = runtime.Event
type Observer = runtime.Observer
type Extractor = runtime.Extractor
type RequestContext = runtime.RequestContext

type HTTPError = runtime.HTTPError
type ErrorMapper = runtime.ErrorMapper

type Option = runtime.Option

// New 返回应用给定选项后的运行时实例，并默认启用请求上下文提取器。
func New(opts ...Option) *Runtime {
	return runtime.New(opts...)
}

// Handle 将操作与处理函数包装为 HTTP Handler，并在构建阶段解析输入 schema；rt、h 为空或 schema 解析失败时会 panic。
func Handle[I any, O any](rt *Runtime, op Operation[I, O], h Handler[I, O]) http.Handler {
	return runtime.Handle(rt, op, h)
}

// DefaultExtractor 从 HTTP 请求提取默认请求上下文，包含请求 ID、方法、目标地址和远端地址。
func DefaultExtractor(r *http.Request) RequestContext {
	return runtime.DefaultExtractor(r)
}

// DefaultLogger 基于给定的 slog Logger 创建默认观察器；logger 为空时使用 slog.Default。
func DefaultLogger(logger *slog.Logger) Observer {
	return runtime.DefaultLogger(logger)
}

// WithErrorMapper 追加错误映射器。
func WithErrorMapper(mapper ErrorMapper) Option {
	return runtime.WithErrorMapper(mapper)
}

// WithObserver 设置观察器。
func WithObserver(observer Observer) Option {
	return runtime.WithObserver(observer)
}

// WithExtractor 设置请求上下文提取器。
func WithExtractor(extractor Extractor) Option {
	return runtime.WithExtractor(extractor)
}

// WithSuccessStatus 设置成功响应的 HTTP 状态码；status 小于等于 0 时会 panic。
func WithSuccessStatus(status int) Option {
	return runtime.WithSuccessStatus(status)
}
