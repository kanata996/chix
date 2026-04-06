package reqx

// 用例清单：
// - [✓] 测试辅助文件：提供 JSON 请求构造与 HTTP/violation 断言辅助，无独立业务用例。

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/kanata996/chix/resp"
)

func newJSONRequest(method, target, body string) *http.Request {
	req := httptest.NewRequest(method, target, strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	return req
}

func assertHTTPError(t *testing.T, err error, wantStatus int, wantCode, wantDetail string) *resp.HTTPError {
	t.Helper()

	httpErr, ok := err.(*resp.HTTPError)
	if !ok {
		t.Fatalf("error type = %T, want *resp.HTTPError", err)
	}
	if got := httpErr.Status(); got != wantStatus {
		t.Fatalf("status = %d, want %d", got, wantStatus)
	}
	if got := httpErr.Code(); got != wantCode {
		t.Fatalf("code = %q, want %q", got, wantCode)
	}
	if got := httpErr.Detail(); got != wantDetail {
		t.Fatalf("detail = %q, want %q", got, wantDetail)
	}
	return httpErr
}

func assertViolations(t *testing.T, err error) []Violation {
	t.Helper()

	httpErr := assertHTTPError(
		t,
		err,
		http.StatusUnprocessableEntity,
		CodeInvalidRequest,
		"request contains invalid fields",
	)

	details := httpErr.Errors()
	violations := make([]Violation, 0, len(details))
	for i, detail := range details {
		violation, ok := detail.(Violation)
		if !ok {
			t.Fatalf("detail[%d] type = %T, want reqx.Violation", i, detail)
		}
		violations = append(violations, violation)
	}
	return violations
}

func assertSingleViolation(t *testing.T, err error) Violation {
	t.Helper()

	violations := assertViolations(t, err)
	if len(violations) != 1 {
		t.Fatalf("details len = %d, want 1", len(violations))
	}
	return violations[0]
}
