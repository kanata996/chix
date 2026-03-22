package core

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
)

type errorEnvelope struct {
	Error errorBody `json:"error"`
}

type errorBody struct {
	Code    string `json:"code"`
	Message string `json:"message"`
	Details []any  `json:"details"`
}

var FallbackErrorJSON = []byte(`{"error":{"code":"internal_error","message":"internal server error","details":[]}}`)

type ErrorPayload struct {
	Status  int
	Code    string
	Message string
	Details []any
}

func WriteSuccess(w http.ResponseWriter, status int, data any, meta any, includeMeta bool) error {
	if err := ValidateSuccessBodyStatus(status); err != nil {
		return err
	}

	dataJSON, err := json.Marshal(data)
	if err != nil {
		return err
	}
	if isJSONNullBytes(dataJSON) {
		return fmt.Errorf("chix: data must exist and must not encode to null")
	}

	var metaJSON []byte

	if includeMeta {
		metaJSON, err = json.Marshal(meta)
		if err != nil {
			return err
		}
		if !isJSONNullBytes(metaJSON) {
			if !isJSONObjectBytes(metaJSON) {
				return fmt.Errorf("chix: meta must encode as a JSON object")
			}
		} else {
			metaJSON = nil
		}
	}

	body := buildSuccessBody(dataJSON, metaJSON)
	return WriteJSONBytes(w, status, body)
}

func WriteEmpty(w http.ResponseWriter, status int) error {
	if err := ValidateSuccessStatus(status); err != nil {
		return err
	}

	w.WriteHeader(status)
	return nil
}

func WriteError(w http.ResponseWriter, payload ErrorPayload) {
	body, err := json.Marshal(errorEnvelope{
		Error: errorBody{
			Code:    payload.Code,
			Message: payload.Message,
			Details: normalizeDetails(payload.Details),
		},
	})
	if err != nil {
		if fallbackErr := WriteJSONBytes(w, http.StatusInternalServerError, FallbackErrorJSON); fallbackErr != nil {
			return
		}
		return
	}

	if writeErr := WriteJSONBytes(w, payload.Status, body); writeErr != nil {
		return
	}
}

func normalizeDetails(details []any) []any {
	if len(details) == 0 {
		return []any{}
	}
	return details
}

func ValidateSuccessBodyStatus(status int) error {
	if err := ValidateSuccessStatus(status); err != nil {
		return err
	}
	if status < http.StatusOK {
		return fmt.Errorf("chix: success writers with a body cannot use informational status %d", status)
	}
	switch status {
	case http.StatusNoContent, http.StatusResetContent, http.StatusNotModified:
		return fmt.Errorf("chix: success writers with a body cannot use bodyless status %d", status)
	}
	return nil
}

func ValidateSuccessStatus(status int) error {
	if status >= 400 {
		return fmt.Errorf("chix: success writers cannot use error status %d", status)
	}
	if status < 100 {
		return fmt.Errorf("chix: invalid HTTP status %d", status)
	}
	return nil
}

func buildSuccessBody(dataJSON []byte, metaJSON []byte) []byte {
	body := make([]byte, 0, len(dataJSON)+len(metaJSON)+24)
	body = append(body, `{"data":`...)
	body = append(body, dataJSON...)
	if len(metaJSON) > 0 {
		body = append(body, `,"meta":`...)
		body = append(body, metaJSON...)
	}
	body = append(body, '}')
	return body
}

func WriteJSONBytes(w http.ResponseWriter, status int, body []byte) error {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_, err := w.Write(body)
	return err
}

func IsJSONNullValue(v any) bool {
	body, err := json.Marshal(v)
	if err != nil {
		return false
	}
	return isJSONNullBytes(body)
}

func IsJSONObjectLike(v any) bool {
	body, err := json.Marshal(v)
	if err != nil {
		return false
	}
	if isJSONNullBytes(body) {
		return true
	}
	return isJSONObjectBytes(body)
}

func isJSONNullBytes(body []byte) bool {
	return bytes.Equal(bytes.TrimSpace(body), []byte("null"))
}

func isJSONObjectBytes(body []byte) bool {
	trimmed := bytes.TrimSpace(body)
	if len(trimmed) == 0 {
		return false
	}
	if trimmed[0] != '{' || trimmed[len(trimmed)-1] != '}' {
		return false
	}
	return true
}
