package paramx

import (
	"context"
	"github.com/kanata996/chix/reqx"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-chi/chi/v5"
)

const (
	upperUUID1     = "A2B5AA07-F5E5-4C6D-921D-00F13ED4E580"
	canonicalUUID1 = "a2b5aa07-f5e5-4c6d-921d-00f13ed4e580"
	upperUUID2     = "BBFE2586-65EC-4CF1-B4D5-85B5C6AB3100"
	canonicalUUID2 = "bbfe2586-65ec-4cf1-b4d5-85b5c6ab3100"
)

func TestPathReader(t *testing.T) {
	t.Run("string trims value", func(t *testing.T) {
		req := withRouteParam(httptest.NewRequest(http.MethodGet, "/", nil), "uuid", "  value  ")
		reader := Path(req)

		value, err := reader.String("uuid")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if value != "value" {
			t.Fatalf("unexpected value: %q", value)
		}
	})

	t.Run("string missing is required problem", func(t *testing.T) {
		reader := Path(httptest.NewRequest(http.MethodGet, "/", nil))
		_, err := reader.String("uuid")
		assertProblemDetail(t, err, reqx.InPath, "uuid", reqx.DetailCodeRequired)
	})

	t.Run("uuid canonicalizes value", func(t *testing.T) {
		req := withRouteParam(httptest.NewRequest(http.MethodGet, "/", nil), "uuid", " "+upperUUID1+" ")
		reader := Path(req)
		value, err := reader.UUID("uuid")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if value != canonicalUUID1 {
			t.Fatalf("unexpected value: %q", value)
		}
	})

	t.Run("uuid invalid format", func(t *testing.T) {
		req := withRouteParam(httptest.NewRequest(http.MethodGet, "/", nil), "uuid", "bad")
		reader := Path(req)
		_, err := reader.UUID("uuid")
		assertProblemDetail(t, err, reqx.InPath, "uuid", reqx.DetailCodeInvalidUUID)
	})

	t.Run("int invalid format", func(t *testing.T) {
		req := withRouteParam(httptest.NewRequest(http.MethodGet, "/", nil), "id", "bad")
		reader := Path(req)
		_, err := reader.Int("id")
		assertProblemDetail(t, err, reqx.InPath, "id", reqx.DetailCodeInvalidInteger)
	})

	t.Run("uuid missing is required problem", func(t *testing.T) {
		_, err := Path(httptest.NewRequest(http.MethodGet, "/", nil)).UUID("uuid")
		assertProblemDetail(t, err, reqx.InPath, "uuid", reqx.DetailCodeRequired)
	})

	t.Run("int parses value", func(t *testing.T) {
		req := withRouteParam(httptest.NewRequest(http.MethodGet, "/", nil), "id", " 42 ")
		value, err := Path(req).Int("id")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if value != 42 {
			t.Fatalf("unexpected value: %d", value)
		}
	})

	t.Run("int missing is required problem", func(t *testing.T) {
		_, err := Path(httptest.NewRequest(http.MethodGet, "/", nil)).Int("id")
		assertProblemDetail(t, err, reqx.InPath, "id", reqx.DetailCodeRequired)
	})

	t.Run("nil request is programming error", func(t *testing.T) {
		_, err := Path(nil).String("uuid")
		assertPlainErrorMessage(t, err, "paramx: request must not be nil")
	})

	t.Run("blank name is programming error", func(t *testing.T) {
		_, err := Path(httptest.NewRequest(http.MethodGet, "/", nil)).String("   ")
		assertPlainErrorMessage(t, err, "paramx: name must not be blank")
	})
}

