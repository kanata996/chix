package resp

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
)

const (
	jsonContentType        = "application/json"
	problemJSONContentType = "application/problem+json"
	defaultJSONIndent      = "  "
)

type responseWriteError struct {
	cause           error
	responseStarted bool
}

func (e *responseWriteError) Error() string {
	if e == nil || e.cause == nil {
		return "resp: write response failed"
	}
	return "resp: write response failed: " + e.cause.Error()
}

func (e *responseWriteError) Unwrap() error {
	if e == nil {
		return nil
	}
	return e.cause
}

// writeJSONBytes 以 application/json 写出原始 JSON 字节切片。
// 调用方需要自行保证 body 已经是合法 JSON。
func writeJSONBytes(w http.ResponseWriter, status int, body []byte) error {
	return writeJSONBytesWithContentType(w, status, jsonContentType, body)
}

// writeJSONBytesWithContentType 以指定 JSON 媒体类型写出原始 JSON 字节切片。
// 调用方需要自行保证 body 已经是合法 JSON。
func writeJSONBytesWithContentType(w http.ResponseWriter, status int, contentType string, body []byte) error {
	if w == nil {
		return errors.New("resp: response writer is nil")
	}
	if err := validateHTTPStatus(status); err != nil {
		return err
	}
	w.Header().Set("Content-Type", contentType)
	w.WriteHeader(status)
	if _, err := w.Write(body); err != nil {
		return &responseWriteError{
			cause:           err,
			responseStarted: true,
		}
	}
	return nil
}

// writeStatus 仅写出状态码，不包含响应体。
func writeStatus(w http.ResponseWriter, status int) error {
	if w == nil {
		return errors.New("resp: response writer is nil")
	}
	if err := validateHTTPStatus(status); err != nil {
		return err
	}
	w.WriteHeader(status)
	return nil
}

// encodeJSON 使用标准库编码 JSON。
// 当 indent 非空时，会输出 pretty JSON；两种模式都会保留标准库尾部换行。
func encodeJSON(data any, indent string) ([]byte, error) {
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	if indent != "" {
		enc.SetIndent("", indent)
	}
	if err := enc.Encode(data); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

func validateHTTPStatus(status int) error {
	if status < 100 || status > 999 {
		return fmt.Errorf("resp: invalid HTTP status %d", status)
	}
	return nil
}

// isJSONNullBytes 判断一段 JSON 字节在去掉首尾空白后是否等于 null。
func isJSONNullBytes(body []byte) bool {
	return bytes.Equal(bytes.TrimSpace(body), []byte("null"))
}
