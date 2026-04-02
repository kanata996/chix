package reqx

import (
	"net/http"
	"strings"
	"testing"
)

func TestBindBody_RejectsUnsupportedMediaType(t *testing.T) {
	type request struct {
		Name string `json:"name"`
	}

	req := newJSONRequest(http.MethodPost, "/", `{"name":"kanata"}`)
	req.Header.Set("Content-Type", "text/plain")

	var dst request
	err := BindBody(req, &dst)
	_ = assertHTTPError(t, err, http.StatusUnsupportedMediaType, CodeUnsupportedMediaType, "Content-Type must be application/json")
}

func TestBindBody_RejectsInvalidMediaTypeHeader(t *testing.T) {
	type request struct {
		Name string `json:"name"`
	}

	req := newJSONRequest(http.MethodPost, "/", `{"name":"kanata"}`)
	req.Header.Set("Content-Type", `application/json; charset="`)

	var dst request
	err := BindBody(req, &dst)
	_ = assertHTTPError(t, err, http.StatusUnsupportedMediaType, CodeUnsupportedMediaType, "Content-Type must be application/json")
}

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

func TestBindBody_RejectsInvalidJSON(t *testing.T) {
	type request struct {
		Name string `json:"name"`
	}

	req := newJSONRequest(http.MethodPost, "/", `{"name":`)

	var dst request
	err := BindBody(req, &dst)
	_ = assertHTTPError(t, err, http.StatusBadRequest, CodeInvalidJSON, "request body must be valid JSON")
}

func TestBindBody_RejectsMultipleJSONValues(t *testing.T) {
	type request struct {
		Name string `json:"name"`
	}

	req := newJSONRequest(http.MethodPost, "/", `{"name":"a"}{"name":"b"}`)

	var dst request
	err := BindBody(req, &dst)
	_ = assertHTTPError(t, err, http.StatusBadRequest, CodeInvalidJSON, "request body must contain a single JSON value")
}

func TestBindBody_RespectsMaxBodyBytes(t *testing.T) {
	type request struct {
		Name string `json:"name"`
	}

	req := newJSONRequest(http.MethodPost, "/", `{"name":"kanata"}`)

	var dst request
	err := BindBody(req, &dst, WithMaxBodyBytes(4))
	_ = assertHTTPError(t, err, http.StatusRequestEntityTooLarge, CodeRequestTooLarge, "request body is too large")
}

func TestBindBody_RequestMustNotBeNil(t *testing.T) {
	var dst struct{}
	err := BindBody[struct{}](nil, &dst)
	if err == nil {
		t.Fatal("BindBody() error = nil")
	}
	if got := err.Error(); got != "reqx: request must not be nil" {
		t.Fatalf("error = %q", got)
	}
}

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
