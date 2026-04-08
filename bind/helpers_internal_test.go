package bind

import (
	"encoding/json"
	"errors"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/kanata996/chix/errx"
)

type failingReadCloser struct {
	err error
}

func (r failingReadCloser) Read(_ []byte) (int, error) {
	return 0, r.err
}

func (r failingReadCloser) Close() error {
	return nil
}

type customParamValue struct {
	value string
	err   error
}

func (v *customParamValue) UnmarshalParam(param string) error {
	if v.err != nil {
		return v.err
	}
	v.value = param
	return nil
}

type customParamsValue struct {
	values []string
	err    error
}

func (v *customParamsValue) UnmarshalParams(params []string) error {
	if v.err != nil {
		return v.err
	}
	v.values = append([]string(nil), params...)
	return nil
}

type customTextValue string

func (v *customTextValue) UnmarshalText(text []byte) error {
	if string(text) == "bad" {
		return errors.New("bad text")
	}
	*v = customTextValue(text)
	return nil
}

func TestBindEntryPointsAndDefaultBinderBranches(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	var dst struct{}

	if err := Bind(nil, &dst); err == nil || err.Error() != "bind: request must not be nil" {
		t.Fatalf("Bind(nil) error = %v", err)
	}
	if err := Bind(req, nil); err == nil || err.Error() != "bind: destination must not be nil" {
		t.Fatalf("Bind(nil target) error = %v", err)
	}
	if err := BindBody(nil, &dst); err == nil || err.Error() != "bind: request must not be nil" {
		t.Fatalf("BindBody(nil) error = %v", err)
	}
	if err := BindBody(req, nil); err == nil || err.Error() != "bind: destination must not be nil" {
		t.Fatalf("BindBody(nil target) error = %v", err)
	}
	if err := BindQueryParams(nil, &dst); err == nil || err.Error() != "bind: request must not be nil" {
		t.Fatalf("BindQueryParams(nil) error = %v", err)
	}
	if err := BindQueryParams(req, nil); err == nil || err.Error() != "bind: destination must not be nil" {
		t.Fatalf("BindQueryParams(nil target) error = %v", err)
	}
	if err := BindPathValues(nil, &dst); err == nil || err.Error() != "bind: request must not be nil" {
		t.Fatalf("BindPathValues(nil) error = %v", err)
	}
	if err := BindPathValues(req, nil); err == nil || err.Error() != "bind: destination must not be nil" {
		t.Fatalf("BindPathValues(nil target) error = %v", err)
	}
	if err := BindHeaders(nil, &dst); err == nil || err.Error() != "bind: request must not be nil" {
		t.Fatalf("BindHeaders(nil) error = %v", err)
	}
	if err := BindHeaders(req, nil); err == nil || err.Error() != "bind: destination must not be nil" {
		t.Fatalf("BindHeaders(nil target) error = %v", err)
	}

	var binder DefaultBinder
	if err := binder.Bind(req, &dst); err != nil {
		t.Fatalf("DefaultBinder.Bind() error = %v", err)
	}

	if err := validateBindingDestination(1); err == nil || err.Error() != "bind: destination must not be nil" {
		t.Fatalf("validateBindingDestination(non-pointer) error = %v", err)
	}
	if err := validateBindingDestination(nil); err == nil || err.Error() != "bind: destination must not be nil" {
		t.Fatalf("validateBindingDestination(nil) error = %v", err)
	}
	if err := errorsf("boom %d", 1); err == nil || err.Error() != "bind: boom 1" {
		t.Fatalf("errorsf() = %v", err)
	}

	type request struct {
		ID int `param:"id"`
	}
	pathReq := requestWithPathParams(map[string][]string{"id": {"oops"}})
	if err := bindWithConfig(nil, &request{}, defaultBindConfig()); err == nil || err.Error() != "bind: request must not be nil" {
		t.Fatalf("bindWithConfig(nil) error = %v", err)
	}
	if err := bindWithConfig(req, request{}, defaultBindConfig()); err == nil || err.Error() != "bind: destination must not be nil" {
		t.Fatalf("bindWithConfig(non-pointer) error = %v", err)
	}
	if err := bindWithConfig(pathReq, &request{}, defaultBindConfig()); err == nil {
		t.Fatal("bindWithConfig(path error) = nil, want error")
	}
}

