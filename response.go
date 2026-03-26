package chix

import (
	"encoding"
	"encoding/json"
	"fmt"
	"mime"
	"net/http"
	"reflect"
	"strings"
)

const (
	defaultJSONResponseContentType    = "application/json; charset=utf-8"
	defaultProblemResponseContentType = "application/problem+json; charset=utf-8"
)

func writeJSON(w http.ResponseWriter, status int, value any) error {
	return writeJSONWithContentType(w, status, "", value)
}

func writeJSONWithContentType(w http.ResponseWriter, status int, contentType string, value any) error {
	body, err := json.Marshal(value)
	if err != nil {
		return err
	}
	if strings.TrimSpace(contentType) == "" {
		contentType = defaultJSONResponseContentType
	}
	return writeBytes(w, status, contentType, body)
}

func writeProblem(w http.ResponseWriter, status int, contentType string, value any) error {
	if !isJSONContentType(contentType) {
		contentType = ""
	}
	if strings.TrimSpace(contentType) == "" {
		contentType = defaultProblemResponseContentType
	}
	return writeJSONWithContentType(w, status, contentType, value)
}

func writeResponse(w http.ResponseWriter, status int, contentType string, value any) error {
	contentType = strings.TrimSpace(contentType)
	switch {
	case contentType == "" || isJSONContentType(contentType):
		return writeJSONWithContentType(w, status, contentType, value)
	case isTextContentType(contentType):
		body, err := textResponseBody(value)
		if err != nil {
			return err
		}
		return writeBytes(w, status, contentType, body)
	default:
		body, err := binaryResponseBody(value)
		if err != nil {
			return err
		}
		return writeBytes(w, status, contentType, body)
	}
}

func writeBytes(w http.ResponseWriter, status int, contentType string, body []byte) error {
	if strings.TrimSpace(contentType) != "" {
		w.Header().Set("Content-Type", contentType)
	}
	w.WriteHeader(status)
	_, err := w.Write(body)
	return err
}

func isJSONContentType(contentType string) bool {
	base := mediaTypeBase(contentType)
	return base == "application/json" || strings.HasSuffix(base, "+json")
}

func isTextContentType(contentType string) bool {
	return strings.HasPrefix(mediaTypeBase(contentType), "text/")
}

func mediaTypeBase(contentType string) string {
	contentType = strings.TrimSpace(contentType)
	if contentType == "" {
		return ""
	}
	base, _, err := mime.ParseMediaType(contentType)
	if err == nil {
		return strings.ToLower(base)
	}
	if index := strings.Index(contentType, ";"); index >= 0 {
		contentType = contentType[:index]
	}
	return strings.ToLower(strings.TrimSpace(contentType))
}

func textResponseBody(value any) ([]byte, error) {
	value = indirectResponseValue(value)
	switch v := value.(type) {
	case string:
		return []byte(v), nil
	case []byte:
		return v, nil
	case encoding.TextMarshaler:
		return v.MarshalText()
	case fmt.Stringer:
		return []byte(v.String()), nil
	default:
		if body, ok := primitiveTextResponseBody(value); ok {
			return body, nil
		}
		return nil, fmt.Errorf("chix: unsupported response type %T for text content", value)
	}
}

func binaryResponseBody(value any) ([]byte, error) {
	value = indirectResponseValue(value)
	switch v := value.(type) {
	case []byte:
		return v, nil
	case string:
		return []byte(v), nil
	default:
		if body, ok := byteSliceResponseBody(value); ok {
			return body, nil
		}
		return nil, fmt.Errorf("chix: unsupported response type %T for content type", value)
	}
}

func indirectResponseValue(value any) any {
	if value == nil {
		return nil
	}
	current := reflect.ValueOf(value)
	for current.Kind() == reflect.Pointer {
		if current.IsNil() {
			return nil
		}
		current = current.Elem()
	}
	return current.Interface()
}

func primitiveTextResponseBody(value any) ([]byte, bool) {
	current := reflect.ValueOf(value)
	if !current.IsValid() {
		return nil, false
	}
	switch current.Kind() {
	case reflect.String:
		return []byte(current.String()), true
	default:
		return nil, false
	}
}

func byteSliceResponseBody(value any) ([]byte, bool) {
	current := reflect.ValueOf(value)
	if !current.IsValid() {
		return nil, false
	}
	if current.Kind() != reflect.Slice || current.Type().Elem().Kind() != reflect.Uint8 {
		return nil, false
	}

	if current.Type() == reflect.TypeOf([]byte(nil)) {
		return current.Bytes(), true
	}

	converted := current.Convert(reflect.TypeOf([]byte(nil)))
	return converted.Bytes(), true
}
