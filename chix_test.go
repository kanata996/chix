package chix

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-chi/chi/v5"
)

type pathReaderContract interface {
	String(string) (string, error)
	UUID(string) (string, error)
	Int(string) (int, error)
}

type queryReaderContract interface {
	String(string) (string, bool, error)
	Strings(string) ([]string, bool, error)
	RequiredString(string) (string, error)
	RequiredStrings(string) ([]string, error)
	Int(string) (int, bool, error)
	RequiredInt(string) (int, error)
	Int16(string) (int16, bool, error)
	RequiredInt16(string) (int16, error)
	UUID(string) (string, bool, error)
	UUIDs(string) ([]string, bool, error)
	RequiredUUID(string) (string, error)
	RequiredUUIDs(string) ([]string, error)
	Bool(string) (bool, bool, error)
	RequiredBool(string) (bool, error)
}

type headerReaderContract interface {
	String(string) (string, bool, error)
	Strings(string) ([]string, bool, error)
	RequiredString(string) (string, error)
	RequiredStrings(string) ([]string, error)
	Int(string) (int, bool, error)
	RequiredInt(string) (int, error)
	UUID(string) (string, bool, error)
	RequiredUUID(string) (string, error)
	Bool(string) (bool, bool, error)
	RequiredBool(string) (bool, error)
}

func TestReaderContracts(t *testing.T) {
	var _ pathReaderContract = PathReader{}
	var _ queryReaderContract = QueryReader{}
	var _ headerReaderContract = HeaderReader{}
}

func TestPathReader_ForwardsSemantics(t *testing.T) {
	req := withRouteParam(httptest.NewRequest(http.MethodGet, "/", nil), "uuid", "A2B5AA07-F5E5-4C6D-921D-00F13ED4E580")

	value, err := Path(req).UUID("uuid")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if value != "a2b5aa07-f5e5-4c6d-921d-00f13ed4e580" {
		t.Fatalf("unexpected value: %q", value)
	}
}

func TestQueryReader_ForwardsSemantics(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/?enabled=false", nil)

	value, ok, err := Query(req).Bool("enabled")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !ok || value {
		t.Fatalf("unexpected result: value=%v ok=%v", value, ok)
	}
}

func TestQueryReader_CoversPublicMethods(t *testing.T) {
	req := httptest.NewRequest(
		http.MethodGet,
		"/?keyword=books&tag=a&tag=b&limit=42&status=7&uuid=a2b5aa07-f5e5-4c6d-921d-00f13ed4e580&tag_uuid=a2b5aa07-f5e5-4c6d-921d-00f13ed4e580&tag_uuid=bbfe2586-65ec-4cf1-b4d5-85b5c6ab3100&enabled=true",
		nil,
	)

	reader := Query(req)

	if value, ok, err := reader.String("keyword"); err != nil || !ok || value != "books" {
		t.Fatalf("String mismatch: value=%q ok=%v err=%v", value, ok, err)
	}
	if values, ok, err := reader.Strings("tag"); err != nil || !ok || len(values) != 2 {
		t.Fatalf("Strings mismatch: values=%#v ok=%v err=%v", values, ok, err)
	}
	if value, err := reader.RequiredString("keyword"); err != nil || value != "books" {
		t.Fatalf("RequiredString mismatch: value=%q err=%v", value, err)
	}
	if values, err := reader.RequiredStrings("tag"); err != nil || len(values) != 2 {
		t.Fatalf("RequiredStrings mismatch: values=%#v err=%v", values, err)
	}
	if value, ok, err := reader.Int("limit"); err != nil || !ok || value != 42 {
		t.Fatalf("Int mismatch: value=%d ok=%v err=%v", value, ok, err)
	}
	if value, err := reader.RequiredInt("limit"); err != nil || value != 42 {
		t.Fatalf("RequiredInt mismatch: value=%d err=%v", value, err)
	}
	if value, ok, err := reader.Int16("status"); err != nil || !ok || value != 7 {
		t.Fatalf("Int16 mismatch: value=%d ok=%v err=%v", value, ok, err)
	}
	if value, err := reader.RequiredInt16("status"); err != nil || value != 7 {
		t.Fatalf("RequiredInt16 mismatch: value=%d err=%v", value, err)
	}
	if value, ok, err := reader.UUID("uuid"); err != nil || !ok || value != "a2b5aa07-f5e5-4c6d-921d-00f13ed4e580" {
		t.Fatalf("UUID mismatch: value=%q ok=%v err=%v", value, ok, err)
	}
	if values, ok, err := reader.UUIDs("tag_uuid"); err != nil || !ok || len(values) != 2 {
		t.Fatalf("UUIDs mismatch: values=%#v ok=%v err=%v", values, ok, err)
	}
	if value, err := reader.RequiredUUID("uuid"); err != nil || value != "a2b5aa07-f5e5-4c6d-921d-00f13ed4e580" {
		t.Fatalf("RequiredUUID mismatch: value=%q err=%v", value, err)
	}
	if values, err := reader.RequiredUUIDs("tag_uuid"); err != nil || len(values) != 2 {
		t.Fatalf("RequiredUUIDs mismatch: values=%#v err=%v", values, err)
	}
	if value, ok, err := reader.Bool("enabled"); err != nil || !ok || !value {
		t.Fatalf("Bool mismatch: value=%v ok=%v err=%v", value, ok, err)
	}
	if value, err := reader.RequiredBool("enabled"); err != nil || !value {
		t.Fatalf("RequiredBool mismatch: value=%v err=%v", value, err)
	}
}

