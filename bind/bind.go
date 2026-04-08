package bind

import (
	"bytes"
	"encoding"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/textproto"
	"reflect"
	"strconv"
	"strings"
	"time"

	"github.com/kanata996/chix/errx"
)

const defaultMaxBodyBytes int64 = 1 << 20

const (
	mimeApplicationJSON = "application/json"
)

const (
	CodeInvalidJSON          = "invalid_json"
	CodeUnsupportedMediaType = "unsupported_media_type"
	CodeRequestTooLarge      = "request_too_large"
)

// Binder 定义默认请求绑定器接口。
type Binder interface {
	Bind(r *http.Request, target any) error
}

// DefaultBinder 是面向 JSON API 的默认绑定器。
type DefaultBinder struct{}

// BindUnmarshaler 允许字段从单个字符串输入值自定义解码。
type BindUnmarshaler interface {
	UnmarshalParam(param string) error
}

type bindMultipleUnmarshaler interface {
	UnmarshalParams(params []string) error
}

type bindBodyConfig struct {
	maxBodyBytes       int64
	allowUnknownFields bool
}

type bindConfig struct {
	body bindBodyConfig
}

func defaultBindConfig() bindConfig {
	return bindConfig{
		body: bindBodyConfig{
			maxBodyBytes:       defaultMaxBodyBytes,
			allowUnknownFields: true,
		},
	}
}

// Bind 按默认顺序绑定请求数据：path -> query(GET/DELETE/HEAD) -> body。
func Bind(r *http.Request, target any) error {
	if r == nil {
		return errorsf("request must not be nil")
	}
	if target == nil {
		return errorsf("destination must not be nil")
	}

	return bindWithConfig(r, target, defaultBindConfig())
}

// BindBody 只从请求 body 绑定数据。
func BindBody(r *http.Request, target any) error {
	if r == nil {
		return errorsf("request must not be nil")
	}
	if target == nil {
		return errorsf("destination must not be nil")
	}

	return bindBodyDefault(r, target, defaultBindConfig().body)
}

// BindQueryParams 只从 query 参数绑定数据。
func BindQueryParams(r *http.Request, target any) error {
	if r == nil {
		return errorsf("request must not be nil")
	}
	if target == nil {
		return errorsf("destination must not be nil")
	}

	return bindQueryParamsDefault(r, target)
}

// BindPathValues 只从 path 参数绑定数据。
func BindPathValues(r *http.Request, target any) error {
	if r == nil {
		return errorsf("request must not be nil")
	}
	if target == nil {
		return errorsf("destination must not be nil")
	}

	return bindPathValuesDefault(r, target)
}

// BindHeaders 只从 header 绑定数据。
func BindHeaders(r *http.Request, target any) error {
	if r == nil {
		return errorsf("request must not be nil")
	}
	if target == nil {
		return errorsf("destination must not be nil")
	}

	return bindHeadersDefault(r, target)
}

func (b *DefaultBinder) Bind(r *http.Request, target any) error {
	return bindWithConfig(r, target, defaultBindConfig())
}

func bindWithConfig(r *http.Request, target any, cfg bindConfig) error {
	if r == nil {
		return errorsf("request must not be nil")
	}
	if err := validateBindingDestination(target); err != nil {
		return err
	}

	if err := bindPathValuesDefault(r, target); err != nil {
		return err
	}

	method := strings.ToUpper(strings.TrimSpace(r.Method))
	if method == http.MethodGet || method == http.MethodDelete || method == http.MethodHead {
		if err := bindQueryParamsDefault(r, target); err != nil {
			return err
		}
	}

	return bindBodyDefault(r, target, cfg.body)
}

func validateBindingDestination(target any) error {
	if target == nil {
		return errorsf("destination must not be nil")
	}
	value := reflect.ValueOf(target)
	if value.Kind() != reflect.Pointer || value.IsNil() {
		return errorsf("destination must not be nil")
	}
	return nil
}

func bindPathValuesDefault(r *http.Request, target any) error {
	params := map[string][]string{}
	if r != nil {
		for _, name := range pathWildcardNames(r.Pattern) {
			params[name] = []string{r.PathValue(name)}
		}
	}
	if err := bindDataDefault(target, params, "param", nil); err != nil {
		return badRequestWrap(err)
	}
	return nil
}

func bindQueryParamsDefault(r *http.Request, target any) error {
	params := map[string][]string{}
	if r != nil && r.URL != nil {
		params = r.URL.Query()
	}
	if err := bindDataDefault(target, params, "query", nil); err != nil {
		return badRequestWrap(err)
	}
	return nil
}

