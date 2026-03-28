// 本文件职责：集中处理公开错误解析、mapper 组合、observer 触发和错误响应写回。
// 定位：作为 v1 runtime failure / observation 语义的收口点，确保行为单一且可验证。
package runtime

import "net/http"

func (cfg executionConfig) writeFailure(w http.ResponseWriter, requestContext RequestContext, raw error, opMappers []ErrorMapper) {
	failure := cfg.resolveFailure(raw, opMappers)
	cfg.observeFailure(requestContext, failure)
	cfg.writePublicError(w, requestContext, failure.public)
}

func (cfg executionConfig) writePublicError(w http.ResponseWriter, requestContext RequestContext, public *HTTPError) {
	payload, err := marshalErrorEnvelope(public)
	if err != nil {
		cfg.observeInternalFailure(requestContext, err)
		cfg.writeInternalError(w, requestContext)
		return
	}

	if err := writeJSONResponse(w, public.Status, payload); err != nil {
		cfg.observeInternalFailure(requestContext, err)
	}
}

func (cfg executionConfig) writeInternalError(w http.ResponseWriter, requestContext RequestContext) {
	internal := internalHTTPError()
	payload, err := marshalErrorEnvelope(internal)
	if err != nil {
		cfg.observeInternalFailure(requestContext, err)
		return
	}

	if err := writeJSONResponse(w, internal.Status, payload); err != nil {
		cfg.observeInternalFailure(requestContext, err)
	}
}

func (cfg executionConfig) resolveFailure(raw error, opMappers []ErrorMapper) resolvedFailure {
	return resolvedFailure{
		raw:    raw,
		public: cfg.resolvePublicError(raw, opMappers),
	}
}

func (cfg executionConfig) resolvePublicError(raw error, opMappers []ErrorMapper) *HTTPError {
	public := publicErrorFromRuntime(raw)
	if public != nil {
		return public
	}

	public = publicErrorFromValue(raw)
	if public != nil {
		return public
	}

	// 公开边界错误必须直接旁路 mapper；只有未公开化的原始错误才进入 mapper 链。
	// 优先级顺序与技术手册保持一致：operation > 内层 scope > 外层 scope/runtime。
	for _, mapper := range opMappers {
		if mapper == nil {
			continue
		}
		if mapped := mapper(raw); mapped != nil {
			return normalizeHTTPError(mapped)
		}
	}

	for _, mapper := range cfg.errorMappers {
		if mapper == nil {
			continue
		}
		if mapped := mapper(raw); mapped != nil {
			return normalizeHTTPError(mapped)
		}
	}

	return internalHTTPError()
}

func (cfg executionConfig) observeFailure(requestContext RequestContext, failure resolvedFailure) {
	cfg.observe(requestContext, failure.raw, failure.public)
}

func (cfg executionConfig) observeInternalFailure(requestContext RequestContext, raw error) {
	cfg.observe(requestContext, raw, internalHTTPError())
}

func (cfg executionConfig) observe(requestContext RequestContext, raw error, public *HTTPError) {
	if cfg.observer == nil {
		return
	}

	cfg.observer(Event{
		Error:      raw,
		Public:     normalizeHTTPError(public),
		RequestID:  requestContext.RequestID,
		Method:     requestContext.Method,
		Target:     requestContext.Target,
		RemoteAddr: requestContext.RemoteAddr,
	})
}
