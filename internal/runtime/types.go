// 本文件职责：定义 runtime 的公开类型。
// 定位：作为 v1 runtime 的类型入口，不承载内部配置与执行期骨架。
package runtime

import (
	"context"
	"net/http"
)

type Runtime struct {
	parent *Runtime
	local  scopeConfig
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
