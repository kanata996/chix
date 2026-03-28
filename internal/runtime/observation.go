// 本文件职责：放置请求上下文提取和默认日志观测器。
// 定位：服务 failure / observation 边界，不承载 scope 继承与配置解析语义。
package runtime

import (
	"log/slog"
	"net/http"

	"github.com/go-chi/chi/v5/middleware"
)

// DefaultExtractor 从 HTTP 请求中提取默认的观测上下文；请求为空时返回零值。
func DefaultExtractor(r *http.Request) RequestContext {
	if r == nil {
		return RequestContext{}
	}

	return RequestContext{
		RequestID:  middleware.GetReqID(r.Context()),
		Method:     r.Method,
		Target:     r.URL.RequestURI(),
		RemoteAddr: r.RemoteAddr,
	}
}

// DefaultLogger 返回将失败事件写入 slog 的默认观察器；传入 logger 为 nil 时使用 slog.Default，并记录归一化后的公开错误字段与请求上下文。
func DefaultLogger(logger *slog.Logger) Observer {
	if logger == nil {
		logger = slog.Default()
	}

	return func(event Event) {
		public := normalizeHTTPError(event.Public)
		logger.Error(
			public.Message,
			"status", public.Status,
			"code", public.Code,
			"request_id", event.RequestID,
			"method", event.Method,
			"target", event.Target,
			"remote_addr", event.RemoteAddr,
			"error", event.Error,
		)
	}
}

// extractRequestContext 调用执行期配置中的请求上下文提取器；未配置时返回零值。
func (cfg executionConfig) extractRequestContext(r *http.Request) RequestContext {
	if cfg.extractor == nil {
		return RequestContext{}
	}
	return cfg.extractor(r)
}