func TestBodyHelperBranches(t *testing.T) {
	if got := bodyMediaType(nil); got != "" {
		t.Fatalf("bodyMediaType(nil) = %q, want empty", got)
	}

	req := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(`{"name":"kanata"}`))
	req.Header.Set("Content-Type", " application/json ; charset=utf-8 ")
	if got := bodyMediaType(req); got != mimeApplicationJSON {
		t.Fatalf("bodyMediaType() = %q, want %q", got, mimeApplicationJSON)
	}

	type payload struct {
		Name string `json:"name"`
	}

	if err := decodeJSONBody([]byte(`{"name":"kanata"}`), &payload{}, false); err != nil {
		t.Fatalf("decodeJSONBody() error = %v", err)
	}

	err := decodeJSONBody([]byte(`{"extra":1}`), &payload{}, false)
	_ = assertHTTPError(t, err, http.StatusBadRequest, CodeInvalidJSON, "request body must be valid JSON")

	invalidUnmarshalErr := &json.InvalidUnmarshalError{Type: reflect.TypeOf(payload{})}
	if got := mapJSONBodyDecodeError(invalidUnmarshalErr); got != invalidUnmarshalErr {
		t.Fatalf("mapJSONBodyDecodeError() = %v, want same error", got)
	}

	if got := badRequestWrap(nil); got != nil {
		t.Fatalf("badRequestWrap(nil) = %v, want nil", got)
	}

	httpErr := errx.BadRequest("bad_request", "bad request")
	if got := badRequestWrap(httpErr); got != httpErr {
		t.Fatalf("badRequestWrap(http error) = %v, want same error", got)
	}

	wrapped := badRequestWrap(errors.New("boom"))
	_ = assertHTTPError(t, wrapped, http.StatusBadRequest, "bad_request", "Bad Request")

	data, err := readBody(io.NopCloser(strings.NewReader("ok")), 0)
	if err != nil || string(data) != "ok" {
		t.Fatalf("readBody(default max) = (%q, %v), want (ok, nil)", data, err)
	}
	if data, err := readBody(nil, 10); err != nil || data != nil {
		t.Fatalf("readBody(nil) = (%v, %v), want (nil, nil)", data, err)
	}

	wantErr := errors.New("read failed")
	if _, err := readBody(failingReadCloser{err: wantErr}, 10); !errors.Is(err, wantErr) {
		t.Fatalf("readBody(failing) error = %v, want %v", err, wantErr)
	}

	if err := bindBodyDefault(nil, &payload{}, defaultBindConfig().body); err == nil || err.Error() != "bind: request must not be nil" {
		t.Fatalf("bindBodyDefault(nil) error = %v", err)
	}
	if err := bindBodyDefault(req, payload{}, defaultBindConfig().body); err == nil || err.Error() != "bind: destination must not be nil" {
		t.Fatalf("bindBodyDefault(non-pointer) error = %v", err)
	}

	readErrReq := httptest.NewRequest(http.MethodPost, "/", nil)
	readErrReq.ContentLength = 1
	readErrReq.Header.Set("Content-Type", mimeApplicationJSON)
	readErrReq.Body = failingReadCloser{err: wantErr}
	if err := bindBodyDefault(readErrReq, &payload{}, defaultBindConfig().body); !errors.Is(err, wantErr) {
		t.Fatalf("bindBodyDefault(read error) = %v, want %v", err, wantErr)
	}
}

