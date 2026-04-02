package reqx

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestBindAndValidateWrappersReturnBindErrors(t *testing.T) {
	var dst struct{}

	if err := BindAndValidate[struct{}](nil, &dst); err == nil || err.Error() != "reqx: request must not be nil" {
		t.Fatalf("BindAndValidate() error = %v", err)
	}
	if err := BindAndValidateBody[struct{}](nil, &dst); err == nil || err.Error() != "reqx: request must not be nil" {
		t.Fatalf("BindAndValidateBody() error = %v", err)
	}
	if err := BindAndValidateQuery[struct{}](nil, &dst); err == nil || err.Error() != "reqx: request must not be nil" {
		t.Fatalf("BindAndValidateQuery() error = %v", err)
	}
	if err := BindAndValidatePath[struct{}](nil, &dst); err == nil || err.Error() != "reqx: request must not be nil" {
		t.Fatalf("BindAndValidatePath() error = %v", err)
	}
	if err := BindAndValidateHeaders[struct{}](nil, &dst); err == nil || err.Error() != "reqx: request must not be nil" {
		t.Fatalf("BindAndValidateHeaders() error = %v", err)
	}
}

func TestBindAndValidateWrappersSuccessPaths(t *testing.T) {
	type bodyRequest struct {
		Name string `json:"name" validate:"required"`
	}
	bodyReq := newJSONRequest(http.MethodPost, "/", `{"name":"kanata"}`)
	var bodyDst bodyRequest
	if err := BindAndValidateBody(bodyReq, &bodyDst); err != nil {
		t.Fatalf("BindAndValidateBody() error = %v", err)
	}

	type requestRequest struct {
		ID string `param:"id" validate:"required"`
	}
	req := requestWithPathParams(map[string][]string{"id": {"route-id"}})
	req.Method = http.MethodGet
	req.URL.RawQuery = "ignored=1"
	var requestDst requestRequest
	if err := BindAndValidate(req, &requestDst); err != nil {
		t.Fatalf("BindAndValidate() error = %v", err)
	}

	type queryRequest struct {
		Cursor string `query:"cursor" validate:"required"`
	}
	queryReq := httptest.NewRequest(http.MethodGet, "/?cursor=abc", nil)
	var queryDst queryRequest
	if err := BindAndValidateQuery(queryReq, &queryDst); err != nil {
		t.Fatalf("BindAndValidateQuery() error = %v", err)
	}

	type pathRequest struct {
		UUID string `param:"uuid" validate:"required"`
	}
	pathReq := requestWithPathParams(map[string][]string{"uuid": {"u_1"}})
	var pathDst pathRequest
	if err := BindAndValidatePath(pathReq, &pathDst); err != nil {
		t.Fatalf("BindAndValidatePath() error = %v", err)
	}

	type headerRequest struct {
		RequestID string `header:"x-request-id" validate:"required"`
	}
	headerReq := httptest.NewRequest(http.MethodGet, "/", nil)
	headerReq.Header.Set("X-Request-Id", "req-1")
	var headerDst headerRequest
	if err := BindAndValidateHeaders(headerReq, &headerDst); err != nil {
		t.Fatalf("BindAndValidateHeaders() error = %v", err)
	}
}

func TestValidateAndNormalizeHelpers(t *testing.T) {
	type request struct {
		Name string
	}

	if err := Validate(&request{Name: "ok"}, nil); err != nil {
		t.Fatalf("Validate(nil fn) error = %v", err)
	}
	if err := Validate(&request{Name: "ok"}, func(*request) []Violation { return nil }); err != nil {
		t.Fatalf("Validate(no violations) error = %v", err)
	}
}

func TestValidateBranches(t *testing.T) {
	type request struct {
		Name string `validate:"required"`
	}

	if err := validate(&request{Name: "ok"}, sourceBody); err != nil {
		t.Fatalf("validate(success) error = %v", err)
	}

	var nilTarget *request
	if err := validate(nilTarget, sourceBody); err == nil || err.Error() != "reqx: target must be a non-nil pointer to struct" {
		t.Fatalf("validate(typed nil) error = %v", err)
	}

	value := 1
	if err := validate(&value, sourceBody); err == nil || err.Error() != "reqx: target must be a non-nil pointer to struct" {
		t.Fatalf("validate(non-struct) error = %v", err)
	}
}

func TestValidateStructValidationErrors(t *testing.T) {
	target := &struct {
		Name string `json:"name" validate:"required"`
	}{}

	violations, err := validateStruct(target, sourceBody)
	if err != nil {
		t.Fatalf("validateStruct() error = %v", err)
	}
	if len(violations) != 1 {
		t.Fatalf("violations len = %d, want 1", len(violations))
	}
	if violations[0].Field != "name" || violations[0].Code != ViolationCodeRequired || violations[0].Message != "is required" {
		t.Fatalf("violations[0] = %#v", violations[0])
	}
}

func TestNormalizeViolationBranches(t *testing.T) {
	testCases := []struct {
		name string
		in   Violation
		want Violation
	}{
		{
			name: "required",
			in:   Violation{Field: "name", Code: ViolationCodeRequired},
			want: Violation{Field: "name", Code: ViolationCodeRequired, Message: "is required"},
		},
		{
			name: "unknown",
			in:   Violation{Field: "name", Code: ViolationCodeUnknown},
			want: Violation{Field: "name", Code: ViolationCodeUnknown, Message: "unknown field"},
		},
		{
			name: "type",
			in:   Violation{Field: "name", Code: ViolationCodeType},
			want: Violation{Field: "name", Code: ViolationCodeType, Message: "has invalid type"},
		},
		{
			name: "default",
			in:   Violation{Field: "name"},
			want: Violation{Field: "name", Code: ViolationCodeInvalid, Message: "is invalid"},
		},
		{
			name: "explicit message",
			in:   Violation{Field: "name", Code: ViolationCodeInvalid, Message: "custom"},
			want: Violation{Field: "name", Code: ViolationCodeInvalid, Message: "custom"},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			if got := normalizeViolation(tc.in); got != tc.want {
				t.Fatalf("normalizeViolation() = %#v, want %#v", got, tc.want)
			}
		})
	}
}
