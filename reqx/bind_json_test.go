package reqx

import (
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

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
