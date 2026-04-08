package reqx

import (
	"errors"
	"io"
	"net/http"
	"reflect"
	"strings"
	"testing"
)

// 测试清单：
// - 标记说明：[✓] 已核对且已有真实覆盖；[x] 尚未完成，不得作为验收依据。
// - [✓] `RequireBody` 会沿用默认 binder 的 empty-body 判定并返回统一 invalid_request。
// - [✓] `BindAndValidate*` 会在字段校验之前执行请求级规则，并允许规则读取 Normalize 后的 DTO。
// - [✓] mixed-source DTO 可通过 `ValidateRequest` 为可选字段 body 显式声明 body-required 契约。

type requestRuleNormalizedRequest struct {
	Name   string    `json:"name" validate:"required,nospace"`
	events *[]string `json:"-"`
}

func (r *requestRuleNormalizedRequest) Normalize() {
	*r.events = append(*r.events, "normalize")
	r.Name = strings.TrimSpace(r.Name)
}

func (r *requestRuleNormalizedRequest) ValidateRequest(*http.Request) error {
	*r.events = append(*r.events, "request")
	if r.Name != "kanata" {
		return errors.New("request validator saw unnormalized name")
	}
	return nil
}

type requestRuleFailureRequest struct {
	Name string `json:"name" validate:"required"`
}

func (*requestRuleFailureRequest) ValidateRequest(*http.Request) error {
	return errRequestRuleFailed
}

type requestRuleRequireBodyRequest struct {
	OrgID string  `param:"org_id" validate:"required"`
	Name  *string `json:"name"`
}

func (*requestRuleRequireBodyRequest) ValidateRequest(r *http.Request) error {
	return RequireBody(r)
}

var errRequestRuleFailed = errors.New("request rule failed")

func TestRequireBody(t *testing.T) {
	req := newJSONRequest(http.MethodPost, "/", `{"name":"kanata"}`)
	if err := RequireBody(req); err != nil {
		t.Fatalf("RequireBody(non-empty) error = %v", err)
	}

	emptyReq := newJSONRequest(http.MethodPost, "/", "")
	emptyReq.ContentLength = 0

	violation := assertSingleViolation(t, RequireBody(emptyReq))
	if violation.Field != "body" || violation.In != ViolationInBody || violation.Code != ViolationCodeRequired || violation.Detail != "is required" {
		t.Fatalf("violation = %#v", violation)
	}
}

func TestBindAndValidate_RequestValidatorReadsNormalizedDTO(t *testing.T) {
	var events []string
	dst := requestRuleNormalizedRequest{events: &events}
	req := newJSONRequest(http.MethodPost, "/", `{"name":"  kanata  "}`)

	if err := BindAndValidateBody(req, &dst); err != nil {
		t.Fatalf("BindAndValidateBody() error = %v", err)
	}
	if dst.Name != "kanata" {
		t.Fatalf("name = %q, want kanata", dst.Name)
	}
	if !reflect.DeepEqual(events, []string{"normalize", "request"}) {
		t.Fatalf("events = %#v, want normalize -> request", events)
	}
}

func TestBindAndValidate_RequestValidatorRunsBeforeFieldValidation(t *testing.T) {
	req := newJSONRequest(http.MethodPost, "/", `{}`)
	var dst requestRuleFailureRequest

	err := BindAndValidateBody(req, &dst)
	if !errors.Is(err, errRequestRuleFailed) {
		t.Fatalf("error = %v, want %v", err, errRequestRuleFailed)
	}
}

func TestBindAndValidate_RequestValidatorCanRequireBodyForMixedSourceRoute(t *testing.T) {
	req := requestWithPathParams(map[string][]string{
		"org_id": {"org_1"},
	})
	req.Method = http.MethodPost
	req.Header.Set("Content-Type", "application/json")
	req.Body = io.NopCloser(strings.NewReader(""))
	req.ContentLength = 0

	var dst requestRuleRequireBodyRequest
	err := BindAndValidate(req, &dst)
	violation := assertSingleViolation(t, err)
	if violation.Field != "body" || violation.In != ViolationInBody || violation.Code != ViolationCodeRequired || violation.Detail != "is required" {
		t.Fatalf("violation = %#v", violation)
	}
	if dst.OrgID != "org_1" {
		t.Fatalf("org_id = %q, want org_1", dst.OrgID)
	}
}
