// 本文件职责：定义 runtime 的 success / error JSON envelope 以及统一写回逻辑。
// 定位：承载响应 wire contract，避免把 success path 输出细节隐藏在 error model 文件中。
package runtime

import (
	"encoding/json"
	"net/http"
)

func marshalSuccessEnvelope(value any) ([]byte, error) {
	return json.Marshal(struct {
		Data any `json:"data"`
	}{
		Data: value,
	})
}

func marshalErrorEnvelope(public *HTTPError) ([]byte, error) {
	return json.Marshal(struct {
		Error *HTTPError `json:"error"`
	}{
		Error: normalizeHTTPError(public),
	})
}

func writeJSONResponse(w http.ResponseWriter, status int, payload []byte) error {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_, err := w.Write(payload)
	return err
}