func bindHeadersDefault(r *http.Request, target any) error {
	params := map[string][]string{}
	if r != nil {
		for key, values := range r.Header {
			trimmed := strings.TrimSpace(key)
			if trimmed == "" {
				continue
			}
			params[textproto.CanonicalMIMEHeaderKey(trimmed)] = values
		}
	}
	if err := bindDataDefault(target, params, "header", nil); err != nil {
		return badRequestWrap(err)
	}
	return nil
}

func bindBodyDefault(r *http.Request, target any, cfg bindBodyConfig) error {
	if r == nil {
		return errorsf("request must not be nil")
	}
	if err := validateBindingDestination(target); err != nil {
		return err
	}

	if r.ContentLength == 0 {
		return nil
	}

	body, err := readBody(r.Body, cfg.maxBodyBytes)
	if err != nil {
		if errors.Is(err, errRequestTooLarge) {
			return requestTooLargeError()
		}
		return err
	}

	mediaType := strings.TrimSpace(bodyMediaType(r))
	switch mediaType {
	case mimeApplicationJSON:
		return decodeJSONBody(body, target, cfg.allowUnknownFields)
	default:
		return unsupportedMediaTypeError()
	}
}

func bodyMediaType(r *http.Request) string {
	if r == nil {
		return ""
	}
	base, _, _ := strings.Cut(r.Header.Get("Content-Type"), ";")
	return strings.TrimSpace(base)
}

func decodeJSONBody(body []byte, target any, allowUnknownFields bool) error {
	dec := json.NewDecoder(bytes.NewReader(body))
	if !allowUnknownFields {
		dec.DisallowUnknownFields()
	}
	if err := dec.Decode(target); err != nil {
		return mapJSONBodyDecodeError(err)
	}
	return nil
}

func mapJSONBodyDecodeError(err error) error {
	var invalidUnmarshalErr *json.InvalidUnmarshalError
	if errors.As(err, &invalidUnmarshalErr) {
		return err
	}

	return errx.NewHTTPErrorWithCause(
		http.StatusBadRequest,
		CodeInvalidJSON,
		"request body must be valid JSON",
		err,
	)
}

func badRequestWrap(err error) error {
	if err == nil {
		return nil
	}

	var httpErr *errx.HTTPError
	if errors.As(err, &httpErr) && httpErr != nil {
		return err
	}

	return errx.NewHTTPErrorWithCause(http.StatusBadRequest, "", "", err)
}

func bindDataDefault(destination any, data map[string][]string, tag string, dataFiles map[string][]*multipart.FileHeader) error {
	if destination == nil || (len(data) == 0 && len(dataFiles) == 0) {
		return nil
	}

	typ := reflect.TypeOf(destination)
	val := reflect.ValueOf(destination)
	if typ.Kind() != reflect.Pointer || val.IsNil() {
		return errors.New("binding element must be a pointer")
	}

	typ = typ.Elem()
	val = val.Elem()
	hasFiles := len(dataFiles) > 0

	if typ.Kind() == reflect.Map && typ.Key().Kind() == reflect.String {
		elemKind := typ.Elem().Kind()
		isElemInterface := elemKind == reflect.Interface
		isElemString := elemKind == reflect.String
		isElemSliceOfStrings := elemKind == reflect.Slice && typ.Elem().Elem().Kind() == reflect.String
		if !isElemSliceOfStrings && !isElemString && !isElemInterface {
			return nil
		}
		if val.IsNil() {
			val.Set(reflect.MakeMap(typ))
		}
		for key, values := range data {
			switch {
			case isElemString, isElemInterface:
				if len(values) == 0 {
					continue
				}
				val.SetMapIndex(reflect.ValueOf(key), reflect.ValueOf(values[0]))
			default:
				val.SetMapIndex(reflect.ValueOf(key), reflect.ValueOf(values))
			}
		}
		return nil
	}

	if typ.Kind() != reflect.Struct {
		if tag == "param" || tag == "query" || tag == "header" {
			return nil
		}
		return errors.New("binding element must be a struct")
	}

	for i := 0; i < typ.NumField(); i++ {
		typeField := typ.Field(i)
		structField := val.Field(i)
		if typeField.Anonymous && structField.Kind() == reflect.Pointer {
			if structField.IsNil() {
				continue
			}
			structField = structField.Elem()
		}
		if !structField.CanSet() {
			continue
		}

		structFieldKind := structField.Kind()
		inputFieldName := typeField.Tag.Get(tag)
		if typeField.Anonymous && structFieldKind == reflect.Struct && inputFieldName != "" {
			return errors.New("query/param/form tags are not allowed with anonymous struct field")
		}

		if inputFieldName == "" {
			if _, ok := structField.Addr().Interface().(BindUnmarshaler); !ok && structFieldKind == reflect.Struct {
				if err := bindDataDefault(structField.Addr().Interface(), data, tag, dataFiles); err != nil {
					return err
				}
			}
			continue
		}

		if hasFiles {
			if ok, err := isFieldMultipartFile(structField.Type()); err != nil {
				return err
			} else if ok {
				if ok := setMultipartFileHeaderTypes(structField, inputFieldName, dataFiles); ok {
					continue
				}
			}
		}

		inputValue, exists := data[inputFieldName]
		if !exists {
			for key, values := range data {
				if strings.EqualFold(key, inputFieldName) {
					inputValue = values
					exists = true
					break
				}
			}
		}
		if !exists {
			continue
		}

		if ok, err := unmarshalInputsToFieldDefault(typeField.Type.Kind(), inputValue, structField); ok {
			if err != nil {
				return err
			}
			continue
		}

		formatTag := typeField.Tag.Get("format")
		if ok, err := unmarshalInputToFieldDefault(typeField.Type.Kind(), inputValue[0], structField, formatTag); ok {
			if err != nil {
				return err
			}
			continue
		}

		if structFieldKind == reflect.Pointer {
			structFieldKind = structField.Elem().Kind()
			structField = structField.Elem()
		}

		if structFieldKind == reflect.Slice {
			sliceOf := structField.Type().Elem().Kind()
			numElems := len(inputValue)
			slice := reflect.MakeSlice(structField.Type(), numElems, numElems)
			for j := 0; j < numElems; j++ {
				if err := setWithProperTypeDefault(sliceOf, inputValue[j], slice.Index(j)); err != nil {
					return err
				}
			}
			structField.Set(slice)
			continue
		}

		if err := setWithProperTypeDefault(structFieldKind, inputValue[0], structField); err != nil {
			return err
		}
	}

	return nil
}

