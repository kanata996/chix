package reqx

import (
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// 非空 body 需要明确声明 JSON Content-Type；application/json 和 application/*+json 均可接受。
func TestBindBody_ContentTypeContract(t *testing.T) {
	type request struct {
		Name string `json:"name"`
	}

	testCases := []struct {
		name          string
		contentType   string
		setHeader     bool
		wantName      string
		wantErrStatus int
	}{
		{
			name:          "rejects unsupported media type",
			contentType:   "text/plain",
			setHeader:     true,
			wantErrStatus: http.StatusUnsupportedMediaType,
		},
		{
			name:          "rejects malformed media type header",
			contentType:   `application/json; charset="`,
			setHeader:     true,
			wantErrStatus: http.StatusUnsupportedMediaType,
		},
		{
			name:          "rejects missing content type",
			setHeader:     false,
			wantErrStatus: http.StatusUnsupportedMediaType,
		},
		{
			name:        "allows parameterized application json",
			contentType: "application/json; charset=utf-8",
			setHeader:   true,
			wantName:    "kanata",
		},
		{
			name:        "allows json suffix media type",
			contentType: "application/problem+json",
			setHeader:   true,
			wantName:    "kanata",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(`{"name":"kanata"}`))
			if tc.setHeader {
				req.Header.Set("Content-Type", tc.contentType)
			}

			var dst request
			err := BindBody(req, &dst)
			if tc.wantErrStatus != 0 {
				_ = assertHTTPError(t, err, tc.wantErrStatus, CodeUnsupportedMediaType, "Content-Type must be application/json or application/*+json")
				return
			}
			if err != nil {
				t.Fatalf("BindBody() error = %v", err)
			}
			if dst.Name != tc.wantName {
				t.Fatalf("name = %q, want %q", dst.Name, tc.wantName)
			}
		})
	}
}

// 空 body 且未声明 Content-Type 时仍会按 no-op 处理。
func TestBindBody_IgnoresEmptyBodyWithoutContentType(t *testing.T) {
	type request struct {
		Name string `json:"name"`
	}

	req := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(""))
	dst := request{Name: "kanata"}

	if err := BindBody(req, &dst); err != nil {
		t.Fatalf("BindBody() error = %v", err)
	}
	if dst.Name != "kanata" {
		t.Fatalf("name = %q, want kanata", dst.Name)
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

// 成功绑定时，未出现在 JSON 里的字段应保留已有值。
func TestBindBody_PreservesExistingValuesForOmittedFields(t *testing.T) {
	type request struct {
		Name string `json:"name"`
		Age  int    `json:"age"`
	}

	req := newJSONRequest(http.MethodPost, "/", `{"name":"new-name"}`)
	dst := request{
		Name: "existing-name",
		Age:  17,
	}

	if err := BindBody(req, &dst); err != nil {
		t.Fatalf("BindBody() error = %v", err)
	}
	if dst.Name != "new-name" || dst.Age != 17 {
		t.Fatalf("dst = %#v, want name updated and age preserved", dst)
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

// 单个 JSON 值后附带空白仍应被视为合法。
func TestBindBody_AllowsTrailingWhitespaceAfterSingleJSONValue(t *testing.T) {
	type request struct {
		Name string `json:"name"`
	}

	req := newJSONRequest(http.MethodPost, "/", "{\"name\":\"kanata\"} \n\t ")

	var dst request
	if err := BindBody(req, &dst); err != nil {
		t.Fatalf("BindBody() error = %v", err)
	}
	if dst.Name != "kanata" {
		t.Fatalf("name = %q, want kanata", dst.Name)
	}
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

// body 大小恰好等于限制时仍应允许通过，避免 off-by-one。
func TestBindBody_AllowsBodyAtExactMaxBodyBytes(t *testing.T) {
	type request struct {
		Name string `json:"name"`
	}

	body := `{"name":"kanata"}`
	req := newJSONRequest(http.MethodPost, "/", body)

	var dst request
	if err := BindBody(req, &dst, WithMaxBodyBytes(int64(len(body)))); err != nil {
		t.Fatalf("BindBody() error = %v", err)
	}
	if dst.Name != "kanata" {
		t.Fatalf("name = %q, want kanata", dst.Name)
	}
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