func TestHeaderReader_ForwardsSemantics(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("X-Request-ID", " req-123 ")

	value, ok, err := Header(req).String("X-Request-ID")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !ok || value != "req-123" {
		t.Fatalf("unexpected result: value=%q ok=%v", value, ok)
	}
}

func TestHeaderReader_CoversPublicMethods(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("X-Request-ID", "req-123")
	req.Header.Add("X-Tag", "a")
	req.Header.Add("X-Tag", "b")
	req.Header.Set("X-Limit", "42")
	req.Header.Set("X-Resource-UUID", "a2b5aa07-f5e5-4c6d-921d-00f13ed4e580")
	req.Header.Set("X-Enabled", "true")

	reader := Header(req)

	if value, ok, err := reader.String("X-Request-ID"); err != nil || !ok || value != "req-123" {
		t.Fatalf("String mismatch: value=%q ok=%v err=%v", value, ok, err)
	}
	if values, ok, err := reader.Strings("X-Tag"); err != nil || !ok || len(values) != 2 {
		t.Fatalf("Strings mismatch: values=%#v ok=%v err=%v", values, ok, err)
	}
	if value, err := reader.RequiredString("X-Request-ID"); err != nil || value != "req-123" {
		t.Fatalf("RequiredString mismatch: value=%q err=%v", value, err)
	}
	if values, err := reader.RequiredStrings("X-Tag"); err != nil || len(values) != 2 {
		t.Fatalf("RequiredStrings mismatch: values=%#v err=%v", values, err)
	}
	if value, ok, err := reader.Int("X-Limit"); err != nil || !ok || value != 42 {
		t.Fatalf("Int mismatch: value=%d ok=%v err=%v", value, ok, err)
	}
	if value, err := reader.RequiredInt("X-Limit"); err != nil || value != 42 {
		t.Fatalf("RequiredInt mismatch: value=%d err=%v", value, err)
	}
	if value, ok, err := reader.UUID("X-Resource-UUID"); err != nil || !ok || value != "a2b5aa07-f5e5-4c6d-921d-00f13ed4e580" {
		t.Fatalf("UUID mismatch: value=%q ok=%v err=%v", value, ok, err)
	}
	if value, err := reader.RequiredUUID("X-Resource-UUID"); err != nil || value != "a2b5aa07-f5e5-4c6d-921d-00f13ed4e580" {
		t.Fatalf("RequiredUUID mismatch: value=%q err=%v", value, err)
	}
	if value, ok, err := reader.Bool("X-Enabled"); err != nil || !ok || !value {
		t.Fatalf("Bool mismatch: value=%v ok=%v err=%v", value, ok, err)
	}
	if value, err := reader.RequiredBool("X-Enabled"); err != nil || !value {
		t.Fatalf("RequiredBool mismatch: value=%v err=%v", value, err)
	}
}

func TestPathReader_CoversPublicMethods(t *testing.T) {
	reqUUID := withRouteParam(httptest.NewRequest(http.MethodGet, "/", nil), "uuid", "a2b5aa07-f5e5-4c6d-921d-00f13ed4e580")
	reqID := withRouteParam(httptest.NewRequest(http.MethodGet, "/", nil), "id", "42")

	if value, err := Path(reqUUID).String("uuid"); err != nil || value != "a2b5aa07-f5e5-4c6d-921d-00f13ed4e580" {
		t.Fatalf("String mismatch: value=%q err=%v", value, err)
	}
	if value, err := Path(reqUUID).UUID("uuid"); err != nil || value != "a2b5aa07-f5e5-4c6d-921d-00f13ed4e580" {
		t.Fatalf("UUID mismatch: value=%q err=%v", value, err)
	}
	if value, err := Path(reqID).Int("id"); err != nil || value != 42 {
		t.Fatalf("Int mismatch: value=%d err=%v", value, err)
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
