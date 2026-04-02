package reqx

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

type queryState string

func (s *queryState) UnmarshalText(text []byte) error {
	switch string(text) {
	case "active", "disabled":
		*s = queryState(text)
		return nil
	default:
		return errors.New("invalid state")
	}
}

func TestBindQueryParams_BindsSupportedTypes(t *testing.T) {
	type request struct {
		Name    string       `query:"name"`
		Enabled bool         `query:"enabled"`
		Age     *int         `query:"age"`
		Score   float64      `query:"score"`
		Tags    []string     `query:"tag"`
		State   queryState   `query:"state"`
		States  []queryState `query:"states"`
	}

	req := httptest.NewRequest(
		http.MethodGet,
		"/?name=kanata&enabled=true&age=17&score=9.5&tag=a&tag=b&state=active&states=active&states=disabled",
		nil,
	)

	var dst request
	if err := BindQueryParams(req, &dst); err != nil {
		t.Fatalf("BindQueryParams() error = %v", err)
	}
	if dst.Name != "kanata" {
		t.Fatalf("name = %q, want kanata", dst.Name)
	}
	if !dst.Enabled {
		t.Fatal("enabled = false, want true")
	}
	if dst.Age == nil || *dst.Age != 17 {
		t.Fatalf("age = %#v, want 17", dst.Age)
	}
	if dst.Score != 9.5 {
		t.Fatalf("score = %v, want 9.5", dst.Score)
	}
	if strings.Join(dst.Tags, ",") != "a,b" {
		t.Fatalf("tags = %#v, want [a b]", dst.Tags)
	}
	if dst.State != "active" {
		t.Fatalf("state = %q, want active", dst.State)
	}
	if len(dst.States) != 2 || dst.States[0] != "active" || dst.States[1] != "disabled" {
		t.Fatalf("states = %#v, want [active disabled]", dst.States)
	}
}

func TestBindQueryParams_IgnoresUnknownFieldsByDefault(t *testing.T) {
	type request struct {
		Page int `query:"page"`
	}

	req := httptest.NewRequest(http.MethodGet, "/?page=2&extra=1", nil)

	var dst request
	if err := BindQueryParams(req, &dst); err != nil {
		t.Fatalf("BindQueryParams() error = %v", err)
	}
	if dst.Page != 2 {
		t.Fatalf("page = %d, want 2", dst.Page)
	}
}

func TestBindQueryParams_RejectsRepeatedScalar(t *testing.T) {
	type request struct {
		ID string `query:"id"`
	}

	req := httptest.NewRequest(http.MethodGet, "/?id=1&id=2", nil)

	var dst request
	err := BindQueryParams(req, &dst)
	violation := assertSingleViolation(t, err)
	if violation.Field != "id" || violation.Code != ViolationCodeMultiple || violation.Message != "must not be repeated" {
		t.Fatalf("violation = %#v", violation)
	}
}

func TestBindQueryParams_RejectsTypeMismatch(t *testing.T) {
	type request struct {
		Page int `query:"page"`
	}

	req := httptest.NewRequest(http.MethodGet, "/?page=nope", nil)

	var dst request
	err := BindQueryParams(req, &dst)
	violation := assertSingleViolation(t, err)
	if violation.Field != "page" || violation.Code != ViolationCodeType || violation.Message != "must be number" {
		t.Fatalf("violation = %#v", violation)
	}
}

func TestBindQueryParams_RejectsInvalidTextUnmarshalerValue(t *testing.T) {
	type request struct {
		State queryState `query:"state"`
	}

	req := httptest.NewRequest(http.MethodGet, "/?state=unknown", nil)

	var dst request
	err := BindQueryParams(req, &dst)
	violation := assertSingleViolation(t, err)
	if violation.Field != "state" || violation.Code != ViolationCodeInvalid || violation.Message != "is invalid" {
		t.Fatalf("violation = %#v", violation)
	}
}

func TestBindQueryParams_RejectsUnsupportedFieldType(t *testing.T) {
	type nested struct {
		Name string
	}
	type request struct {
		Nested nested `query:"nested"`
	}

	req := httptest.NewRequest(http.MethodGet, "/?nested=x", nil)

	var dst request
	err := BindQueryParams(req, &dst)
	if err == nil {
		t.Fatal("BindQueryParams() error = nil")
	}
	if got := err.Error(); !strings.Contains(got, `unsupported query type`) {
		t.Fatalf("error = %q, want unsupported query type", got)
	}
}

func TestBindQueryParams_RejectsDuplicateTaggedFields(t *testing.T) {
	type request struct {
		Page  int `query:"page"`
		Limit int `query:"page"`
	}

	req := httptest.NewRequest(http.MethodGet, "/?page=1", nil)

	var dst request
	err := BindQueryParams(req, &dst)
	if err == nil {
		t.Fatal("BindQueryParams() error = nil")
	}
	if got := err.Error(); got != `reqx: duplicate query field "page" on Page and Limit` {
		t.Fatalf("error = %q", got)
	}
}

func TestBindQueryParams_RejectsUnexportedTaggedField(t *testing.T) {
	type request struct {
		name string `query:"name"`
	}

	req := httptest.NewRequest(http.MethodGet, "/?name=kanata", nil)

	var dst request
	_ = dst.name
	err := BindQueryParams(req, &dst)
	if err == nil {
		t.Fatal("BindQueryParams() error = nil")
	}
	if got := err.Error(); !strings.Contains(got, `must be exported`) {
		t.Fatalf("error = %q, want must be exported", got)
	}
}

func TestBindQueryParams_RequestMustNotBeNil(t *testing.T) {
	var dst struct{}
	err := BindQueryParams[struct{}](nil, &dst)
	if err == nil {
		t.Fatal("BindQueryParams() error = nil")
	}
	if got := err.Error(); got != "reqx: request must not be nil" {
		t.Fatalf("error = %q", got)
	}
}

func TestBindAndValidateQuery_UsesQueryTagName(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/", nil)

	var dst struct {
		Cursor string `query:"cursor" validate:"required"`
	}
	err := BindAndValidateQuery(req, &dst)
	violation := assertSingleViolation(t, err)
	if violation.Field != "cursor" || violation.Code != ViolationCodeRequired || violation.Message != "is required" {
		t.Fatalf("violation = %#v", violation)
	}
}

func TestSourceValues_PanicsOnUnsupportedSource(t *testing.T) {
	defer func() {
		if recover() == nil {
			t.Fatal("sourceValues() did not panic")
		}
	}()

	_ = sourceValues(httptest.NewRequest(http.MethodGet, "/", nil), valueSource{tag: "unsupported"})
}