func unmarshalInputsToFieldDefault(valueKind reflect.Kind, values []string, field reflect.Value) (bool, error) {
	if valueKind == reflect.Pointer {
		if field.IsNil() {
			field.Set(reflect.New(field.Type().Elem()))
		}
		field = field.Elem()
	}

	fieldIValue := field.Addr().Interface()
	unmarshaler, ok := fieldIValue.(bindMultipleUnmarshaler)
	if !ok {
		return false, nil
	}
	return true, unmarshaler.UnmarshalParams(values)
}

func unmarshalInputToFieldDefault(valueKind reflect.Kind, value string, field reflect.Value, formatTag string) (bool, error) {
	if valueKind == reflect.Pointer {
		if field.IsNil() {
			field.Set(reflect.New(field.Type().Elem()))
		}
		field = field.Elem()
	}

	fieldIValue := field.Addr().Interface()
	if formatTag != "" {
		if _, isTime := fieldIValue.(*time.Time); isTime {
			t, err := time.Parse(formatTag, value)
			if err != nil {
				return true, err
			}
			field.Set(reflect.ValueOf(t))
			return true, nil
		}
	}

	switch unmarshaler := fieldIValue.(type) {
	case BindUnmarshaler:
		return true, unmarshaler.UnmarshalParam(value)
	case encoding.TextUnmarshaler:
		return true, unmarshaler.UnmarshalText([]byte(value))
	}

	return false, nil
}

func setWithProperTypeDefault(valueKind reflect.Kind, value string, structField reflect.Value) error {
	if ok, err := unmarshalInputToFieldDefault(valueKind, value, structField, ""); ok {
		return err
	}

	switch valueKind {
	case reflect.Pointer:
		return setWithProperTypeDefault(structField.Elem().Kind(), value, structField.Elem())
	case reflect.Int:
		return setIntFieldDefault(value, 0, structField)
	case reflect.Int8:
		return setIntFieldDefault(value, 8, structField)
	case reflect.Int16:
		return setIntFieldDefault(value, 16, structField)
	case reflect.Int32:
		return setIntFieldDefault(value, 32, structField)
	case reflect.Int64:
		return setIntFieldDefault(value, 64, structField)
	case reflect.Uint:
		return setUintFieldDefault(value, 0, structField)
	case reflect.Uint8:
		return setUintFieldDefault(value, 8, structField)
	case reflect.Uint16:
		return setUintFieldDefault(value, 16, structField)
	case reflect.Uint32:
		return setUintFieldDefault(value, 32, structField)
	case reflect.Uint64:
		return setUintFieldDefault(value, 64, structField)
	case reflect.Bool:
		return setBoolFieldDefault(value, structField)
	case reflect.Float32:
		return setFloatFieldDefault(value, 32, structField)
	case reflect.Float64:
		return setFloatFieldDefault(value, 64, structField)
	case reflect.String:
		structField.SetString(value)
	default:
		return errors.New("unknown type")
	}
	return nil
}

