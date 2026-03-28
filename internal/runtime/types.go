// 本文件职责：定义 runtime core 的公开类型和内部配置骨架。
// 定位：作为 v1 runtime 的类型入口，只承载结构定义，不放执行流程细节。
package runtime

import (
	"context"
	"net/http"
)

type Runtime struct {
	parent *Runtime
	local  scopeConfig
}

type scopeConfig struct {
	errorMappers     []ErrorMapper
	observer         Observer
	hasObserver      bool
	extractor        Extractor
	hasExtractor     bool
	successStatus    int
	hasSuccessStatus bool
}

type executionConfig struct {
	errorMappers  []ErrorMapper
	observer      Observer
	extractor     Extractor
	successStatus int
}

type resolvedFailure struct {
	raw    error
	public *HTTPError
}

type Operation[I any, O any] struct {
	Method        string
	SuccessStatus int
	ErrorMappers  []ErrorMapper
}

type Handler[I any, O any] func(context.Context, *I) (*O, error)

type Event struct {
	Error      error
	Public     *HTTPError
	RequestID  string
	Method     string
	Target     string
	RemoteAddr string
}

type Observer func(Event)

type Extractor func(*http.Request) RequestContext

type RequestContext struct {
	RequestID  string
	Method     string
	Target     string
	RemoteAddr string
}
