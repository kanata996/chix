package bind

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"reflect"
	"testing"

	echo "github.com/labstack/echo/v5"
)

// 测试清单：
// - 标记说明：[✓] 已核对且已有真实覆盖；[x] 尚未完成，不得作为验收依据。
// - [✓] `DefaultBinder.Bind` 在 `HEAD` 请求下会执行 path -> query -> body，并允许后阶段覆盖前阶段字段。
// - [✓] `Bind` / `BindPathValues` / `BindQueryParams` / `BindBody` / `BindHeaders` 对 nil request、nil target、非指针 target 做统一入参校验。
// - [✓] path/query/header 单源结构体绑定在类型转换失败时收敛为 `400 bad_request`。
// - [✓] `Bind` 在 path 或 query 阶段失败时会立即短路，同时保留前序已成功写入的字段，不执行后续阶段。
// - [✓] `BindBody` 在 serializer 返回普通错误时会统一映射为 `400 invalid_json`，并保留底层 cause 错误链。
type stubJSONSerializer struct {
	err error
}

func (s stubJSONSerializer) Serialize(c *echo.Context, target any, indent string) error {
	return nil
}

func (s stubJSONSerializer) Deserialize(c *echo.Context, target any) error {
	return s.err
}

func TestDefaultBinderBind_HEADIncludesQueryAndBodyOrder(t *testing.T) {
	type request struct {
		ID   string `param:"id" query:"id" json:"id"`
		Name string `query:"name" json:"name"`
	}

	req := newRouteRequest(
		http.MethodHead,
		"/items?id=query-id&name=query-name",
		map[string]string{"id": "route-id"},
		stringsNewReader(`{"id":"body-id"}`),
	)
	req.Header.Set("Content-Type", "application/json")

	var got request
	if err := (&DefaultBinder{}).Bind(req, &got); err != nil {
		t.Fatalf("DefaultBinder.Bind() error = %v", err)
	}

	want := request{
		ID:   "body-id",
		Name: "query-name",
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got = %#v, want %#v", got, want)
	}
}

func TestBindPublicFunctionsValidateInputs(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/", nil)

	tests := []struct {
		name string
		fn   func(*http.Request, any) error
	}{
		{name: "Bind", fn: Bind},
		{name: "BindPathValues", fn: BindPathValues},
		{name: "BindQueryParams", fn: BindQueryParams},
		{name: "BindBody", fn: BindBody},
		{name: "BindHeaders", fn: BindHeaders},
	}

	cases := []struct {
		name    string
		request *http.Request
		target  any
		want    string
	}{
		{
			name:    "nil request",
			request: nil,
			target:  &struct{}{},
			want:    "bind: request must not be nil",
		},
		{
			name:    "nil target",
			request: req,
			target:  nil,
			want:    "bind: target must not be nil",
		},
		{
			name:    "non pointer target",
			request: req,
			target:  struct{}{},
			want:    "bind: target must be a non-nil pointer",
		},
	}

	for _, tt := range tests {
		for _, tc := range cases {
			t.Run(tt.name+"/"+tc.name, func(t *testing.T) {
				err := tt.fn(tc.request, tc.target)
				if err == nil || err.Error() != tc.want {
					t.Fatalf("error = %v, want %q", err, tc.want)
				}
			})
		}
	}
}

func TestBindSingleSourceBindersReturnBadRequest(t *testing.T) {
	t.Run("path", func(t *testing.T) {
		type request struct {
			ID int `param:"id"`
		}

		req := newRouteRequest(http.MethodGet, "/items", map[string]string{"id": "oops"}, nil)
		err := BindPathValues(req, &request{})
		assertHTTPError(t, err, http.StatusBadRequest, "bad_request", "Bad Request")
	})

	t.Run("query", func(t *testing.T) {
		type request struct {
			Page int `query:"page"`
		}

		req := httptest.NewRequest(http.MethodGet, "/search?page=oops", nil)
		err := BindQueryParams(req, &request{})
		assertHTTPError(t, err, http.StatusBadRequest, "bad_request", "Bad Request")
	})

	t.Run("header", func(t *testing.T) {
		type request struct {
			Retry int `header:"X-Retry"`
		}

		req := httptest.NewRequest(http.MethodGet, "/", nil)
		req.Header.Set("X-Retry", "oops")
		err := BindHeaders(req, &request{})
		assertHTTPError(t, err, http.StatusBadRequest, "bad_request", "Bad Request")
	})
}

func TestBindStopsAfterQueryErrorAndKeepsEarlierWrites(t *testing.T) {
	type request struct {
		ID   string `param:"id"`
		Page int    `query:"page"`
		Name string `json:"name"`
	}

	req := newRouteRequest(
		http.MethodGet,
		"/items?page=oops",
		map[string]string{"id": "route-id"},
		stringsNewReader(`{"name":"body-name"}`),
	)
	req.Header.Set("Content-Type", "application/json")

	var got request
	err := Bind(req, &got)
	assertHTTPError(t, err, http.StatusBadRequest, "bad_request", "Bad Request")

	want := request{ID: "route-id"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got = %#v, want %#v", got, want)
	}
}

func TestBindStopsAfterPathErrorAndSkipsLaterStages(t *testing.T) {
	type request struct {
		ID   int    `param:"id"`
		Name string `query:"name" json:"name"`
	}

	req := newRouteRequest(
		http.MethodGet,
		"/items?name=query-name",
		map[string]string{"id": "oops"},
		stringsNewReader(`{"name":"body-name"}`),
	)
	req.Header.Set("Content-Type", "application/json")

	dst := request{Name: "existing"}
	err := Bind(req, &dst)
	assertHTTPError(t, err, http.StatusBadRequest, "bad_request", "Bad Request")

	if dst.Name != "existing" {
		t.Fatalf("dst.Name = %q, want existing", dst.Name)
	}
}

func TestBindBodySerializerPlainErrorMapsToInvalidJSON(t *testing.T) {
	cause := errors.New("serializer boom")
	req := httptest.NewRequest(http.MethodPost, "/", stringsNewReader(`{"name":"kanata"}`))
	req.Header.Set("Content-Type", "application/json")
	e := echo.New()
	e.JSONSerializer = stubJSONSerializer{err: cause}
	c := e.NewContext(req, httptest.NewRecorder())

	err := bindJSONBody(c, &struct{}{})
	assertHTTPError(t, err, http.StatusBadRequest, CodeInvalidJSON, "request body must be valid JSON")
	if !errors.Is(err, cause) {
		t.Fatalf("errors.Is(err, cause) = false, want true")
	}
}
