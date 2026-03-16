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

func withRouteParam(req *http.Request, key string, value string) *http.Request {
	rctx := chi.RouteContext(req.Context())
	if rctx == nil {
		rctx = chi.NewRouteContext()
		req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))
	}

	rctx.URLParams.Add(key, value)
	return req
}
