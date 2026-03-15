package reqx

import (
	"encoding/json"
	"errors"
	"io"
	"mime"
	"net/http"
	"reflect"
	"strings"
)

// DefaultJSONMaxBytes 是默认 JSON body 大小上限，当前为 1 MiB。
const DefaultJSONMaxBytes int64 = 1 << 20

// DecodeJSON 负责 JSON body 的通用边界检查。
// 它会处理 content-type、大小限制、unknown field、trailing data
// 以及常见 JSON 语法/类型错误，并统一返回 *Problem。
func DecodeJSON(w http.ResponseWriter, r *http.Request, target any) error {
	if w == nil {
		return errors.New("reqx: response writer must not be nil")
	}
	if r == nil {
		return errors.New("reqx: request must not be nil")
	}
	if target == nil {
		return errors.New("reqx: target must not be nil")
	}
	if err := validateDecodeTarget(target); err != nil {
		return err
	}

	if !hasJSONContentType(r.Header.Get("Content-Type")) {
		return UnsupportedMediaType()
	}

	r.Body = http.MaxBytesReader(w, r.Body, DefaultJSONMaxBytes)

	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(target); err != nil {
		return mapJSONError(err)
	}

	if err := decoder.Decode(&struct{}{}); err == nil {
		return BadRequest(TrailingData())
	} else if !errors.Is(err, io.EOF) {
		var maxBytesErr *http.MaxBytesError
		if errors.As(err, &maxBytesErr) {
			return PayloadTooLarge()
		}
		return BadRequest(TrailingData())
	}

	return nil
}

func validateDecodeTarget(target any) error {
	value := reflect.ValueOf(target)
	if !value.IsValid() {
		return errors.New("reqx: target must not be nil")
	}
	if value.Kind() != reflect.Pointer {
		return errors.New("reqx: decode target must be a non-nil pointer")
	}
	if value.IsNil() {
		return errors.New("reqx: decode target must be a non-nil pointer")
	}
	return nil
}

func hasJSONContentType(contentType string) bool {
	mediaType, _, err := mime.ParseMediaType(strings.TrimSpace(contentType))
	if err != nil {
		return false
	}
	return mediaType == "application/json" || strings.HasSuffix(mediaType, "+json")
}

// mapJSONError 把底层 JSON 解码错误压成稳定的请求错误细节。
func mapJSONError(err error) error {
	var maxBytesErr *http.MaxBytesError
	if errors.As(err, &maxBytesErr) {
		return PayloadTooLarge()
	}

	if errors.Is(err, io.EOF) {
		return BadRequest(Required(InBody, "body"))
	}

	var syntaxErr *json.SyntaxError
	if errors.As(err, &syntaxErr) {
		return BadRequest(MalformedJSON())
	}

	var typeErr *json.UnmarshalTypeError
	if errors.As(err, &typeErr) {
		field := strings.TrimSpace(typeErr.Field)
		if field == "" {
			field = "body"
		}
		return BadRequest(InvalidType(InBody, field))
	}

	const unknownFieldPrefix = "json: unknown field "
	msg := err.Error()
	if strings.HasPrefix(msg, unknownFieldPrefix) {
		field := strings.Trim(strings.TrimPrefix(msg, unknownFieldPrefix), "\"")
		if field == "" {
			field = "body"
		}
		return BadRequest(UnknownField(field))
	}

	return BadRequest(MalformedJSON())
}
