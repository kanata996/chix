package reqx

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/yogorobot/app-mall/chix/resp"
)

func newJSONRequest(method, target, body string) *http.Request {
	req := httptest.NewRequest(method, target, strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	return req
}

func assertHTTPError(t *testing.T, err error, wantStatus int, wantCode, wantMessage string) *resp.HTTPError {
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
	if got := httpErr.Message(); got != wantMessage {
		t.Fatalf("message = %q, want %q", got, wantMessage)
	}
	return httpErr
}

func assertSingleViolation(t *testing.T, err error) Violation {
	t.Helper()

	httpErr := assertHTTPError(
		t,
		err,
		http.StatusUnprocessableEntity,
		CodeInvalidRequest,
		"request contains invalid fields",
	)

	details := httpErr.Details()
	if len(details) != 1 {
		t.Fatalf("details len = %d, want 1", len(details))
	}

	violation, ok := details[0].(Violation)
	if !ok {
		t.Fatalf("detail type = %T, want reqx.Violation", details[0])
	}
	return violation
}