func TestQueryReader(t *testing.T) {
	t.Run("string missing", func(t *testing.T) {
		value, ok, err := Query(httptest.NewRequest(http.MethodGet, "/", nil)).String("keyword")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if ok || value != "" {
			t.Fatalf("unexpected result: value=%q ok=%v", value, ok)
		}
	})

	t.Run("string keeps present empty", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/?keyword=%20%20%20", nil)
		value, ok, err := Query(req).String("keyword")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !ok || value != "" {
			t.Fatalf("unexpected result: value=%q ok=%v", value, ok)
		}
	})

	t.Run("string multiple values", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/?keyword=a&keyword=b", nil)
		_, _, err := Query(req).String("keyword")
		assertProblemDetail(t, err, reqx.InQuery, "keyword", reqx.DetailCodeMultipleValues)
	})

	t.Run("required string rejects empty", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/?keyword=%20%20%20", nil)
		_, err := Query(req).RequiredString("keyword")
		assertProblemDetail(t, err, reqx.InQuery, "keyword", reqx.DetailCodeRequired)
	})

	t.Run("strings support repeated keys", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/?tag=a&tag=%20b%20&tag=c", nil)
		values, ok, err := Query(req).Strings("tag")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !ok {
			t.Fatal("expected values")
		}
		if len(values) != 3 || values[0] != "a" || values[1] != "b" || values[2] != "c" {
			t.Fatalf("unexpected values: %#v", values)
		}
	})

	t.Run("strings reject blank item", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/?tag=a&tag=%20%20%20", nil)
		_, _, err := Query(req).Strings("tag")
		assertProblemDetail(t, err, reqx.InQuery, "tag", reqx.DetailCodeInvalidValue)
	})

	t.Run("strings missing", func(t *testing.T) {
		values, ok, err := Query(httptest.NewRequest(http.MethodGet, "/", nil)).Strings("tag")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if ok || values != nil {
			t.Fatalf("unexpected result: values=%#v ok=%v", values, ok)
		}
	})

	t.Run("required strings missing", func(t *testing.T) {
		_, err := Query(httptest.NewRequest(http.MethodGet, "/", nil)).RequiredStrings("tag")
		assertProblemDetail(t, err, reqx.InQuery, "tag", reqx.DetailCodeRequired)
	})

	t.Run("required string trims value", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/?keyword=%20hello%20", nil)
		value, err := Query(req).RequiredString("keyword")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if value != "hello" {
			t.Fatalf("unexpected value: %q", value)
		}
	})

	t.Run("required string missing", func(t *testing.T) {
		_, err := Query(httptest.NewRequest(http.MethodGet, "/", nil)).RequiredString("keyword")
		assertProblemDetail(t, err, reqx.InQuery, "keyword", reqx.DetailCodeRequired)
	})

	t.Run("required string multiple values", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/?keyword=a&keyword=b", nil)
		_, err := Query(req).RequiredString("keyword")
		assertProblemDetail(t, err, reqx.InQuery, "keyword", reqx.DetailCodeMultipleValues)
	})

	t.Run("required strings trim values", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/?tag=%20a%20&tag=b", nil)
		values, err := Query(req).RequiredStrings("tag")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		assertStrings(t, values, []string{"a", "b"})
	})

	t.Run("required strings reject blank item", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/?tag=a&tag=%20%20", nil)
		_, err := Query(req).RequiredStrings("tag")
		assertProblemDetail(t, err, reqx.InQuery, "tag", reqx.DetailCodeInvalidValue)
	})

	t.Run("int missing", func(t *testing.T) {
		value, ok, err := Query(httptest.NewRequest(http.MethodGet, "/", nil)).Int("limit")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if ok || value != 0 {
			t.Fatalf("unexpected result: value=%d ok=%v", value, ok)
		}
	})

	t.Run("int empty is invalid", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/?limit=%20%20%20", nil)
		_, _, err := Query(req).Int("limit")
		assertProblemDetail(t, err, reqx.InQuery, "limit", reqx.DetailCodeInvalidInteger)
	})

	t.Run("int parses value", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/?limit=%2042%20", nil)
		value, ok, err := Query(req).Int("limit")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !ok || value != 42 {
			t.Fatalf("unexpected result: value=%d ok=%v", value, ok)
		}
	})

	t.Run("int rejects non numeric", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/?limit=bad", nil)
		_, _, err := Query(req).Int("limit")
		assertProblemDetail(t, err, reqx.InQuery, "limit", reqx.DetailCodeInvalidInteger)
	})

	t.Run("int multiple values", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/?limit=1&limit=2", nil)
		_, _, err := Query(req).Int("limit")
		assertProblemDetail(t, err, reqx.InQuery, "limit", reqx.DetailCodeMultipleValues)
	})

	t.Run("int16 parses value", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/?status=%202%20", nil)
		value, ok, err := Query(req).Int16("status")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !ok || value != 2 {
			t.Fatalf("unexpected result: value=%d ok=%v", value, ok)
		}
	})

	t.Run("required int parses value", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/?limit=%2012%20", nil)
		value, err := Query(req).RequiredInt("limit")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if value != 12 {
			t.Fatalf("unexpected value: %d", value)
		}
	})

	t.Run("required int missing", func(t *testing.T) {
		_, err := Query(httptest.NewRequest(http.MethodGet, "/", nil)).RequiredInt("limit")
		assertProblemDetail(t, err, reqx.InQuery, "limit", reqx.DetailCodeRequired)
	})

	t.Run("required int empty is invalid", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/?limit=%20%20", nil)
		_, err := Query(req).RequiredInt("limit")
		assertProblemDetail(t, err, reqx.InQuery, "limit", reqx.DetailCodeInvalidInteger)
	})

	t.Run("required int rejects non numeric", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/?limit=bad", nil)
		_, err := Query(req).RequiredInt("limit")
		assertProblemDetail(t, err, reqx.InQuery, "limit", reqx.DetailCodeInvalidInteger)
	})

	t.Run("required int multiple values", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/?limit=1&limit=2", nil)
		_, err := Query(req).RequiredInt("limit")
		assertProblemDetail(t, err, reqx.InQuery, "limit", reqx.DetailCodeMultipleValues)
	})

	t.Run("int16 missing", func(t *testing.T) {
		value, ok, err := Query(httptest.NewRequest(http.MethodGet, "/", nil)).Int16("status")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if ok || value != 0 {
			t.Fatalf("unexpected result: value=%d ok=%v", value, ok)
		}
	})

	t.Run("int16 empty is invalid", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/?status=%20%20", nil)
		_, _, err := Query(req).Int16("status")
		assertProblemDetail(t, err, reqx.InQuery, "status", reqx.DetailCodeInvalidInteger)
	})

	t.Run("int16 rejects out of range value", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/?status=32768", nil)
		_, _, err := Query(req).Int16("status")
		assertProblemDetail(t, err, reqx.InQuery, "status", reqx.DetailCodeInvalidInteger)
	})

	t.Run("int16 multiple values", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/?status=1&status=2", nil)
		_, _, err := Query(req).Int16("status")
		assertProblemDetail(t, err, reqx.InQuery, "status", reqx.DetailCodeMultipleValues)
	})

	t.Run("required int16 parses value", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/?status=%207%20", nil)
		value, err := Query(req).RequiredInt16("status")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if value != 7 {
			t.Fatalf("unexpected value: %d", value)
		}
	})

	t.Run("required int16 missing", func(t *testing.T) {
		_, err := Query(httptest.NewRequest(http.MethodGet, "/", nil)).RequiredInt16("status")
		assertProblemDetail(t, err, reqx.InQuery, "status", reqx.DetailCodeRequired)
	})

	t.Run("required int16 empty is invalid", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/?status=%20%20", nil)
		_, err := Query(req).RequiredInt16("status")
		assertProblemDetail(t, err, reqx.InQuery, "status", reqx.DetailCodeInvalidInteger)
	})

	t.Run("required int16 rejects out of range value", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/?status=32768", nil)
		_, err := Query(req).RequiredInt16("status")
		assertProblemDetail(t, err, reqx.InQuery, "status", reqx.DetailCodeInvalidInteger)
	})

	t.Run("required int16 multiple values", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/?status=1&status=2", nil)
		_, err := Query(req).RequiredInt16("status")
		assertProblemDetail(t, err, reqx.InQuery, "status", reqx.DetailCodeMultipleValues)
	})

	t.Run("uuid canonicalizes value", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/?uuid=%20"+upperUUID1+"%20", nil)
		value, ok, err := Query(req).UUID("uuid")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !ok || value != canonicalUUID1 {
			t.Fatalf("unexpected result: value=%q ok=%v", value, ok)
		}
	})

	t.Run("uuid empty is invalid", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/?uuid=%20%20%20", nil)
		_, _, err := Query(req).UUID("uuid")
		assertProblemDetail(t, err, reqx.InQuery, "uuid", reqx.DetailCodeInvalidUUID)
	})

	t.Run("uuid missing", func(t *testing.T) {
		value, ok, err := Query(httptest.NewRequest(http.MethodGet, "/", nil)).UUID("uuid")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if ok || value != "" {
			t.Fatalf("unexpected result: value=%q ok=%v", value, ok)
		}
	})

	t.Run("uuid invalid format", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/?uuid=bad", nil)
		_, _, err := Query(req).UUID("uuid")
		assertProblemDetail(t, err, reqx.InQuery, "uuid", reqx.DetailCodeInvalidUUID)
	})

	t.Run("uuid multiple values", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/?uuid="+canonicalUUID1+"&uuid="+canonicalUUID2, nil)
		_, _, err := Query(req).UUID("uuid")
		assertProblemDetail(t, err, reqx.InQuery, "uuid", reqx.DetailCodeMultipleValues)
	})

	t.Run("uuids support repeated keys", func(t *testing.T) {
		req := httptest.NewRequest(
			http.MethodGet,
			"/?tag_uuid="+upperUUID1+"&tag_uuid="+upperUUID2,
			nil,
		)
		values, ok, err := Query(req).UUIDs("tag_uuid")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !ok {
			t.Fatal("expected values")
		}
		assertStrings(t, values, []string{canonicalUUID1, canonicalUUID2})
	})

	t.Run("uuids invalid item", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/?tag_uuid="+canonicalUUID1+"&tag_uuid=bad", nil)
		_, _, err := Query(req).UUIDs("tag_uuid")
		assertProblemDetail(t, err, reqx.InQuery, "tag_uuid", reqx.DetailCodeInvalidUUID)
	})

	t.Run("uuids missing", func(t *testing.T) {
		values, ok, err := Query(httptest.NewRequest(http.MethodGet, "/", nil)).UUIDs("tag_uuid")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if ok || values != nil {
			t.Fatalf("unexpected result: values=%#v ok=%v", values, ok)
		}
	})

	t.Run("uuids reject blank item", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/?tag_uuid=%20%20", nil)
		_, _, err := Query(req).UUIDs("tag_uuid")
		assertProblemDetail(t, err, reqx.InQuery, "tag_uuid", reqx.DetailCodeInvalidValue)
	})

	t.Run("required uuid canonicalizes value", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/?uuid=%20"+upperUUID1+"%20", nil)
		value, err := Query(req).RequiredUUID("uuid")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if value != canonicalUUID1 {
			t.Fatalf("unexpected value: %q", value)
		}
	})

	t.Run("required uuid missing", func(t *testing.T) {
		_, err := Query(httptest.NewRequest(http.MethodGet, "/", nil)).RequiredUUID("uuid")
		assertProblemDetail(t, err, reqx.InQuery, "uuid", reqx.DetailCodeRequired)
	})

	t.Run("required uuid empty is invalid", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/?uuid=%20%20", nil)
		_, err := Query(req).RequiredUUID("uuid")
		assertProblemDetail(t, err, reqx.InQuery, "uuid", reqx.DetailCodeInvalidUUID)
	})

	t.Run("required uuid invalid format", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/?uuid=bad", nil)
		_, err := Query(req).RequiredUUID("uuid")
		assertProblemDetail(t, err, reqx.InQuery, "uuid", reqx.DetailCodeInvalidUUID)
	})

	t.Run("required uuid multiple values", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/?uuid="+canonicalUUID1+"&uuid="+canonicalUUID2, nil)
		_, err := Query(req).RequiredUUID("uuid")
		assertProblemDetail(t, err, reqx.InQuery, "uuid", reqx.DetailCodeMultipleValues)
	})

	t.Run("required uuids support repeated keys", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/?tag_uuid="+upperUUID1+"&tag_uuid="+upperUUID2, nil)
		values, err := Query(req).RequiredUUIDs("tag_uuid")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		assertStrings(t, values, []string{canonicalUUID1, canonicalUUID2})
	})

	t.Run("required uuids missing", func(t *testing.T) {
		_, err := Query(httptest.NewRequest(http.MethodGet, "/", nil)).RequiredUUIDs("tag_uuid")
		assertProblemDetail(t, err, reqx.InQuery, "tag_uuid", reqx.DetailCodeRequired)
	})

	t.Run("required uuids reject blank item", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/?tag_uuid=%20%20", nil)
		_, err := Query(req).RequiredUUIDs("tag_uuid")
		assertProblemDetail(t, err, reqx.InQuery, "tag_uuid", reqx.DetailCodeInvalidValue)
	})

	t.Run("required uuids invalid item", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/?tag_uuid="+canonicalUUID1+"&tag_uuid=bad", nil)
		_, err := Query(req).RequiredUUIDs("tag_uuid")
		assertProblemDetail(t, err, reqx.InQuery, "tag_uuid", reqx.DetailCodeInvalidUUID)
	})

	t.Run("bool missing", func(t *testing.T) {
		value, ok, err := Query(httptest.NewRequest(http.MethodGet, "/", nil)).Bool("enabled")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if ok || value {
			t.Fatalf("unexpected result: value=%v ok=%v", value, ok)
		}
	})

	t.Run("bool parses true", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/?enabled=true", nil)
		value, ok, err := Query(req).Bool("enabled")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !ok || !value {
			t.Fatalf("unexpected result: value=%v ok=%v", value, ok)
		}
	})

	t.Run("bool parses false", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/?enabled=false", nil)
		value, ok, err := Query(req).Bool("enabled")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !ok || value {
			t.Fatalf("unexpected result: value=%v ok=%v", value, ok)
		}
	})

	t.Run("bool only accepts lowercase true false", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/?enabled=True", nil)
		_, _, err := Query(req).Bool("enabled")
		assertProblemDetail(t, err, reqx.InQuery, "enabled", reqx.DetailCodeInvalidValue)
	})

	t.Run("bool multiple values", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/?enabled=true&enabled=false", nil)
		_, _, err := Query(req).Bool("enabled")
		assertProblemDetail(t, err, reqx.InQuery, "enabled", reqx.DetailCodeMultipleValues)
	})

	t.Run("required bool missing", func(t *testing.T) {
		_, err := Query(httptest.NewRequest(http.MethodGet, "/", nil)).RequiredBool("enabled")
		assertProblemDetail(t, err, reqx.InQuery, "enabled", reqx.DetailCodeRequired)
	})

	t.Run("required bool parses true", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/?enabled=true", nil)
		value, err := Query(req).RequiredBool("enabled")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !value {
			t.Fatal("expected true")
		}
	})

	t.Run("required bool parses false", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/?enabled=false", nil)
		value, err := Query(req).RequiredBool("enabled")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if value {
			t.Fatal("expected false")
		}
	})

	t.Run("required bool rejects invalid value", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/?enabled=TRUE", nil)
		_, err := Query(req).RequiredBool("enabled")
		assertProblemDetail(t, err, reqx.InQuery, "enabled", reqx.DetailCodeInvalidValue)
	})

	t.Run("required bool multiple values", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/?enabled=true&enabled=false", nil)
		_, err := Query(req).RequiredBool("enabled")
		assertProblemDetail(t, err, reqx.InQuery, "enabled", reqx.DetailCodeMultipleValues)
	})

	t.Run("nil request is programming error", func(t *testing.T) {
		_, _, err := Query(nil).String("keyword")
		assertPlainErrorMessage(t, err, "paramx: request must not be nil")
	})

	t.Run("strings nil request is programming error", func(t *testing.T) {
		_, _, err := Query(nil).Strings("tag")
		assertPlainErrorMessage(t, err, "paramx: request must not be nil")
	})

	t.Run("blank name is programming error", func(t *testing.T) {
		_, _, err := Query(httptest.NewRequest(http.MethodGet, "/", nil)).String("   ")
		assertPlainErrorMessage(t, err, "paramx: name must not be blank")
	})

	t.Run("strings blank name is programming error", func(t *testing.T) {
		_, _, err := Query(httptest.NewRequest(http.MethodGet, "/", nil)).Strings("   ")
		assertPlainErrorMessage(t, err, "paramx: name must not be blank")
	})
}

