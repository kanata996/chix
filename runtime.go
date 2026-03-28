// 本文件职责：作为根包对外暴露 runtime core 的公开 API。
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

type Violation = runtime.Violation
type Validator[I any] = runtime.Validator[I]

type Event = runtime.Event
type Observer = runtime.Observer
type Extractor = runtime.Extractor
type RequestContext = runtime.RequestContext

type HTTPError = runtime.HTTPError
type ErrorMapper = runtime.ErrorMapper

type Option = runtime.Option

func New(opts ...Option) *Runtime {
	return runtime.New(opts...)
}

func Handle[I any, O any](rt *Runtime, op Operation[I, O], h Handler[I, O]) http.Handler {
	return runtime.Handle[I, O](rt, op, h)
}

func DefaultExtractor(r *http.Request) RequestContext {
	return runtime.DefaultExtractor(r)
}

func DefaultLogger(logger *slog.Logger) Observer {
	return runtime.DefaultLogger(logger)
}

func WithErrorMapper(mapper ErrorMapper) Option {
	return runtime.WithErrorMapper(mapper)
}

func WithObserver(observer Observer) Option {
	return runtime.WithObserver(observer)
}

func WithExtractor(extractor Extractor) Option {
	return runtime.WithExtractor(extractor)
}

func WithSuccessStatus(status int) Option {
	return runtime.WithSuccessStatus(status)
}
