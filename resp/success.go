package resp

import (
	"bytes"
	"encoding/json"
	"log/slog"
	"net/http"
)

// Success 写 200 统一成功包络。
// 调用方必须保证 data 语义完整且顶层编码后非 null。
func Success(w http.ResponseWriter, data any) {
	writeSuccess(w, http.StatusOK, data)
}

// Created 写 201 统一成功包络。
// 调用方必须保证 data 语义完整且顶层编码后非 null。
func Created(w http.ResponseWriter, data any) {
	writeSuccess(w, http.StatusCreated, data)
}

// writeSuccess 负责 success 包络的统一写回。
// 若 payload 缺失或边界编码失败，则 fail-closed 为裸 500。
func writeSuccess(w http.ResponseWriter, statusCode int, data any) {
	dataBody, ok := encodeSuccessData(w, statusCode, data)
	if !ok {
		return
	}

	body, err := encodeJSON(envelope{Code: 0, Data: json.RawMessage(dataBody)})
	if err != nil {
		failClosedSuccess(w, statusCode, "could not encode success response", err)
		return
	}

	writeJSONBody(w, statusCode, body)
}

// encodeSuccessData 预先验证 success payload 是否存在且不会编码成顶层 null。
func encodeSuccessData(w http.ResponseWriter, statusCode int, data any) ([]byte, bool) {
	if data == nil {
		failClosedSuccess(w, statusCode, "missing success response payload", nil)
		return nil, false
	}

	dataBody, err := encodeJSON(data)
	if err != nil {
		failClosedSuccess(w, statusCode, "could not encode success response", err)
		return nil, false
	}

	dataBody = bytes.TrimSpace(dataBody)
	if len(dataBody) == 0 || bytes.Equal(dataBody, []byte("null")) {
		failClosedSuccess(w, statusCode, "missing success response payload", nil)
		return nil, false
	}

	return dataBody, true
}

// failClosedSuccess 统一处理 success 边界自故障。
func failClosedSuccess(w http.ResponseWriter, statusCode int, message string, err error) {
	attrs := []any{"status_code", statusCode}
	if err != nil {
		attrs = append(attrs, "error", err)
	}
	slog.Error(message, attrs...)
	if w != nil {
		writeStatusOnly(w, http.StatusInternalServerError)
	}
}

// NoContent 写 204 无 body 成功响应。
func NoContent(w http.ResponseWriter) {
	writeStatusOnly(w, http.StatusNoContent)
}