func TestBindDataDefaultBranches(t *testing.T) {
	if err := bindDataDefault(nil, nil, "query", nil); err != nil {
		t.Fatalf("bindDataDefault(nil empty) error = %v", err)
	}
	if err := bindDataDefault(1, map[string][]string{"x": {"1"}}, "query", nil); err == nil || err.Error() != "binding element must be a pointer" {
		t.Fatalf("bindDataDefault(non-pointer) error = %v", err)
	}

	t.Run("map targets", func(t *testing.T) {
		stringMap := map[string]string(nil)
		if err := bindDataDefault(&stringMap, map[string][]string{"name": {"kanata"}, "skip": {}}, "query", nil); err != nil {
			t.Fatalf("bindDataDefault(map[string]string) error = %v", err)
		}
		if got := stringMap["name"]; got != "kanata" {
			t.Fatalf("stringMap[name] = %q, want kanata", got)
		}
		if _, ok := stringMap["skip"]; ok {
			t.Fatalf("stringMap[skip] unexpectedly set")
		}

		sliceMap := map[string][]string(nil)
		if err := bindDataDefault(&sliceMap, map[string][]string{"tag": {"a", "b"}}, "query", nil); err != nil {
			t.Fatalf("bindDataDefault(map[string][]string) error = %v", err)
		}
		if got := strings.Join(sliceMap["tag"], ","); got != "a,b" {
			t.Fatalf("sliceMap[tag] = %q, want a,b", got)
		}

		anyMap := map[string]any(nil)
		if err := bindDataDefault(&anyMap, map[string][]string{"name": {"kanata"}}, "query", nil); err != nil {
			t.Fatalf("bindDataDefault(map[string]any) error = %v", err)
		}
		if got := anyMap["name"]; got != "kanata" {
			t.Fatalf("anyMap[name] = %#v, want kanata", got)
		}

		intMap := map[string]int(nil)
		if err := bindDataDefault(&intMap, map[string][]string{"n": {"1"}}, "query", nil); err != nil {
			t.Fatalf("bindDataDefault(map[string]int) error = %v", err)
		}
		if intMap != nil {
			t.Fatalf("intMap = %#v, want nil no-op", intMap)
		}
	})

	t.Run("scalar destination rules", func(t *testing.T) {
		value := 1
		if err := bindDataDefault(&value, map[string][]string{"n": {"1"}}, "json", nil); err == nil || err.Error() != "binding element must be a struct" {
			t.Fatalf("bindDataDefault(json scalar) error = %v", err)
		}
		if err := bindDataDefault(&value, map[string][]string{"n": {"1"}}, "query", nil); err != nil {
			t.Fatalf("bindDataDefault(query scalar) error = %v", err)
		}
	})

	t.Run("struct binding", func(t *testing.T) {
		type nested struct {
			Name string `query:"name"`
		}
		type request struct {
			Nested nested
			Age    *int              `query:"age"`
			IDs    []int             `query:"id"`
			When   time.Time         `query:"when" format:"2006-01-02"`
			Custom customParamValue  `query:"custom"`
			Multi  customParamsValue `query:"multi"`
			State  customTextValue   `query:"state"`
			Trace  string            `header:"x-trace-id"`
		}

		var dst request
		err := bindDataDefault(&dst, map[string][]string{
			"name":       {"kanata"},
			"age":        {"17"},
			"id":         {"1", "2"},
			"when":       {"2026-04-09"},
			"custom":     {"x"},
			"multi":      {"a", "b"},
			"state":      {"open"},
			"X-Trace-Id": {"req-1"},
		}, "query", nil)
		if err != nil {
			t.Fatalf("bindDataDefault(struct) error = %v", err)
		}
		if dst.Nested.Name != "kanata" {
			t.Fatalf("Nested.Name = %q, want kanata", dst.Nested.Name)
		}
		if dst.Age == nil || *dst.Age != 17 {
			t.Fatalf("Age = %#v, want 17", dst.Age)
		}
		if !reflect.DeepEqual(dst.IDs, []int{1, 2}) {
			t.Fatalf("IDs = %#v, want [1 2]", dst.IDs)
		}
		if got := dst.When.Format("2006-01-02"); got != "2026-04-09" {
			t.Fatalf("When = %q, want 2026-04-09", got)
		}
		if dst.Custom.value != "x" {
			t.Fatalf("Custom = %#v, want x", dst.Custom)
		}
		if !reflect.DeepEqual(dst.Multi.values, []string{"a", "b"}) {
			t.Fatalf("Multi = %#v, want [a b]", dst.Multi)
		}
		if dst.State != "open" {
			t.Fatalf("State = %q, want open", dst.State)
		}

		headerDst := struct {
			Trace string `header:"x-trace-id"`
		}{}
		if err := bindDataDefault(&headerDst, map[string][]string{"X-Trace-Id": {"req-1"}}, "header", nil); err != nil {
			t.Fatalf("bindDataDefault(case-insensitive header) error = %v", err)
		}
		if headerDst.Trace != "req-1" {
			t.Fatalf("Trace = %q, want req-1", headerDst.Trace)
		}
	})

	t.Run("anonymous tagged struct is rejected", func(t *testing.T) {
		type Embedded struct {
			Name string
		}
		type request struct {
			Embedded `query:"name"`
		}

		var dst request
		err := bindDataDefault(&dst, map[string][]string{"name": {"kanata"}}, "query", nil)
		if err == nil || err.Error() != "query/param/form tags are not allowed with anonymous struct field" {
			t.Fatalf("bindDataDefault(anonymous tagged) error = %v", err)
		}
	})

	t.Run("anonymous pointer nil and unexported field are skipped", func(t *testing.T) {
		type embedded struct {
			Name string `query:"name"`
		}
		type request struct {
			*embedded
			name string `query:"name"`
		}

		dst := request{}
		if err := bindDataDefault(&dst, map[string][]string{"name": {"kanata"}}, "query", nil); err != nil {
			t.Fatalf("bindDataDefault(skip nil embedded/unexported) error = %v", err)
		}
		if dst.embedded != nil {
			t.Fatalf("embedded = %#v, want nil", dst.embedded)
		}
		if dst.name != "" {
			t.Fatalf("name = %q, want empty", dst.name)
		}
	})

	t.Run("anonymous pointer non nil is traversed", func(t *testing.T) {
		type Embedded struct {
			Name string `query:"name"`
		}
		type request struct {
			*Embedded
		}

		dst := request{Embedded: &Embedded{}}
		if err := bindDataDefault(&dst, map[string][]string{"name": {"kanata"}}, "query", nil); err != nil {
			t.Fatalf("bindDataDefault(non-nil embedded pointer) error = %v", err)
		}
		if dst.Embedded == nil || dst.Name != "kanata" {
			t.Fatalf("embedded = %#v, want name=kanata", dst.Embedded)
		}
	})

	t.Run("recursive and decoder errors propagate", func(t *testing.T) {
		type nested struct {
			Age int `query:"age"`
		}
		type request struct {
			Nested nested
		}

		var recursive request
		err := bindDataDefault(&recursive, map[string][]string{"age": {"oops"}}, "query", nil)
		if err == nil {
			t.Fatal("bindDataDefault(recursive error) = nil")
		}

		type withMulti struct {
			Multi customParamsValue `query:"multi"`
		}
		var multi withMulti
		multi.Multi.err = errors.New("multi failed")
		err = bindDataDefault(&multi, map[string][]string{"multi": {"x"}}, "query", nil)
		if err == nil || err.Error() != "multi failed" {
			t.Fatalf("bindDataDefault(multi error) = %v", err)
		}

		type withCustom struct {
			Custom customParamValue `query:"custom"`
		}
		var custom withCustom
		custom.Custom.err = errors.New("custom failed")
		err = bindDataDefault(&custom, map[string][]string{"custom": {"x"}}, "query", nil)
		if err == nil || err.Error() != "custom failed" {
			t.Fatalf("bindDataDefault(custom error) = %v", err)
		}

		type withTime struct {
			When time.Time `query:"when" format:"2006-01-02"`
		}
		var timed withTime
		err = bindDataDefault(&timed, map[string][]string{"when": {"bad"}}, "query", nil)
		if err == nil {
			t.Fatal("bindDataDefault(time parse error) = nil")
		}

		type withIDs struct {
			IDs []int `query:"id"`
		}
		var ids withIDs
		err = bindDataDefault(&ids, map[string][]string{"id": {"1", "oops"}}, "query", nil)
		if err == nil {
			t.Fatal("bindDataDefault(slice parse error) = nil")
		}
	})
}

