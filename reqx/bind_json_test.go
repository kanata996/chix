package reqx

import (
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// 非 JSON Content-Type 会被拒绝。
func TestBindBody_RejectsUnsupportedMediaType(t *testing.T) {
	type request struct {
		Name string `json:"name"`
	}

	req := newJSONRequest(http.MethodPost, "/", `{"name":"kanata"}`)
	req.Header.Set("Content-Type", "text/plain")

	var dst request
	err := BindBody(req, &dst)
	_ = assertHTTPError(t, err, http.StatusUnsupportedMediaType, CodeUnsupportedMediaType, "Content-Type must be application/json or application/*+json")
}

// 非法的媒体类型头会被拒绝。
func TestBindBody_RejectsInvalidMediaTypeHeader(t *testing.T) {
	type request struct {
		Name string `json:"name"`
	}

	req := newJSONRequest(http.MethodPost, "/", `{"name":"kanata"}`)
	req.Header.Set("Content-Type", `application/json; charset="`)

	var dst request
	err := BindBody(req, &dst)
	_ = assertHTTPError(t, err, http.StatusUnsupportedMediaType, CodeUnsupportedMediaType, "Content-Type must be application/json or application/*+json")
}

// 默认配置下，空 body 会被忽略。
func TestBindBody_IgnoresEmptyBodyByDefault(t *testing.T) {
	type request struct {
		Name string `json:"name"`
	}

	req := newJSONRequest(http.MethodPost, "/", "")

	var dst request
	if err := BindBody(req, &dst); err != nil {
		t.Fatalf("BindBody() error = %v", err)
	}
}

// 默认配置下，纯空白 body 会被视为空 body 并忽略。
func TestBindBody_IgnoresWhitespaceOnlyBodyByDefault(t *testing.T) {
	type request struct {
		Name string `json:"name"`
	}

	req := newJSONRequest(http.MethodPost, "/", " \n\t ")

	var dst request
	if err := BindBody(req, &dst); err != nil {
		t.Fatalf("BindBody() error = %v", err)
	}
	if dst.Name != "" {
		t.Fatalf("name = %q, want empty", dst.Name)
	}
}

// 默认配置下，未知字段会被忽略。
func TestBindBody_IgnoresUnknownFieldsByDefault(t *testing.T) {
	type request struct {
		Name string `json:"name"`
	}

	req := newJSONRequest(http.MethodPost, "/", `{"name":"kanata","extra":1}`)

	var dst request
	if err := BindBody(req, &dst); err != nil {
		t.Fatalf("BindBody() error = %v", err)
	}
	if dst.Name != "kanata" {
		t.Fatalf("name = %q, want kanata", dst.Name)
	}
}

// 字段类型不匹配时返回类型错误。
func TestBindBody_RejectsTypeMismatch(t *testing.T) {
	type request struct {
		Age int `json:"age"`
	}

	req := newJSONRequest(http.MethodPost, "/", `{"age":"x"}`)

	var dst request
	err := BindBody(req, &dst)
	violation := assertSingleViolation(t, err)
	if violation.Field != "age" || violation.Code != ViolationCodeType || violation.Message != "must be number" {
		t.Fatalf("violation = %#v", violation)
	}
}

// 非法 JSON 会被拒绝。
func TestBindBody_RejectsInvalidJSON(t *testing.T) {
	type request struct {
		Name string `json:"name"`
	}

	req := newJSONRequest(http.MethodPost, "/", `{"name":`)

	var dst request
	err := BindBody(req, &dst)
	_ = assertHTTPError(t, err, http.StatusBadRequest, CodeInvalidJSON, "request body must be valid JSON")
}

// 多个顶层 JSON 值会被拒绝。
func TestBindBody_RejectsMultipleJSONValues(t *testing.T) {
	type request struct {
		Name string `json:"name"`
	}

	req := newJSONRequest(http.MethodPost, "/", `{"name":"a"}{"name":"b"}`)

	var dst request
	err := BindBody(req, &dst)
	_ = assertHTTPError(t, err, http.StatusBadRequest, CodeInvalidJSON, "request body must contain a single JSON value")
}

// body 超过限制时返回过大错误。
func TestBindBody_RespectsMaxBodyBytes(t *testing.T) {
	type request struct {
		Name string `json:"name"`
	}

	req := newJSONRequest(http.MethodPost, "/", `{"name":"kanata"}`)

	var dst request
	err := BindBody(req, &dst, WithMaxBodyBytes(4))
	_ = assertHTTPError(t, err, http.StatusRequestEntityTooLarge, CodeRequestTooLarge, "request body is too large")
}

// 请求对象不能为空。
func TestBindBody_RequestMustNotBeNil(t *testing.T) {
	var dst struct{}
	err := BindBody[struct{}](nil, &dst)
	if err == nil {
		t.Fatal("BindBody() error = nil")
	}
	if got := err.Error(); got != "reqx: request must not be nil" {
		t.Fatalf("error = %q, want reqx: request must not be nil", got)
	}
}

// 空 body 忽略模式下，不校验 Content-Type。
func TestBindJSONWithConfig_IgnoresEmptyBodyWithoutCheckingContentType(t *testing.T) {
	req := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(" \n\t "))
	req.Header.Set("Content-Type", "text/plain")

	var dst struct {
		Name string `json:"name"`
	}
	err := bindJSONWithConfig(req, &dst, bindBodyConfig{}, bodyBindMode{
		ignoreEmptyBody: true,
	})
	if err != nil {
		t.Fatalf("bindJSONWithConfig() error = %v", err)
	}
}

