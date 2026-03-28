// 本文件职责：承载 runtime 的主执行路径，包括 Handle 挂载、输入绑定、业务调用和成功响应。
// 定位：作为 success / failure 单一路径的入口，只编排流程，不展开 failure 细节实现。
package runtime

import (
	"net/http"
	"reflect"
	"strings"

	"github.com/kanata996/chix/internal/schema"
)

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

func execute[I any, O any](
	cfg executionConfig,
	w http.ResponseWriter,
	r *http.Request,
	op Operation[I, O],
	h Handler[I, O],
	inputSchema *schema.Schema,
) {
	requestContext := cfg.extractRequestContext(r)

	var input I
	if err := bindInputWithSchema(r, &input, inputSchema); err != nil {
		cfg.writeFailure(w, requestContext, err, op.ErrorMappers)
		return
	}

	output, err := h(r.Context(), &input)
	if err != nil {
		cfg.writeFailure(w, requestContext, err, op.ErrorMappers)
		return
	}

	status := resolveSuccessStatus(op.Method, r.Method, op.SuccessStatus, cfg.successStatus)
	if status == http.StatusNoContent {
		w.WriteHeader(status)
		return
	}

	payload, err := marshalSuccessEnvelope(output)
	if err != nil {
		cfg.writeFailure(w, requestContext, err, op.ErrorMappers)
		return
	}

	if err := writeJSONResponse(w, status, payload); err != nil {
		cfg.observeInternalFailure(requestContext, err)
	}
}

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
