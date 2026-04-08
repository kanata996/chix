package bind

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"net/url"
	"reflect"
	"testing"
)

// 测试清单：
// - 标记说明：[✓] 已核对且已有真实覆盖；[x] 尚未完成，不得作为验收依据。
// - [✓] `PathParam` / `PathParamOr` 覆盖成功、缺失、空值、名称跳过、默认值和类型转换失败分支。
// - [✓] `QueryParam` / `QueryParamOr` / `QueryParams` / `QueryParamsOr` 覆盖成功、缺失、空值、默认值和类型转换失败分支。
// - [✓] `FormValue` / `FormValueOr` / `FormValues` / `FormValuesOr` 覆盖成功、缺失、空值、默认值、nil request 和类型转换失败分支。
// - [✓] `ParseValueOr` / `ParseValues` / `ParseValuesOr` 的默认值和批量解析行为稳定。
// - [✓] `queryParams` / `formValues` / `bindingErrorDetails` 这些内部 helper 的稳定契约已单独覆盖。
func TestPathAndQueryGenericHelpers(t *testing.T) {
	t.Run("path param success", func(t *testing.T) {
		req := newRouteRequest(http.MethodGet, "/items", map[string]string{"id": "42"}, nil)

		got, err := PathParam[int](req, "id")
		if err != nil {
			t.Fatalf("PathParam() error = %v", err)
		}
		if got != 42 {
			t.Fatalf("got = %d, want 42", got)
		}
	})

	t.Run("path param missing returns ErrNonExistentKey", func(t *testing.T) {
		req := newRouteRequest(http.MethodGet, "/items", nil, nil)

		_, err := PathParam[int](req, "id")
		if !errors.Is(err, ErrNonExistentKey) {
			t.Fatalf("errors.Is(err, ErrNonExistentKey) = false, err = %v", err)
		}
	})

	t.Run("path param or missing returns default", func(t *testing.T) {
		req := newRouteRequest(http.MethodGet, "/items", map[string]string{"id": "42"}, nil)

		got, err := PathParamOr(req, "missing", 7)
		if err != nil {
			t.Fatalf("PathParamOr() error = %v", err)
		}
		if got != 7 {
			t.Fatalf("got = %d, want 7", got)
		}
	})

	t.Run("path param or invalid returns binding error", func(t *testing.T) {
		req := newRouteRequest(http.MethodGet, "/items", map[string]string{"id": "oops"}, nil)

		_, err := PathParamOr[int](req, "id", 7)
		field, values, message, ok := bindingErrorDetails(err)
		if !ok {
			t.Fatalf("bindingErrorDetails(%T) = false, want true", err)
		}
		if field != "id" || !reflect.DeepEqual(values, []string{"oops"}) || message != "path value" {
			t.Fatalf("got = (%q, %#v, %q)", field, values, message)
		}
	})

	t.Run("path param skips non matching names", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/orgs/acme/items/42", nil)
		req.Pattern = "/orgs/{org_id}/items/{id}"
		req.SetPathValue("org_id", "acme")
		req.SetPathValue("id", "42")

		got, err := PathParam[int](req, "id")
		if err != nil {
			t.Fatalf("PathParam() error = %v", err)
		}
		if got != 42 {
			t.Fatalf("got = %d, want 42", got)
		}
	})

	t.Run("path param invalid returns binding error", func(t *testing.T) {
		req := newRouteRequest(http.MethodGet, "/items", map[string]string{"id": "oops"}, nil)

		_, err := PathParam[int](req, "id")
		assertBindingError(t, err, "id", []string{"oops"}, "path value")
	})

	t.Run("path param or success returns parsed value", func(t *testing.T) {
		req := newRouteRequest(http.MethodGet, "/items", map[string]string{"id": "42"}, nil)

		got, err := PathParamOr(req, "id", 7)
		if err != nil {
			t.Fatalf("PathParamOr() error = %v", err)
		}
		if got != 42 {
			t.Fatalf("got = %d, want 42", got)
		}
	})

	t.Run("path param empty returns zero value", func(t *testing.T) {
		req := newRouteRequest(http.MethodGet, "/items", map[string]string{"id": ""}, nil)

		got, err := PathParam[int](req, "id")
		if err != nil {
			t.Fatalf("PathParam() error = %v", err)
		}
		if got != 0 {
			t.Fatalf("got = %d, want 0", got)
		}
	})

	t.Run("query param missing returns ErrNonExistentKey", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/search", nil)

		_, err := QueryParam[int](req, "page")
		if !errors.Is(err, ErrNonExistentKey) {
			t.Fatalf("errors.Is(err, ErrNonExistentKey) = false, err = %v", err)
		}
	})

	t.Run("query param empty returns zero value", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/search?page=", nil)

		got, err := QueryParam[int](req, "page")
		if err != nil {
			t.Fatalf("QueryParam() error = %v", err)
		}
		if got != 0 {
			t.Fatalf("got = %d, want 0", got)
		}
	})

	t.Run("query param or missing returns default", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/search", nil)

		got, err := QueryParamOr(req, "page", 9)
		if err != nil {
			t.Fatalf("QueryParamOr() error = %v", err)
		}
		if got != 9 {
			t.Fatalf("got = %d, want 9", got)
		}
	})

	t.Run("query param or empty returns default", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/search?page=", nil)

		got, err := QueryParamOr(req, "page", 9)
		if err != nil {
			t.Fatalf("QueryParamOr() error = %v", err)
		}
		if got != 9 {
			t.Fatalf("got = %d, want 9", got)
		}
	})

	t.Run("query param success", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/search?page=2", nil)

		got, err := QueryParam[int](req, "page")
		if err != nil {
			t.Fatalf("QueryParam() error = %v", err)
		}
		if got != 2 {
			t.Fatalf("got = %d, want 2", got)
		}
	})

	t.Run("query param invalid returns binding error", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/search?page=oops", nil)

		_, err := QueryParam[int](req, "page")
		assertBindingError(t, err, "page", []string{"oops"}, "query param")
	})

	t.Run("query param or invalid returns binding error", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/search?page=oops", nil)

		_, err := QueryParamOr[int](req, "page", 9)
		assertBindingError(t, err, "page", []string{"oops"}, "query param")
	})

	t.Run("query params success", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/search?page=1&page=2", nil)

		got, err := QueryParams[int](req, "page")
		if err != nil {
			t.Fatalf("QueryParams() error = %v", err)
		}
		if !reflect.DeepEqual(got, []int{1, 2}) {
			t.Fatalf("got = %#v, want %#v", got, []int{1, 2})
		}
	})

	t.Run("query params invalid returns binding error", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/search?page=1&page=oops", nil)

		_, err := QueryParams[int](req, "page")
		field, values, message, ok := bindingErrorDetails(err)
		if !ok {
			t.Fatalf("bindingErrorDetails(%T) = false, want true", err)
		}
		if field != "page" || !reflect.DeepEqual(values, []string{"1", "oops"}) || message != "query params" {
			t.Fatalf("got = (%q, %#v, %q)", field, values, message)
		}
	})

	t.Run("query params missing returns ErrNonExistentKey", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/search", nil)

		_, err := QueryParams[int](req, "page")
		if !errors.Is(err, ErrNonExistentKey) {
			t.Fatalf("errors.Is(err, ErrNonExistentKey) = false, err = %v", err)
		}
	})

	t.Run("query params or missing returns default", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/search", nil)

		got, err := QueryParamsOr(req, "page", []int{9, 10})
		if err != nil {
			t.Fatalf("QueryParamsOr() error = %v", err)
		}
		if !reflect.DeepEqual(got, []int{9, 10}) {
			t.Fatalf("got = %#v, want %#v", got, []int{9, 10})
		}
	})

	t.Run("query params or success returns parsed values", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/search?page=1&page=2", nil)

		got, err := QueryParamsOr(req, "page", []int{9})
		if err != nil {
			t.Fatalf("QueryParamsOr() error = %v", err)
		}
		if !reflect.DeepEqual(got, []int{1, 2}) {
			t.Fatalf("got = %#v, want %#v", got, []int{1, 2})
		}
	})

	t.Run("query params or invalid returns binding error", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/search?page=oops", nil)

		_, err := QueryParamsOr(req, "page", []int{9})
		field, values, message, ok := bindingErrorDetails(err)
		if !ok {
			t.Fatalf("bindingErrorDetails(%T) = false, want true", err)
		}
		if field != "page" || !reflect.DeepEqual(values, []string{"oops"}) || message != "query params" {
			t.Fatalf("got = (%q, %#v, %q)", field, values, message)
		}
	})
}