// 非正数限制会回退到默认 body 大小上限。
func TestReadBody_UsesDefaultMaxBodyBytesWhenLimitIsNonPositive(t *testing.T) {
	body := io.NopCloser(strings.NewReader(strings.Repeat("a", int(DefaultMaxBodyBytes)+1)))

	_, err := readBody(body, 0)
	if !errors.Is(err, errRequestTooLarge) {
		t.Fatalf("readBody() error = %v, want %v", err, errRequestTooLarge)
	}
}

// 带前后空白的 +json 媒体类型也应视为合法。
func TestValidateJSONContentType_AllowsWhitespaceWrappedJSONSuffixMediaType(t *testing.T) {
	if err := validateJSONContentType(" application/problem+json ; charset=utf-8 "); err != nil {
		t.Fatalf("validateJSONContentType() error = %v", err)
	}
}

// 未知字段错误消息格式不完整时，回退为通用非法 JSON 错误。
func TestMapDecodeError_FallsBackToInvalidJSONForMalformedUnknownFieldMessage(t *testing.T) {
	err := mapDecodeError(errors.New(`json: unknown field "extra`))
	_ = assertHTTPError(t, err, http.StatusBadRequest, CodeInvalidJSON, "request body must be valid JSON")
}

// 绑定后会先做标准化再校验。
func TestBindAndValidateBody_NormalizesBeforeValidation(t *testing.T) {
	req := newJSONRequest(http.MethodPost, "/", `{"name":"  kanata  "}`)

	var dst normalizedBodyRequest
	if err := BindAndValidateBody(req, &dst); err != nil {
		t.Fatalf("BindAndValidateBody() error = %v", err)
	}
	if dst.Name != "kanata" {
		t.Fatalf("name = %q, want kanata", dst.Name)
	}
}

type normalizedBodyRequest struct {
	Name string `json:"name" validate:"required,nospace"`
}

func (r *normalizedBodyRequest) Normalize() {
	r.Name = strings.TrimSpace(r.Name)
}

// 校验错误字段名使用 json tag 名称。
func TestBindAndValidateBody_UsesJSONTagNameInValidationError(t *testing.T) {
	req := newJSONRequest(http.MethodPost, "/", `{"display_name":"kanata aqua"}`)

	var dst struct {
		DisplayName string `json:"display_name" validate:"required,nospace"`
	}
	err := BindAndValidateBody(req, &dst)
	violation := assertSingleViolation(t, err)
	if violation.Field != "display_name" || violation.Code != ViolationCodeInvalid || violation.Message != "is invalid" {
		t.Fatalf("violation = %#v", violation)
	}
}