func TestUnmarshalHelpersAndSetters(t *testing.T) {
	var multi customParamsValue
	ok, err := unmarshalInputsToFieldDefault(reflect.Slice, []string{"a", "b"}, reflect.ValueOf(&multi).Elem())
	if !ok || err != nil || !reflect.DeepEqual(multi.values, []string{"a", "b"}) {
		t.Fatalf("unmarshalInputsToFieldDefault(slice) = (%v, %v), values=%#v", ok, err, multi.values)
	}

	var multiPtr *customParamsValue
	ok, err = unmarshalInputsToFieldDefault(reflect.Pointer, []string{"x"}, reflect.ValueOf(&multiPtr).Elem())
	if !ok || err != nil || multiPtr == nil || !reflect.DeepEqual(multiPtr.values, []string{"x"}) {
		t.Fatalf("unmarshalInputsToFieldDefault(pointer) = (%v, %v), value=%#v", ok, err, multiPtr)
	}

	var plain string
	ok, err = unmarshalInputsToFieldDefault(reflect.String, []string{"x"}, reflect.ValueOf(&plain).Elem())
	if ok || err != nil {
		t.Fatalf("unmarshalInputsToFieldDefault(string) = (%v, %v), want false nil", ok, err)
	}

	var when time.Time
	ok, err = unmarshalInputToFieldDefault(reflect.Struct, "2026-04-09", reflect.ValueOf(&when).Elem(), "2006-01-02")
	if !ok || err != nil || when.Format("2006-01-02") != "2026-04-09" {
		t.Fatalf("unmarshalInputToFieldDefault(time) = (%v, %v), when=%v", ok, err, when)
	}
	ok, err = unmarshalInputToFieldDefault(reflect.Struct, "bad", reflect.ValueOf(&when).Elem(), "2006-01-02")
	if !ok || err == nil {
		t.Fatalf("unmarshalInputToFieldDefault(invalid time) = (%v, %v), want true error", ok, err)
	}

	var custom *customParamValue
	ok, err = unmarshalInputToFieldDefault(reflect.Pointer, "value", reflect.ValueOf(&custom).Elem(), "")
	if !ok || err != nil || custom == nil || custom.value != "value" {
		t.Fatalf("unmarshalInputToFieldDefault(BindUnmarshaler) = (%v, %v), value=%#v", ok, err, custom)
	}

	var text customTextValue
	ok, err = unmarshalInputToFieldDefault(reflect.String, "open", reflect.ValueOf(&text).Elem(), "")
	if !ok || err != nil || text != "open" {
		t.Fatalf("unmarshalInputToFieldDefault(TextUnmarshaler) = (%v, %v), value=%q", ok, err, text)
	}

	ok, err = unmarshalInputToFieldDefault(reflect.Int, "1", reflect.ValueOf(new(int)).Elem(), "")
	if ok || err != nil {
		t.Fatalf("unmarshalInputToFieldDefault(int) = (%v, %v), want false nil", ok, err)
	}

	t.Run("scalar kinds", func(t *testing.T) {
		var i int
		if err := setWithProperTypeDefault(reflect.Int, "1", reflect.ValueOf(&i).Elem()); err != nil || i != 1 {
			t.Fatalf("setWithProperTypeDefault(int) error = %v, value=%d", err, i)
		}
		var i8 int8
		_ = setWithProperTypeDefault(reflect.Int8, "1", reflect.ValueOf(&i8).Elem())
		var i16 int16
		_ = setWithProperTypeDefault(reflect.Int16, "1", reflect.ValueOf(&i16).Elem())
		var i32 int32
		_ = setWithProperTypeDefault(reflect.Int32, "1", reflect.ValueOf(&i32).Elem())
		var i64 int64
		_ = setWithProperTypeDefault(reflect.Int64, "1", reflect.ValueOf(&i64).Elem())
		var u uint
		if err := setWithProperTypeDefault(reflect.Uint, "", reflect.ValueOf(&u).Elem()); err != nil || u != 0 {
			t.Fatalf("setWithProperTypeDefault(uint) error = %v, value=%d", err, u)
		}
		var u8 uint8
		_ = setWithProperTypeDefault(reflect.Uint8, "1", reflect.ValueOf(&u8).Elem())
		var u16 uint16
		_ = setWithProperTypeDefault(reflect.Uint16, "1", reflect.ValueOf(&u16).Elem())
		var u32 uint32
		_ = setWithProperTypeDefault(reflect.Uint32, "1", reflect.ValueOf(&u32).Elem())
		var u64 uint64
		_ = setWithProperTypeDefault(reflect.Uint64, "1", reflect.ValueOf(&u64).Elem())
		var b bool
		if err := setWithProperTypeDefault(reflect.Bool, "", reflect.ValueOf(&b).Elem()); err != nil || b {
			t.Fatalf("setWithProperTypeDefault(bool empty) error = %v, value=%v", err, b)
		}
		var f32 float32
		if err := setWithProperTypeDefault(reflect.Float32, "", reflect.ValueOf(&f32).Elem()); err != nil || f32 != 0 {
			t.Fatalf("setWithProperTypeDefault(float32 empty) error = %v, value=%v", err, f32)
		}
		var f64 float64
		if err := setWithProperTypeDefault(reflect.Float64, "1.5", reflect.ValueOf(&f64).Elem()); err != nil || f64 != 1.5 {
			t.Fatalf("setWithProperTypeDefault(float64) error = %v, value=%v", err, f64)
		}
		var s string
		if err := setWithProperTypeDefault(reflect.String, "x", reflect.ValueOf(&s).Elem()); err != nil || s != "x" {
			t.Fatalf("setWithProperTypeDefault(string) error = %v, value=%q", err, s)
		}
		var ptr *int
		if err := setWithProperTypeDefault(reflect.Pointer, "2", reflect.ValueOf(&ptr).Elem()); err != nil || ptr == nil || *ptr != 2 {
			t.Fatalf("setWithProperTypeDefault(pointer) error = %v, value=%#v", err, ptr)
		}
	})

	var unsupported struct{}
	if err := setWithProperTypeDefault(reflect.Struct, "x", reflect.ValueOf(&unsupported).Elem()); err == nil || err.Error() != "unknown type" {
		t.Fatalf("setWithProperTypeDefault(struct) error = %v", err)
	}

	var customValue customParamValue
	if err := setWithProperTypeDefault(reflect.Struct, "value", reflect.ValueOf(&customValue).Elem()); err != nil || customValue.value != "value" {
		t.Fatalf("setWithProperTypeDefault(BindUnmarshaler) error = %v, value=%#v", err, customValue)
	}
}

