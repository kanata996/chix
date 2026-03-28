// 本文件职责：定义 runtime 的成功/失败 JSON 编码以及统一写回逻辑。
// 定位：承载响应 wire contract，避免把 success / failure path 输出细节隐藏在执行流中。
package runtime

import (
	"encoding/json"
	"net/http"
)

// marshalSuccessBody 直接将业务返回值序列化为成功响应体。
func marshalSuccessBody(value any) ([]byte, error) {
	return json.Marshal(value)
}

// marshalErrorEnvelope 将公开错误归一化后包装为 error envelope 并序列化为 JSON。
func marshalErrorEnvelope(public *HTTPError) ([]byte, error) {
	return json.Marshal(struct {
		Error *HTTPError `json:"error"`
	}{
		Error: normalizeHTTPError(public),
	})
}

// writeJSONResponse 按给定状态码写回 JSON 负载，并固定设置 UTF-8 的 JSON Content-Type。
func writeJSONResponse(w http.ResponseWriter, status int, payload []byte) error {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_, err := w.Write(payload)
	return err
}
