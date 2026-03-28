// 本文件职责：承载 runtime 的主执行路径，包括 Handle 挂载、输入绑定、业务调用和成功响应。
// 定位：作为 success / failure 单一路径的入口，只编排流程，不展开 failure 细节实现。
package runtime

import (
	"net/http"
	"reflect"
	"strings"

	"github.com/kanata996/chix/internal/schema"
)

// Handle 将 operation 与业务处理函数挂载为 http.Handler，并在挂载时完成输入 schema 解析与执行配置快照准备。
func Handle[I any, O any](rt *Runtime, op Operation[I, O], h Handler[I, O]) http.Handler {
	if rt == nil {
		panic("chix: runtime must not be nil")
	}
	if h == nil {
		panic("chix: handler must not be nil")
	}

	inputSchema, err := schema.Load(reflect.TypeOf((*I)(nil)).Elem())
	if err != nil {
		panic(err)
	}

	cfg := rt.executionConfig()
	op.ErrorMappers = append([]ErrorMapper(nil), op.ErrorMappers...)

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		execute(cfg, w, r, op, h, inputSchema)
	})
}

// execute 串联一次请求的完整成功路径：提取观测上下文、绑定输入、调用业务处理函数，并按约定写回成功响应；任一步失败都会转入统一 failure 写回。
func execute[I any, O any](
	cfg executionConfig,
	w http.ResponseWriter,
	r *http.Request,
	op Operation[I, O],
	h Handler[I, O],
	inputSchema *schema.Schema,
) {
	var (
		requestContext    RequestContext
		hasRequestContext bool
	)
	getRequestContext := func() RequestContext {
		if !hasRequestContext {
			requestContext = cfg.extractRequestContext(r)
			hasRequestContext = true
		}
		return requestContext
	}

	var input I
	if err := bindInputWithSchema(r, &input, inputSchema); err != nil {
		cfg.writeFailure(w, getRequestContext(), err, op.ErrorMappers)
		return
	}

	output, err := h(r.Context(), &input)
	if err != nil {
		cfg.writeFailure(w, getRequestContext(), err, op.ErrorMappers)
		return
	}

	status := resolveSuccessStatus(op.Method, r.Method, op.SuccessStatus, cfg.successStatus)
	if status == http.StatusNoContent {
		w.WriteHeader(status)
		return
	}

	payload, err := marshalSuccessEnvelope(output)
	if err != nil {
		cfg.writeFailure(w, getRequestContext(), err, op.ErrorMappers)
		return
	}

	if err := writeJSONResponse(w, status, payload); err != nil {
		cfg.observeInternalFailure(getRequestContext(), err)
	}
}

// resolveSuccessStatus 按 operation 显式配置、runtime 继承配置和请求方法推导成功响应状态码；未显式指定时，POST 默认返回 201，其余方法返回 200。
func resolveSuccessStatus(opMethod string, requestMethod string, explicit int, inherited int) int {
	if explicit > 0 {
		return explicit
	}
	if inherited > 0 {
		return inherited
	}

	method := strings.ToUpper(opMethod)
	if method == "" {
		method = strings.ToUpper(requestMethod)
	}
	if method == http.MethodPost {
		return http.StatusCreated
	}
	return http.StatusOK
}