func TestFormGenericHelpers(t *testing.T) {
	newFormRequest := func(body string) *http.Request {
		req := httptest.NewRequest(http.MethodPost, "/", stringsNewReader(body))
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		return req
	}

	t.Run("form value success", func(t *testing.T) {
		req := newFormRequest("page=2")

		got, err := FormValue[int](req, "page")
		if err != nil {
			t.Fatalf("FormValue() error = %v", err)
		}
		if got != 2 {
			t.Fatalf("got = %d, want 2", got)
		}
	})

	t.Run("form value invalid returns binding error", func(t *testing.T) {
		req := newFormRequest("page=oops")

		_, err := FormValue[int](req, "page")
		field, values, message, ok := bindingErrorDetails(err)
		if !ok {
			t.Fatalf("bindingErrorDetails(%T) = false, want true", err)
		}
		if field != "page" || !reflect.DeepEqual(values, []string{"oops"}) || message != "form value" {
			t.Fatalf("got = (%q, %#v, %q)", field, values, message)
		}
	})

	t.Run("form value empty returns zero value", func(t *testing.T) {
		req := newFormRequest("")
		req.Form = url.Values{"page": {}}

		got, err := FormValue[int](req, "page")
		if err != nil {
			t.Fatalf("FormValue() error = %v", err)
		}
		if got != 0 {
			t.Fatalf("got = %d, want 0", got)
		}
	})

	t.Run("form value missing returns ErrNonExistentKey", func(t *testing.T) {
		req := newFormRequest("")

		_, err := FormValue[int](req, "page")
		if !errors.Is(err, ErrNonExistentKey) {
			t.Fatalf("errors.Is(err, ErrNonExistentKey) = false, err = %v", err)
		}
	})

	t.Run("form value or missing returns default", func(t *testing.T) {
		req := newFormRequest("")

		got, err := FormValueOr(req, "page", 9)
		if err != nil {
			t.Fatalf("FormValueOr() error = %v", err)
		}
		if got != 9 {
			t.Fatalf("got = %d, want 9", got)
		}
	})

	t.Run("form value or invalid returns binding error", func(t *testing.T) {
		req := newFormRequest("page=oops")

		_, err := FormValueOr(req, "page", 9)
		field, values, message, ok := bindingErrorDetails(err)
		if !ok {
			t.Fatalf("bindingErrorDetails(%T) = false, want true", err)
		}
		if field != "page" || !reflect.DeepEqual(values, []string{"oops"}) || message != "form value" {
			t.Fatalf("got = (%q, %#v, %q)", field, values, message)
		}
	})

	t.Run("form value or success returns parsed value", func(t *testing.T) {
		req := newFormRequest("page=2")

		got, err := FormValueOr(req, "page", 9)
		if err != nil {
			t.Fatalf("FormValueOr() error = %v", err)
		}
		if got != 2 {
			t.Fatalf("got = %d, want 2", got)
		}
	})

	t.Run("form values success", func(t *testing.T) {
		req := newFormRequest("page=2&page=3")

		got, err := FormValues[int](req, "page")
		if err != nil {
			t.Fatalf("FormValues() error = %v", err)
		}
		if !reflect.DeepEqual(got, []int{2, 3}) {
			t.Fatalf("got = %#v, want %#v", got, []int{2, 3})
		}
	})

	t.Run("form values invalid returns binding error", func(t *testing.T) {
		req := newFormRequest("page=2&page=oops")

		_, err := FormValues[int](req, "page")
		field, values, message, ok := bindingErrorDetails(err)
		if !ok {
			t.Fatalf("bindingErrorDetails(%T) = false, want true", err)
		}
		if field != "page" || !reflect.DeepEqual(values, []string{"2", "oops"}) || message != "form values" {
			t.Fatalf("got = (%q, %#v, %q)", field, values, message)
		}
	})

	t.Run("form values missing returns ErrNonExistentKey", func(t *testing.T) {
		req := newFormRequest("")

		_, err := FormValues[int](req, "page")
		if !errors.Is(err, ErrNonExistentKey) {
			t.Fatalf("errors.Is(err, ErrNonExistentKey) = false, err = %v", err)
		}
	})

	t.Run("form values or missing returns default", func(t *testing.T) {
		req := newFormRequest("")

		got, err := FormValuesOr(req, "page", []int{9, 10})
		if err != nil {
			t.Fatalf("FormValuesOr() error = %v", err)
		}
		if !reflect.DeepEqual(got, []int{9, 10}) {
			t.Fatalf("got = %#v, want %#v", got, []int{9, 10})
		}
	})

	t.Run("form values or invalid returns binding error", func(t *testing.T) {
		req := newFormRequest("page=oops")

		_, err := FormValuesOr(req, "page", []int{9})
		field, values, message, ok := bindingErrorDetails(err)
		if !ok {
			t.Fatalf("bindingErrorDetails(%T) = false, want true", err)
		}
		if field != "page" || !reflect.DeepEqual(values, []string{"oops"}) || message != "form values" {
			t.Fatalf("got = (%q, %#v, %q)", field, values, message)
		}
	})

	t.Run("form values or success returns parsed values", func(t *testing.T) {
		req := newFormRequest("page=2&page=3")

		got, err := FormValuesOr(req, "page", []int{9})
		if err != nil {
			t.Fatalf("FormValuesOr() error = %v", err)
		}
		if !reflect.DeepEqual(got, []int{2, 3}) {
			t.Fatalf("got = %#v, want %#v", got, []int{2, 3})
		}
	})
}

