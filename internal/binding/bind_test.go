package binding

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestBindClassifiesRequestShapeErrors(t *testing.T) {
	type input struct {
		Name string `json:"name"`
	}

	req := httptest.NewRequest(http.MethodPost, "/users", strings.NewReader(`{"extra":"Ada"}`))
	req.Header.Set("Content-Type", "application/json")

	var dst input
	err := Bind(req, &dst)
	if KindOf(err) != ErrorKindRequestShape {
		t.Fatalf("expected request shape error, got %v", err)
	}
}

func TestBindClassifiesUnsupportedMediaType(t *testing.T) {
	type input struct {
		Name string `json:"name"`
	}

	req := httptest.NewRequest(http.MethodPost, "/users", strings.NewReader(`name=Ada`))
	req.Header.Set("Content-Type", "text/plain")

	var dst input
	err := Bind(req, &dst)
	if KindOf(err) != ErrorKindUnsupportedMediaType {
		t.Fatalf("expected unsupported media type error, got %v", err)
	}
}

func TestBindLeavesConfigurationErrorsUnclassified(t *testing.T) {
	type input struct {
		ID string `path:"id" json:"id"`
	}

	req := httptest.NewRequest(http.MethodPost, "/users/u_1", strings.NewReader(`{"id":"u_1"}`))
	req.Header.Set("Content-Type", "application/json")

	var dst input
	err := Bind(req, &dst)
	if err == nil {
		t.Fatal("expected configuration error")
	}
	if KindOf(err) != ErrorKindUnknown {
		t.Fatalf("expected unclassified error, got kind %d", KindOf(err))
	}
}

func TestBindLeavesNestedUntaggedBodyFieldsUnclassified(t *testing.T) {
	type profile struct {
		Name string
	}
	type input struct {
		Profile profile `json:"profile"`
	}

	req := httptest.NewRequest(http.MethodPost, "/users", strings.NewReader(`{"profile":{"name":"Ada"}}`))
	req.Header.Set("Content-Type", "application/json")

	var dst input
	err := Bind(req, &dst)
	if err == nil {
		t.Fatal("expected configuration error")
	}
	if KindOf(err) != ErrorKindUnknown {
		t.Fatalf("expected unclassified error, got kind %d", KindOf(err))
	}
}

func TestBindClassifiesNestedUnknownBodyFieldErrors(t *testing.T) {
	type profile struct {
		Name string `json:"name"`
	}
	type input struct {
		Profile profile `json:"profile"`
	}

	req := httptest.NewRequest(http.MethodPost, "/users", strings.NewReader(`{"profile":{"name":"Ada","extra":"x"}}`))
	req.Header.Set("Content-Type", "application/json")

	var dst input
	err := Bind(req, &dst)
	if KindOf(err) != ErrorKindRequestShape {
		t.Fatalf("expected request shape error, got %v", err)
	}
}

func TestBindAllowsNestedExplicitlyIgnoredBodyFields(t *testing.T) {
	type profile struct {
		Name   string `json:"name"`
		Secret string `json:"-"`
	}
	type input struct {
		Profile profile `json:"profile"`
	}

	req := httptest.NewRequest(http.MethodPost, "/users", strings.NewReader(`{"profile":{"name":"Ada"}}`))
	req.Header.Set("Content-Type", "application/json")

	var dst input
	if err := Bind(req, &dst); err != nil {
		t.Fatalf("bind request: %v", err)
	}

	if dst.Profile.Name != "Ada" {
		t.Fatalf("expected explicit body field to bind, got %+v", dst)
	}
	if dst.Profile.Secret != "" {
		t.Fatalf("expected ignored body field to remain zero value, got %+v", dst)
	}
}
