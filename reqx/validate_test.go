package reqx

import (
	"net/http"
	"net/http/httptest"
	"reflect"
	"testing"
)

func TestBindHeaders_RejectsRepeatedScalar(t *testing.T) {
	type request struct {
		RequestID string `header:"x-request-id"`
	}

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Add("X-Request-Id", "req-1")
	req.Header.Add("X-Request-Id", "req-2")

	var dst request
	err := BindHeaders(req, &dst)
	violation := assertSingleViolation(t, err)
	if violation.Field != "X-Request-Id" || violation.Code != ViolationCodeMultiple || violation.Message != "must not be repeated" {
		t.Fatalf("violation = %#v", violation)
	}
}

func TestBindAndValidateHeaders_UsesCanonicalHeaderTagName(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/", nil)

	var dst struct {
		RequestID string `header:"x-request-id" validate:"required"`
	}
	err := BindAndValidateHeaders(req, &dst)
	violation := assertSingleViolation(t, err)
	if violation.Field != "X-Request-Id" || violation.Code != ViolationCodeRequired || violation.Message != "is required" {
		t.Fatalf("violation = %#v", violation)
	}
}

func TestValidateBody_UsesJSONTagName(t *testing.T) {
	var dst struct {
		DisplayName string `json:"display_name" validate:"required,nospace"`
	}

	err := ValidateBody(&dst)
	violation := assertSingleViolation(t, err)
	if violation.Field != "display_name" || violation.Code != ViolationCodeRequired || violation.Message != "is required" {
		t.Fatalf("violation = %#v", violation)
	}
}

func TestValidatePath_UsesParamTagName(t *testing.T) {
	var dst struct {
		UUID string `param:"uuid" validate:"required,uuid"`
	}

	err := ValidatePath(&dst)
	violation := assertSingleViolation(t, err)
	if violation.Field != "uuid" || violation.Code != ViolationCodeRequired || violation.Message != "is required" {
		t.Fatalf("violation = %#v", violation)
	}
}

func TestValidate_CustomValidationNormalizesViolation(t *testing.T) {
	dst := struct {
		Name string
	}{}

	err := Validate(&dst, func(_ *struct{ Name string }) []Violation {
		return []Violation{{Field: "name"}}
	})
	violation := assertSingleViolation(t, err)
	if violation.Field != "name" || violation.Code != ViolationCodeInvalid || violation.Message != "is invalid" {
		t.Fatalf("violation = %#v", violation)
	}
}

func TestValidate_NilDestinationReturnsError(t *testing.T) {
	err := Validate[struct{}](nil, func(_ *struct{}) []Violation { return nil })
	if err == nil {
		t.Fatal("Validate() error = nil")
	}
	if got := err.Error(); got != "reqx: destination must not be nil" {
		t.Fatalf("error = %q, want reqx: destination must not be nil", got)
	}
}

func TestBadRequest_ReturnsHTTPError(t *testing.T) {
	err := BadRequest(RequiredField("name"))
	httpErr := assertHTTPError(t, err, http.StatusBadRequest, CodeInvalidRequest, "request contains invalid fields")

	details := httpErr.Details()
	if len(details) != 1 {
		t.Fatalf("details len = %d, want 1", len(details))
	}

	violation, ok := details[0].(Violation)
	if !ok {
		t.Fatalf("detail type = %T, want reqx.Violation", details[0])
	}
	if violation.Field != "name" || violation.Code != ViolationCodeRequired || violation.Message != "is required" {
		t.Fatalf("violation = %#v", violation)
	}
}

func TestApplyBindOptions_SetsAllFlags(t *testing.T) {
	cfg := applyBindOptions(WithMaxBodyBytes(8))

	if cfg.body.maxBodyBytes != 8 {
		t.Fatalf("maxBodyBytes = %d, want 8", cfg.body.maxBodyBytes)
	}
	if !cfg.body.allowUnknownFields {
		t.Fatal("body.allowUnknownFields = false, want true")
	}
	if !cfg.body.allowEmptyBody {
		t.Fatal("body.allowEmptyBody = false, want true")
	}
	if !cfg.query.allowUnknownFields {
		t.Fatal("query.allowUnknownFields = false, want true")
	}
	if !cfg.header.allowUnknownFields {
		t.Fatal("header.allowUnknownFields = false, want true")
	}
}

func TestValidatorFor_PanicsOnUnsupportedSource(t *testing.T) {
	defer func() {
		if recover() == nil {
			t.Fatal("validatorFor() did not panic")
		}
	}()

	_ = validatorFor(sourceKind("unsupported"))
}

func TestSourceTagPriority_PanicsOnUnsupportedSource(t *testing.T) {
	defer func() {
		if recover() == nil {
			t.Fatal("sourceTagPriority() did not panic")
		}
	}()

	_ = sourceTagPriority(sourceKind("unsupported"))
}

func TestSourceTagPriority_UsesBodyPriority(t *testing.T) {
	got := sourceTagPriority(sourceBody)
	want := []string{"json", "query", "param", "header"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("sourceTagPriority(sourceBody) = %#v, want %#v", got, want)
	}
}