func TestMultipartAndPathHelpers(t *testing.T) {
	if ok, err := isFieldMultipartFile(multipartFileHeaderPointerType); !ok || err != nil {
		t.Fatalf("isFieldMultipartFile(pointer) = (%v, %v), want (true, nil)", ok, err)
	}
	if ok, err := isFieldMultipartFile(multipartFileHeaderSliceType); !ok || err != nil {
		t.Fatalf("isFieldMultipartFile(slice) = (%v, %v), want (true, nil)", ok, err)
	}
	if ok, err := isFieldMultipartFile(multipartFileHeaderPointerSliceType); !ok || err != nil {
		t.Fatalf("isFieldMultipartFile(pointer slice) = (%v, %v), want (true, nil)", ok, err)
	}
	if ok, err := isFieldMultipartFile(multipartFileHeaderType); !ok || err == nil {
		t.Fatalf("isFieldMultipartFile(struct) = (%v, %v), want (true, error)", ok, err)
	}
	if ok, err := isFieldMultipartFile(reflect.TypeOf("")); ok || err != nil {
		t.Fatalf("isFieldMultipartFile(string) = (%v, %v), want (false, nil)", ok, err)
	}

	file := &multipart.FileHeader{Filename: "a.txt"}
	files := map[string][]*multipart.FileHeader{"file": {file}}

	var single *multipart.FileHeader
	if ok := setMultipartFileHeaderTypes(reflect.ValueOf(&single).Elem(), "file", files); !ok || single == nil || single.Filename != "a.txt" {
		t.Fatalf("setMultipartFileHeaderTypes(single) = (%v, %#v)", ok, single)
	}

	var slice []multipart.FileHeader
	if ok := setMultipartFileHeaderTypes(reflect.ValueOf(&slice).Elem(), "file", files); !ok || len(slice) != 1 || slice[0].Filename != "a.txt" {
		t.Fatalf("setMultipartFileHeaderTypes(slice) = (%v, %#v)", ok, slice)
	}

	var ptrSlice []*multipart.FileHeader
	if ok := setMultipartFileHeaderTypes(reflect.ValueOf(&ptrSlice).Elem(), "file", files); !ok || len(ptrSlice) != 1 || ptrSlice[0].Filename != "a.txt" {
		t.Fatalf("setMultipartFileHeaderTypes(ptrSlice) = (%v, %#v)", ok, ptrSlice)
	}

	var wrong string
	if ok := setMultipartFileHeaderTypes(reflect.ValueOf(&wrong).Elem(), "file", files); ok {
		t.Fatal("setMultipartFileHeaderTypes(string) = true, want false")
	}
	if ok := setMultipartFileHeaderTypes(reflect.ValueOf(&single).Elem(), "missing", files); ok {
		t.Fatal("setMultipartFileHeaderTypes(missing) = true, want false")
	}

	type upload struct {
		File *multipart.FileHeader `query:"file"`
	}
	var up upload
	if err := bindDataDefault(&up, nil, "query", files); err != nil {
		t.Fatalf("bindDataDefault(file pointer) error = %v", err)
	}
	if up.File == nil || up.File.Filename != "a.txt" {
		t.Fatalf("bindDataDefault(file pointer) = %#v", up.File)
	}

	type badUpload struct {
		File multipart.FileHeader `query:"file"`
	}
	var bad badUpload
	if err := bindDataDefault(&bad, nil, "query", files); err == nil || !strings.Contains(err.Error(), "binding to multipart.FileHeader struct is not supported") {
		t.Fatalf("bindDataDefault(file struct) error = %v", err)
	}

	if got := pathWildcardNames("   "); got != nil {
		t.Fatalf("pathWildcardNames(blank) = %#v, want nil", got)
	}
	if got := pathWildcardNames("GET /users/{user_id}/files/{path...}/{$}/{id:rest}/{ }"); !reflect.DeepEqual(got, []string{"user_id", "path", "id"}) {
		t.Fatalf("pathWildcardNames() = %#v", got)
	}
	if got := pathWildcardNames("/users/{user_id"); len(got) != 0 {
		t.Fatalf("pathWildcardNames(invalid pattern) = %#v, want empty", got)
	}

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header = http.Header{
		" ":      {"ignored"},
		"x-name": {"kanata"},
	}
	var headerDst struct {
		Name string `header:"x-name"`
	}
	if err := bindHeadersDefault(req, &headerDst); err != nil {
		t.Fatalf("bindHeadersDefault(blank key) error = %v", err)
	}
	if headerDst.Name != "kanata" {
		t.Fatalf("headerDst.Name = %q, want kanata", headerDst.Name)
	}
}
