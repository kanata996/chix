// 本文件职责：定义 runtime 的公开类型。
// 定位：作为 v1 runtime 的类型入口，不承载内部配置与执行期骨架。
package runtime

import (
	"context"
	"net/http"
)

// Runtime 表示一组可继承的 runtime 配置作用域，并可继续派生子作用域。
type Runtime struct {
	parent *Runtime
	local  scopeConfig
}

// Operation 描述单个挂载点的成功状态码和 operation 级错误映射策略。
type Operation struct {
	SuccessStatus int
	ErrorMappers  []ErrorMapper
}

// Handler 表示业务处理函数；输入绑定与响应写回由 runtime 负责，业务只返回输出或错误。
type Handler[I any, O any] func(context.Context, *I) (*O, error)

// Event 表示失败路径上的边界观测事件，包含原始错误、公开错误以及提取出的请求上下文。
type Event struct {
	Error      error
	Public     *HTTPError
	RequestID  string
	Method     string
	Target     string
	RemoteAddr string
}

// Observer 处理 runtime 在失败路径上发出的边界观测事件。
type Observer func(Event)

// Extractor 从 HTTP 请求中提取用于观测的请求上下文。
type Extractor func(*http.Request) RequestContext

// RequestContext 表示从请求中提取出的最小观测上下文；缺失值保持零值。
type RequestContext struct {
	RequestID  string
	Method     string
	Target     string
	RemoteAddr string
}
