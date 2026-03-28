// 本文件职责：放置请求上下文提取、默认日志观测器以及作用域配置解析逻辑。
// 定位：服务 failure / observation 边界，但与主执行流程和错误映射实现分开维护。
package runtime

import (
	"log/slog"
	"net/http"

	"github.com/go-chi/chi/v5/middleware"
)

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

func (cfg executionConfig) extractRequestContext(r *http.Request) RequestContext {
	if cfg.extractor == nil {
		return RequestContext{}
	}
	return cfg.extractor(r)
}

func (rt *Runtime) executionConfig() executionConfig {
	return executionConfig{
		errorMappers:  append([]ErrorMapper(nil), rt.errorMappers()...),
		observer:      rt.observer(),
		extractor:     rt.extractor(),
		successStatus: rt.successStatus(),
	}
}

func (rt *Runtime) extractor() Extractor {
	for current := rt; current != nil; current = current.parent {
		if current.local.hasExtractor {
			return current.local.extractor
		}
	}
	return DefaultExtractor
}

func (rt *Runtime) observer() Observer {
	for current := rt; current != nil; current = current.parent {
		if current.local.hasObserver {
			return current.local.observer
		}
	}
	return nil
}

func (rt *Runtime) successStatus() int {
	for current := rt; current != nil; current = current.parent {
		if current.local.hasSuccessStatus {
			return current.local.successStatus
		}
	}
	return 0
}

func (rt *Runtime) errorMappers() []ErrorMapper {
	var chain []ErrorMapper
	for current := rt; current != nil; current = current.parent {
		chain = append(chain, current.local.errorMappers...)
	}
	return chain
}
