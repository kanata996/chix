package resp

import (
	"errors"
	"fmt"
	"net/http"
)

// JSON 按 Echo Response 的核心能力写出 JSON 响应。
// 当请求 URL 带有 ?pretty 时，会自动输出 pretty JSON。
func JSON(w http.ResponseWriter, r *http.Request, status int, data any) error {
	indent := ""
	if shouldPrettyJSON(r) {
		indent = defaultJSONIndent
	}
	return writeJSON(w, status, data, indent)
}

// JSONPretty 使用指定缩进写出 pretty JSON。
func JSONPretty(w http.ResponseWriter, _ *http.Request, status int, data any, indent string) error {
	return writeJSON(w, status, data, indent)
}

// JSONBlob 直接写出原始 JSON 字节。
// 调用方需要自行保证 body 是合法 JSON。
func JSONBlob(w http.ResponseWriter, _ *http.Request, status int, body []byte) error {
	return writeJSONBytes(w, status, body)
}

// OK 写出 200 JSON 成功响应。
func OK(w http.ResponseWriter, r *http.Request, data any) error {
	return writeSuccess(w, r, http.StatusOK, data)
}

// Created 写出 201 JSON 成功响应。
func Created(w http.ResponseWriter, r *http.Request, data any) error {
	return writeSuccess(w, r, http.StatusCreated, data)
}

// NoContent 写出 204 响应且不包含响应体。
func NoContent(w http.ResponseWriter, _ *http.Request) error {
	return writeStatus(w, http.StatusNoContent)
}

func writeJSON(w http.ResponseWriter, status int, data any, indent string) error {
	body, err := encodeJSON(data, indent)
	if err != nil {
		return err
	}
	return writeJSONBytes(w, status, body)
}

func writeSuccess(w http.ResponseWriter, r *http.Request, status int, data any) error {
	if err := validateSuccessBodyStatus(status); err != nil {
		return err
	}
	if w == nil {
		return errors.New("resp: response writer is nil")
	}

	indent := ""
	if shouldPrettyJSON(r) {
		indent = defaultJSONIndent
	}

	dataJSON, err := encodeJSON(data, indent)
	if err != nil {
		return err
	}
	if isJSONNullBytes(dataJSON) {
		return fmt.Errorf("resp: data must exist and must not encode to null")
	}

	return writeJSONBytes(w, status, dataJSON)
}

func shouldPrettyJSON(r *http.Request) bool {
	if r == nil || r.URL == nil {
		return false
	}
	_, pretty := r.URL.Query()["pretty"]
	return pretty
}

// validateSuccessBodyStatus 校验“带响应体”的成功状态码。
// 例如 204/304 虽然不是错误码，但语义上不允许带 body，因此这里直接拒绝。
func validateSuccessBodyStatus(status int) error {
	if err := validateSuccessStatus(status); err != nil {
		return err
	}
	if status < http.StatusOK {
		return fmt.Errorf("resp: success writers with a body cannot use informational status %d", status)
	}
	switch status {
	case http.StatusNoContent, http.StatusResetContent, http.StatusNotModified:
		return fmt.Errorf("resp: success writers with a body cannot use bodyless status %d", status)
	}
	return nil
}

// validateSuccessStatus 校验 success writer 可接受的状态码范围。
// 当前仅允许 1xx-3xx，避免把错误状态误传到成功响应分支。
func validateSuccessStatus(status int) error {
	if err := validateHTTPStatus(status); err != nil {
		return err
	}
	if status < 100 || status > 399 {
		return fmt.Errorf("resp: invalid success status %d", status)
	}
	return nil
}
