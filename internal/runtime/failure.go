// 本文件职责：集中处理公开错误解析、mapper 组合、observer 触发和错误响应写回。
// 定位：作为 v1 runtime failure / observation 语义的收口点，确保行为单一且可验证。
package runtime

import "net/http"

// writeFailure 解析原始错误、触发失败观测，并将公开错误写回响应。
func (cfg executionConfig) writeFailure(w http.ResponseWriter, requestContext RequestContext, raw error, opMappers []ErrorMapper) {
	failure := cfg.resolveFailure(raw, opMappers)
	cfg.observeFailure(requestContext, failure)
	cfg.writePublicError(w, requestContext, failure.public)
}

// writePublicError 将公开错误编码并写回；编码失败时记录内部失败并退回内部错误响应。
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

// writeInternalError 写回固定的内部错误响应；若编码或写回失败，仅记录内部失败。
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

// resolveFailure 封装原始错误及其解析后的公开错误表示。
func (cfg executionConfig) resolveFailure(raw error, opMappers []ErrorMapper) resolvedFailure {
	return resolvedFailure{
		raw:    raw,
		public: cfg.resolvePublicError(raw, opMappers),
	}
}

// resolvePublicError 按 runtime 内部错误、错误值、operation mapper、runtime mapper 的顺序解析公开错误；已公开化的错误不会再进入 mapper 链。
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

// observeFailure 使用已解析的公开错误发出失败观测事件。
func (cfg executionConfig) observeFailure(requestContext RequestContext, failure resolvedFailure) {
	cfg.observe(requestContext, failure.raw, failure.public)
}

// observeInternalFailure 将内部错误按固定的 500 公开错误语义发出观测事件。
func (cfg executionConfig) observeInternalFailure(requestContext RequestContext, raw error) {
	cfg.observe(requestContext, raw, internalHTTPError())
}

// observe 在配置了 observer 时发出失败观测事件，并补齐公开错误的默认字段。
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
