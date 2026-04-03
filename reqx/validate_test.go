package reqx

import (
	"net/http"
	"testing"
)

// 各个 Validate* API 会按来源选择稳定的字段别名和 in 值。
func TestValidate_UsesSourceSpecificFieldAliases(t *testing.T) {
	t.Run("body uses json tag", func(t *testing.T) {
		var dst struct {
			DisplayName string `json:"display_name" validate:"required,nospace"`
		}

		violation := assertSingleViolation(t, ValidateBody(&dst))
		if violation.Field != "display_name" || violation.In != ViolationInBody || violation.Code != ViolationCodeRequired || violation.Detail != "is required" {
			t.Fatalf("violation = %#v", violation)
		}
	})

	t.Run("query uses query tag", func(t *testing.T) {
		var dst struct {
			Cursor string `query:"cursor" validate:"required"`
		}

		violation := assertSingleViolation(t, ValidateQuery(&dst))
		if violation.Field != "cursor" || violation.In != ViolationInQuery || violation.Code != ViolationCodeRequired || violation.Detail != "is required" {
			t.Fatalf("violation = %#v", violation)
		}
	})

	t.Run("path uses param tag", func(t *testing.T) {
		var dst struct {
			UUID string `param:"uuid" validate:"required,uuid"`
		}

		violation := assertSingleViolation(t, ValidatePath(&dst))
		if violation.Field != "uuid" || violation.In != ViolationInPath || violation.Code != ViolationCodeRequired || violation.Detail != "is required" {
			t.Fatalf("violation = %#v", violation)
		}
	})

	t.Run("headers use canonical header tag", func(t *testing.T) {
		var dst struct {
			RequestID string `header:"x-request-id" validate:"required"`
		}

		violation := assertSingleViolation(t, ValidateHeaders(&dst))
		if violation.Field != "X-Request-Id" || violation.In != ViolationInHeader || violation.Code != ViolationCodeRequired || violation.Detail != "is required" {
			t.Fatalf("violation = %#v", violation)
		}
	})
}

// 自定义校验返回的 violation 会被补齐默认错误信息。
func TestValidate_CustomValidationNormalizesViolation(t *testing.T) {
	dst := struct {
		Name string
	}{}

	err := Validate(&dst, func(_ *struct{ Name string }) []Violation {
		return []Violation{{Field: "name"}}
	})
	violation := assertSingleViolation(t, err)
	if violation.Field != "name" || violation.Code != ViolationCodeInvalid || violation.Detail != "is invalid" {
		t.Fatalf("violation = %#v", violation)
	}
}

// 自定义 Validate 在目标对象为空时返回参数错误。
func TestValidate_NilDestinationReturnsError(t *testing.T) {
	err := Validate[struct{}](nil, func(_ *struct{}) []Violation { return nil })
	if err == nil {
		t.Fatal("Validate() error = nil")
	}
	if got := err.Error(); got != "reqx: destination must not be nil" {
		t.Fatalf("error = %q, want reqx: destination must not be nil", got)
	}
}

// BadRequest 会构造带 violation 详情的 HTTP 错误。
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
	if violation.Field != "name" || violation.In != ViolationInBody || violation.Code != ViolationCodeRequired || violation.Detail != "is required" {
		t.Fatalf("violation = %#v", violation)
	}
}