func setIntFieldDefault(value string, bitSize int, field reflect.Value) error {
	if value == "" {
		value = "0"
	}
	intVal, err := strconv.ParseInt(value, 10, bitSize)
	if err == nil {
		field.SetInt(intVal)
	}
	return err
}

func setUintFieldDefault(value string, bitSize int, field reflect.Value) error {
	if value == "" {
		value = "0"
	}
	uintVal, err := strconv.ParseUint(value, 10, bitSize)
	if err == nil {
		field.SetUint(uintVal)
	}
	return err
}

func setBoolFieldDefault(value string, field reflect.Value) error {
	if value == "" {
		value = "false"
	}
	boolVal, err := strconv.ParseBool(value)
	if err == nil {
		field.SetBool(boolVal)
	}
	return err
}

func setFloatFieldDefault(value string, bitSize int, field reflect.Value) error {
	if value == "" {
		value = "0.0"
	}
	floatVal, err := strconv.ParseFloat(value, bitSize)
	if err == nil {
		field.SetFloat(floatVal)
	}
	return err
}

var (
	multipartFileHeaderType             = reflect.TypeFor[multipart.FileHeader]()
	multipartFileHeaderPointerType      = reflect.TypeFor[*multipart.FileHeader]()
	multipartFileHeaderSliceType        = reflect.TypeFor[[]multipart.FileHeader]()
	multipartFileHeaderPointerSliceType = reflect.TypeFor[[]*multipart.FileHeader]()
)

func isFieldMultipartFile(field reflect.Type) (bool, error) {
	switch field {
	case multipartFileHeaderPointerType, multipartFileHeaderSliceType, multipartFileHeaderPointerSliceType:
		return true, nil
	case multipartFileHeaderType:
		return true, errors.New("binding to multipart.FileHeader struct is not supported, use pointer to struct")
	default:
		return false, nil
	}
}

func setMultipartFileHeaderTypes(structField reflect.Value, inputFieldName string, files map[string][]*multipart.FileHeader) bool {
	fileHeaders := files[inputFieldName]
	if len(fileHeaders) == 0 {
		return false
	}

	result := true
	switch structField.Type() {
	case multipartFileHeaderPointerSliceType:
		structField.Set(reflect.ValueOf(fileHeaders))
	case multipartFileHeaderSliceType:
		headers := make([]multipart.FileHeader, len(fileHeaders))
		for i, fileHeader := range fileHeaders {
			headers[i] = *fileHeader
		}
		structField.Set(reflect.ValueOf(headers))
	case multipartFileHeaderPointerType:
		structField.Set(reflect.ValueOf(fileHeaders[0]))
	default:
		result = false
	}

	return result
}

func pathWildcardNames(pattern string) []string {
	pattern = strings.TrimSpace(pattern)
	if pattern == "" {
		return nil
	}

	names := make([]string, 0, 2)
	for i := 0; i < len(pattern); i++ {
		if pattern[i] != '{' {
			continue
		}

		end := strings.IndexByte(pattern[i+1:], '}')
		if end < 0 {
			break
		}

		token := strings.TrimSpace(pattern[i+1 : i+1+end])
		token = strings.TrimSuffix(token, "...")
		token, _, _ = strings.Cut(token, ":")
		token = strings.TrimSpace(token)
		if token != "" && token != "$" {
			names = append(names, token)
		}

		i += end + 1
	}

	return names
}

var errRequestTooLarge = errors.New("bind: request body too large")

func readBody(body io.ReadCloser, maxBytes int64) ([]byte, error) {
	if body == nil {
		return nil, nil
	}
	if maxBytes <= 0 {
		maxBytes = defaultMaxBodyBytes
	}

	data, err := io.ReadAll(io.LimitReader(body, maxBytes+1))
	if err != nil {
		return nil, err
	}
	if int64(len(data)) > maxBytes {
		return nil, errRequestTooLarge
	}
	return data, nil
}

func unsupportedMediaTypeError() error {
	return errx.NewHTTPError(
		http.StatusUnsupportedMediaType,
		CodeUnsupportedMediaType,
		"Content-Type must be application/json",
	)
}

func requestTooLargeError() error {
	return errx.NewHTTPError(
		http.StatusRequestEntityTooLarge,
		CodeRequestTooLarge,
		"request body is too large",
	)
}

func errorsf(format string, args ...any) error {
	return fmt.Errorf("bind: "+format, args...)
}
