package reqx

// 用例清单：
// - 标记说明：[✓] 已核对且已有真实覆盖；[x] 本轮审查发现缺口后补测。
// - [✓] `ValidateBody`、`ValidateQuery`、`ValidatePath`、`ValidateHeaders` 会使用来源专属字段别名和 `in` 值。
// - [✓] `Validate` 会规范化自定义 violation，且空目标返回参数错误。
// - [✓] `BadRequest` 会生成带 violation 详情的 HTTP 错误。
// - [✓] `ValidateRequest` 会按 param/query/json/header/plain 选择字段名，但统一以 `request` 作为 `in`。
// - [✓] `ValidateRequest` 的空目标错误契约会透传当前包装器的参数校验错误。

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

// ValidateRequest 会按请求标签选择字段别名，但统一以 request 作为 in 值。
func TestValidateRequest_UsesRequestFieldAliases(t *testing.T) {
	var dst struct {
		ID      string `param:"id" validate:"required"`
		Cursor  string `query:"cursor" validate:"required"`
		Name    string `json:"name" validate:"required"`
		TraceID string `header:"x-trace-id" validate:"required"`
		Plain   string `validate:"required"`
	}

	violations := assertViolations(t, ValidateRequest(&dst))
	if len(violations) != 5 {
		t.Fatalf("violations len = %d, want 5", len(violations))
	}

	got := map[string]Violation{}
	for _, violation := range violations {
		got[violation.Field] = violation
	}

	want := map[string]string{
		"id":         ViolationInRequest,
		"cursor":     ViolationInRequest,
		"name":       ViolationInRequest,
		"X-Trace-Id": ViolationInRequest,
		"Plain":      ViolationInRequest,
	}
	for field, wantIn := range want {
		violation, ok := got[field]
		if !ok {
			t.Fatalf("missing violation for %q in %#v", field, got)
		}
		if violation.In != wantIn || violation.Code != ViolationCodeRequired || violation.Detail != "is required" {
			t.Fatalf("violation[%q] = %#v", field, violation)
		}
	}
}

// ValidateRequest 会透传当前包装器的空目标参数错误。
func TestValidateRequest_NilDestinationReturnsError(t *testing.T) {
	err := ValidateRequest[struct{}](nil)
	if err == nil {
		t.Fatal("ValidateRequest() error = nil")
	}
	if got := err.Error(); got != "reqx: target must be a non-nil pointer to struct" {
		t.Fatalf("error = %q, want reqx: target must be a non-nil pointer to struct", got)
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