func TestFormGenericHelpersNilRequestErrors(t *testing.T) {
	if _, err := FormValue[int](nil, "page"); err == nil || err.Error() != "failed to parse form value, key: page, err: bind: request must not be nil" {
		t.Fatalf("FormValue(nil) error = %v", err)
	}
	if _, err := FormValueOr[int](nil, "page", 9); err == nil || err.Error() != "failed to parse form value, key: page, err: bind: request must not be nil" {
		t.Fatalf("FormValueOr(nil) error = %v", err)
	}
	if _, err := FormValues[int](nil, "page"); err == nil || err.Error() != "failed to parse form values, key: page, err: bind: request must not be nil" {
		t.Fatalf("FormValues(nil) error = %v", err)
	}
	if _, err := FormValuesOr[int](nil, "page", []int{9}); err == nil || err.Error() != "failed to parse form values, key: page, err: bind: request must not be nil" {
		t.Fatalf("FormValuesOr(nil) error = %v", err)
	}
}

func TestParseHelpers(t *testing.T) {
	got, err := ParseValueOr("", 7)
	if err != nil {
		t.Fatalf("ParseValueOr() error = %v", err)
	}
	if got != 7 {
		t.Fatalf("got = %d, want 7", got)
	}

	gotValues, err := ParseValues[int]([]string{"1", "2"})
	if err != nil {
		t.Fatalf("ParseValues() error = %v", err)
	}
	if !reflect.DeepEqual(gotValues, []int{1, 2}) {
		t.Fatalf("got = %#v, want %#v", gotValues, []int{1, 2})
	}

	gotValues, err = ParseValuesOr[int](nil, []int{9, 10})
	if err != nil {
		t.Fatalf("ParseValuesOr() error = %v", err)
	}
	if !reflect.DeepEqual(gotValues, []int{9, 10}) {
		t.Fatalf("got = %#v, want %#v", gotValues, []int{9, 10})
	}
}

func TestGenericInternalHelpers(t *testing.T) {
	if got := queryParams(nil); len(got) != 0 {
		t.Fatalf("queryParams(nil) = %#v, want empty", got)
	}

	if _, err := formValues(nil); err == nil || err.Error() != "bind: request must not be nil" {
		t.Fatalf("formValues(nil) error = %v", err)
	}

	err := NewBindingError("page", []string{"oops"}, "query param", errors.New("invalid syntax"))
	field, values, message, ok := bindingErrorDetails(err)
	if !ok {
		t.Fatalf("bindingErrorDetails(%T) = false, want true", err)
	}
	if field != "page" || !reflect.DeepEqual(values, []string{"oops"}) || message != "query param" {
		t.Fatalf("got = (%q, %#v, %q)", field, values, message)
	}

	if _, _, _, ok := bindingErrorDetails(errors.New("plain error")); ok {
		t.Fatal("bindingErrorDetails(plain error) = true, want false")
	}
	if _, _, _, ok := bindingErrorDetails(nil); ok {
		t.Fatal("bindingErrorDetails(nil) = true, want false")
	}
}
