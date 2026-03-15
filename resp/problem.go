package resp

import (
	"github.com/kanata996/chix/reqx"
	"log/slog"
	"net/http"
)

// Problem 写请求错误响应。
// 它只接受 reqx.Problem；其他 error 会被视为边界故障并回退到 internal。
func Problem(w http.ResponseWriter, r *http.Request, err error) {
	problem, ok := reqx.AsProblem(err)
	if !ok {
		slog.ErrorContext(requestContext(r), "unexpected non-problem request error", "error", err)
		writeInternalEnvelope(w)
		return
	}

	code, ok := lookupRequestProblemCode(problem.StatusCode)
	if !ok {
		slog.ErrorContext(requestContext(r), "invalid request problem status",
			"status_code", problem.StatusCode,
			"error", err,
		)
		writeInternalEnvelope(w)
		return
	}

	writeProblemEnvelope(w, problem.StatusCode, code, problem.Error(), problem.Details)
}
