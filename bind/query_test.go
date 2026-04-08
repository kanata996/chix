package bind

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
)

type queryState string

func (s *queryState) UnmarshalText(text []byte) error {
	switch string(text) {
	case "open", "closed":
		*s = queryState(text)
		return nil
	default:
		return fmt.Errorf("invalid state %q", string(text))
	}
}

func TestBindQueryParams_BindsSupportedTypes(t *testing.T) {
	type request struct {
		Page   int        `query:"page"`
		Search string     `query:"search"`
		State  queryState `query:"state"`
		IDs    []int      `query:"id"`
	}

	req := httptest.NewRequest(http.MethodGet, "/?page=2&search=kanata&state=open&id=1&id=2", nil)

	var dst request
	if err := BindQueryParams(req, &dst); err != nil {
		t.Fatalf("BindQueryParams() error = %v", err)
	}
	if dst.Page != 2 || dst.Search != "kanata" || dst.State != "open" {
		t.Fatalf("dst = %#v", dst)
	}
	if len(dst.IDs) != 2 || dst.IDs[0] != 1 || dst.IDs[1] != 2 {
		t.Fatalf("ids = %#v, want [1 2]", dst.IDs)
	}
}

func TestBindQueryParams_MissingParamsPreserveExistingValues(t *testing.T) {
	type request struct {
		Page   int    `query:"page"`
		Search string `query:"search"`
	}

	req := httptest.NewRequest(http.MethodGet, "/?other=1", nil)
	dst := request{Page: 3, Search: "existing"}

	if err := BindQueryParams(req, &dst); err != nil {
		t.Fatalf("BindQueryParams() error = %v", err)
	}
	if dst.Page != 3 || dst.Search != "existing" {
		t.Fatalf("dst = %#v, want existing values preserved", dst)
	}
}

func TestBindQueryParams_RepeatedScalarUsesFirstValue(t *testing.T) {
	type request struct {
		Page int `query:"page"`
	}

	req := httptest.NewRequest(http.MethodGet, "/?page=1&page=2", nil)

	var dst request
	if err := BindQueryParams(req, &dst); err != nil {
		t.Fatalf("BindQueryParams() error = %v", err)
	}
	if dst.Page != 1 {
		t.Fatalf("page = %d, want 1", dst.Page)
	}
}

func TestBindQueryParams_BindingErrorsAreBadRequest(t *testing.T) {
	type request struct {
		Page int `query:"page"`
	}

	req := httptest.NewRequest(http.MethodGet, "/?page=oops", nil)

	var dst request
	_ = assertHTTPError(t, BindQueryParams(req, &dst), http.StatusBadRequest, "bad_request", "Bad Request")
}