func assertProblemDetail(t *testing.T, err error, in string, field string, code string) {
	t.Helper()

	problem, ok := reqx.AsProblem(err)
	if !ok {
		t.Fatalf("expected reqx problem, got %T (%v)", err, err)
	}
	if len(problem.Details) != 1 {
		t.Fatalf("unexpected details: %#v", problem.Details)
	}
	detail := problem.Details[0]
	if detail.In != in || detail.Field != field || detail.Code != code {
		t.Fatalf("unexpected detail: %#v", detail)
	}
}

func assertPlainErrorMessage(t *testing.T, err error, want string) {
	t.Helper()

	if err == nil {
		t.Fatal("expected error")
	}
	if _, ok := reqx.AsProblem(err); ok {
		t.Fatalf("expected plain error, got problem: %#v", err)
	}
	if err.Error() != want {
		t.Fatalf("unexpected error: %v", err)
	}
}

func assertStrings(t *testing.T, got []string, want []string) {
	t.Helper()

	if len(got) != len(want) {
		t.Fatalf("unexpected values: got=%#v want=%#v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("unexpected values: got=%#v want=%#v", got, want)
		}
	}
}

func withRouteParam(req *http.Request, key string, value string) *http.Request {
	rctx := chi.RouteContext(req.Context())
	if rctx == nil {
		rctx = chi.NewRouteContext()
		req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))
	}

	rctx.URLParams.Add(key, value)
	return req
}
