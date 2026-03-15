package resp

import (
	"bytes"
	"encoding/json"
	"fmt"
	"github.com/kanata996/chix/errx"
	"github.com/kanata996/chix/reqx"
	"log/slog"
	"net/http"
)

type envelope struct {
	Code    int64         `json:"code"`
	Message string        `json:"message,omitempty"`
	Data    any           `json:"data,omitempty"`
	Details []reqx.Detail `json:"details,omitempty"`
}

var (
	internalMapping = mustMapping(errx.Internal(errx.CodeInternal))

	requestProblemCodes = map[int]int64{
		http.StatusBadRequest:            errx.CodeInvalidRequest,
		http.StatusRequestEntityTooLarge: errx.CodePayloadTooLarge,
		http.StatusUnsupportedMediaType:  errx.CodeUnsupportedMediaType,
		http.StatusUnprocessableEntity:   errx.CodeUnprocessableEntity,
	}
)

// encodeJSON 只负责把 payload 编成一段完整 JSON，不直接写回响应。
func encodeJSON(payload any) ([]byte, error) {
	var buf bytes.Buffer
	if err := json.NewEncoder(&buf).Encode(payload); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

// writeJSON 是 resp 最底层的 JSON 写回 helper。
// 若边界自身编码失败，则 fail-closed 为裸 500。
func writeJSON(w http.ResponseWriter, statusCode int, payload any) {
	body, err := encodeJSON(payload)
	if err != nil {
		slog.Error("could not encode response",
			"status_code", statusCode,
			"error", err,
		)
		writeStatusOnly(w, http.StatusInternalServerError)
		return
	}

	writeJSONBody(w, statusCode, body)
}

// writeErrorEnvelope 统一写业务/系统错误包络。
func writeErrorEnvelope(w http.ResponseWriter, mapping errx.Mapping, details []reqx.Detail) {
	payload := envelope{
		Code:    mapping.Code,
		Message: mapping.Message,
	}
	if len(details) != 0 {
		payload.Details = append([]reqx.Detail(nil), details...)
	}
	writeJSON(w, mapping.StatusCode, payload)
}

// writeInternalEnvelope 用于边界自故障时回退到标准 internal 包络。
func writeInternalEnvelope(w http.ResponseWriter) {
	writeErrorEnvelope(w, internalMapping, nil)
}

// writeProblemEnvelope 统一写请求错误包络。
// 请求错误 message 跟随 reqx.Problem 的 HTTP 语义，code 由 resp 侧稳定映射。
func writeProblemEnvelope(w http.ResponseWriter, statusCode int, code int64, message string, details []reqx.Detail) {
	payload := envelope{
		Code:    code,
		Message: message,
	}
	if len(details) != 0 {
		payload.Details = append([]reqx.Detail(nil), details...)
	}
	writeJSON(w, statusCode, payload)
}

// writeStatusOnly 用于 fail-closed 场景或无 body 场景。
func writeStatusOnly(w http.ResponseWriter, statusCode int) {
	w.Header().Del("Content-Type")
	w.WriteHeader(statusCode)
}

// writeJSONBody 假定 body 已经编码完成，只负责最终写回。
// 若写回阶段发生 transport 错误，只做 best-effort 观测，不再改写响应。
func writeJSONBody(w http.ResponseWriter, statusCode int, body []byte) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	if _, err := w.Write(body); err != nil {
		slog.Warn("response write failed",
			"status_code", statusCode,
			"error", err,
		)
	}
}

// lookupRequestProblemCode 把请求类 HTTP 状态码映射到稳定错误码。
func lookupRequestProblemCode(statusCode int) (int64, bool) {
	code, ok := requestProblemCodes[statusCode]
	return code, ok
}

// mustMapping 用于声明期校验 resp 内建 mapping，避免把契约问题拖到运行时。
func mustMapping(mapping errx.Mapping) errx.Mapping {
	if err := mapping.Validate(); err != nil {
		panic(fmt.Sprintf("resp: invalid mapping: %v", err))
	}
	return mapping
}
