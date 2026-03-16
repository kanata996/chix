package reqx

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"mime"
	"net/http"
	"reflect"
	"strings"
)

// DefaultJSONMaxBytes 是默认 JSON body 大小上限，当前为 1 MiB。
const DefaultJSONMaxBytes int64 = 1 << 20

// DecodeOptions 用于覆盖默认的严格 JSON 解码策略。
// 零值会沿用 DecodeJSON 的默认行为。
type DecodeOptions struct {
	MaxBytes             int64
	AllowUnknownFields   bool
	SkipContentTypeCheck bool
}

// DecodeJSON 负责 JSON body 的通用边界检查。
// 它会处理 content-type、大小限制、unknown field、trailing data
// 以及常见 JSON 语法/类型错误，并统一返回 *Problem。
func DecodeJSON(w http.ResponseWriter, r *http.Request, target any) error {
	return DecodeJSONWith(w, r, target, DecodeOptions{})
}

// DecodeJSONWith 与 DecodeJSON 行为一致，但允许按 endpoint 覆盖默认策略。
func DecodeJSONWith(w http.ResponseWriter, r *http.Request, target any, options DecodeOptions) error {
	if w == nil {
		return ErrNilResponseWriter
	}
	if r == nil {
		return ErrNilRequest
	}
	if target == nil {
		return ErrNilTarget
	}
	if err := validateDecodeTarget(target); err != nil {
		return err
	}

	options = normalizeDecodeOptions(options)

	if !options.SkipContentTypeCheck && !hasJSONContentType(r.Header.Get("Content-Type")) {
		return UnsupportedMediaType()
	}

	r.Body = http.MaxBytesReader(w, r.Body, options.MaxBytes)
	body, err := io.ReadAll(r.Body)
	if err != nil {
		return mapJSONError(err)
	}

	trimmedBody := bytes.TrimSpace(body)
	if len(trimmedBody) == 0 || bytes.Equal(trimmedBody, []byte("null")) {
		return BadRequest(Required(InBody, "body"))
	}

	decoder := json.NewDecoder(bytes.NewReader(body))
	if !options.AllowUnknownFields {
		decoder.DisallowUnknownFields()
	}
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

func normalizeDecodeOptions(options DecodeOptions) DecodeOptions {
	if options.MaxBytes <= 0 {
		options.MaxBytes = DefaultJSONMaxBytes
	}
	return options
}

func validateDecodeTarget(target any) error {
	value := reflect.ValueOf(target)
	if !value.IsValid() {
		return ErrNilTarget
	}
	if value.Kind() != reflect.Pointer {
		return fmt.Errorf("%w: target must be a non-nil pointer", ErrInvalidDecodeTarget)
	}
	if value.IsNil() {
		return fmt.Errorf("%w: target must be a non-nil pointer", ErrInvalidDecodeTarget)
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
