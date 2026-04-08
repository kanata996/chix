package bind

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

// 测试清单：
// - 标记说明：[✓] 已核对且已有真实覆盖；[x] 尚未完成，不得作为验收依据。
// - [✓] `QueryParamsBinder` 在 `FailFast(false)` 下会继续绑定后续字段，并通过 `BindErrors()` 返回字段级 `BindingError`。
// - [✓] `PathValuesBinder` 在 path 值转换失败时返回带 field/values/detail 的 `BindingError`。
// - [✓] `FormFieldBinder` 会优先读取 form body 值，并在转换失败时返回 `BindingError`。
func TestQueryParamsBinderCollectsErrorsAndContinues(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/search?id=nope&nr=7", nil)

	binder := QueryParamsBinder(req).FailFast(false)

	var id, nr int64
	errs := binder.Int64("id", &id).Int64("nr", &nr).BindErrors()

	if id != 0 {
		t.Fatalf("id = %d, want 0", id)
	}
	if nr != 7 {
		t.Fatalf("nr = %d, want 7", nr)
	}
	if len(errs) != 1 {
		t.Fatalf("len(errs) = %d, want 1", len(errs))
	}
	assertBindingError(t, errs[0], "id", []string{"nope"}, "failed to bind field value to int64")
}

func TestPathValuesBinderReturnsBindingError(t *testing.T) {
	req := newRouteRequest(http.MethodGet, "/items", map[string]string{"id": "oops"}, nil)

	binder := PathValuesBinder(req)

	var id int64
	err := binder.Int64("id", &id).BindError()

	if id != 0 {
		t.Fatalf("id = %d, want 0", id)
	}
	assertBindingError(t, err, "id", []string{"oops"}, "failed to bind field value to int64")
}

func TestFormFieldBinderReadsFormBody(t *testing.T) {
	req := httptest.NewRequest(http.MethodPost, "/?page=1", stringsNewReader("page=2"))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	binder := FormFieldBinder(req)

	var page int64
	err := binder.Int64("page", &page).BindError()

	if err != nil {
		t.Fatalf("BindError() = %v, want nil", err)
	}
	if page != 2 {
		t.Fatalf("page = %d, want 2", page)
	}
}

func TestFormFieldBinderReturnsBindingError(t *testing.T) {
	req := httptest.NewRequest(http.MethodPost, "/", stringsNewReader("page=oops"))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	binder := FormFieldBinder(req)

	var page int64
	err := binder.Int64("page", &page).BindError()

	if page != 0 {
		t.Fatalf("page = %d, want 0", page)
	}
	assertBindingError(t, err, "page", []string{"oops"}, "failed to bind field value to int64")
}
