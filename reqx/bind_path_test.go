package reqx

import (
	"net/http"
	"testing"
)

func TestBindPathValues_BindsScalars(t *testing.T) {
	type request struct {
		ID   int    `param:"id"`
		Name string `param:"name"`
	}

	req := requestWithPathParams(map[string][]string{
		"id":   {"42"},
		"name": {"kanata"},
	})

	var dst request
	if err := BindPathValues(req, &dst); err != nil {
		t.Fatalf("BindPathValues() error = %v", err)
	}
	if dst.ID != 42 || dst.Name != "kanata" {
		t.Fatalf("dst = %#v, want bound path values", dst)
	}
}

func TestBindPathValues_MissingParamsPreserveExistingValues(t *testing.T) {
	type request struct {
		ID   int    `param:"id"`
		Name string `param:"name"`
	}

	req := requestWithPathParams(map[string][]string{
		"other": {"1"},
	})
	dst := request{ID: 7, Name: "existing"}

	if err := BindPathValues(req, &dst); err != nil {
		t.Fatalf("BindPathValues() error = %v", err)
	}
	if dst.ID != 7 || dst.Name != "existing" {
		t.Fatalf("dst = %#v, want existing values preserved", dst)
	}
}

func TestBindPathValues_EmptyValueBindsZeroValue(t *testing.T) {
	type request struct {
		ID int `param:"id"`
	}

	req := requestWithPathParams(map[string][]string{
		"id": {""},
	})

	var dst request
	if err := BindPathValues(req, &dst); err != nil {
		t.Fatalf("BindPathValues() error = %v", err)
	}
	if dst.ID != 0 {
		t.Fatalf("id = %d, want 0", dst.ID)
	}
}

func TestBindPathValues_BindingErrorsAreBadRequest(t *testing.T) {
	type request struct {
		ID int `param:"id"`
	}

	req := requestWithPathParams(map[string][]string{
		"id": {"oops"},
	})

	var dst request
	_ = assertHTTPError(t, BindPathValues(req, &dst), http.StatusBadRequest, "bad_request", "Bad Request")
}

func TestBindAndValidatePath_ReturnsBindingErrorBeforeValidation(t *testing.T) {
	type request struct {
		ID int `param:"id" validate:"required"`
	}

	req := requestWithPathParams(map[string][]string{
		"id": {"oops"},
	})

	var dst request
	_ = assertHTTPError(t, BindAndValidatePath(req, &dst), http.StatusBadRequest, "bad_request", "Bad Request")
}
