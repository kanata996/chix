package inputschema

import (
	"reflect"
	"testing"
	"time"
)

type testSlug string

func (s *testSlug) UnmarshalText(text []byte) error {
	*s = testSlug(text)
	return nil
}

func TestLoadBuildsFieldMetadataAndLocations(t *testing.T) {
	type route struct {
		ID string `path:"id"`
	}
	type filters struct {
		Limit int `query:"limit"`
	}
	type payload struct {
		Name string `json:"name"`
	}
	type profile struct {
		Name string `json:"name"`
	}
	type input struct {
		route
		filters
		payload
		Profile profile `json:"profile"`
	}

	schema, err := Load(reflect.TypeOf(input{}))
	if err != nil {
		t.Fatalf("load schema: %v", err)
	}

	if got := schema.ParameterFields; len(got) != 2 {
		t.Fatalf("expected 2 parameter fields, got %+v", got)
	}
	assertField(t, schema.ParameterFields[0], "path", "id", "id", []int{0, 0})
	assertField(t, schema.ParameterFields[1], "query", "limit", "limit", []int{1, 0})

	if got := schema.BodyFields; len(got) != 2 {
		t.Fatalf("expected 2 body fields, got %+v", got)
	}
	assertField(t, schema.BodyFields[0], "body", "name", "name", []int{2, 0})
	assertField(t, schema.BodyFields[1], "body", "profile", "profile", []int{3})
	assertBodyField(t, schema, "name", Field{Source: "body", Name: "name", Path: "name", Index: []int{2, 0}})
	assertBodyField(t, schema, "profile", Field{Source: "body", Name: "profile", Path: "profile", Index: []int{3}})

	assertLocation(t, schema, "input.ID", Location{Source: "path", Field: "id"})
	assertLocation(t, schema, "input.route.ID", Location{Source: "path", Field: "id"})
	assertLocation(t, schema, "input.Limit", Location{Source: "query", Field: "limit"})
	assertLocation(t, schema, "input.filters.Limit", Location{Source: "query", Field: "limit"})
	assertLocation(t, schema, "input.Name", Location{Source: "body", Field: "name"})
	assertLocation(t, schema, "input.payload.Name", Location{Source: "body", Field: "name"})
	assertLocation(t, schema, "input.Profile", Location{Source: "body", Field: "profile"})
	assertLocation(t, schema, "input.Profile.Name", Location{Source: "body", Field: "profile.name"})
}

func TestLoadRejectsNestedUntaggedBodyFields(t *testing.T) {
	type profile struct {
		Name string
	}
	type input struct {
		Profile profile `json:"profile"`
	}

	_, err := Load(reflect.TypeOf(input{}))
	if err == nil {
		t.Fatal("expected schema error")
	}
}

func TestLoadRejectsNestedParameterSourceInsideBody(t *testing.T) {
	type profile struct {
		Name string `query:"name"`
	}
	type input struct {
		Profile profile `json:"profile"`
	}

	_, err := Load(reflect.TypeOf(input{}))
	if err == nil {
		t.Fatal("expected schema error")
	}
}

func TestLoadRejectsDuplicateBodyFields(t *testing.T) {
	type left struct {
		Name string `json:"name"`
	}
	type right struct {
		Name string `json:"name"`
	}
	type input struct {
		left
		right
	}

	_, err := Load(reflect.TypeOf(input{}))
	if err == nil {
		t.Fatal("expected duplicate body field error")
	}
}

func TestLoadIgnoresExplicitlySkippedNestedBodyFields(t *testing.T) {
	type profile struct {
		Name   string `json:"name"`
		Secret string `json:"-"`
	}
	type input struct {
		Profile profile `json:"profile"`
	}

	schema, err := Load(reflect.TypeOf(input{}))
	if err != nil {
		t.Fatalf("load schema: %v", err)
	}

	assertLocation(t, schema, "input.Profile.Name", Location{Source: "body", Field: "profile.name"})

	if _, ok := schema.LookupLocation("input.Profile.Secret"); ok {
		t.Fatalf("expected ignored field to be absent from locations")
	}
}

func TestLoadCachesByType(t *testing.T) {
	type input struct {
		ID string `path:"id"`
	}

	first, err := Load(reflect.TypeOf(input{}))
	if err != nil {
		t.Fatalf("load schema: %v", err)
	}

	second, err := Load(reflect.TypeOf(&input{}))
	if err != nil {
		t.Fatalf("load schema from pointer: %v", err)
	}

	if first != second {
		t.Fatalf("expected cached schema instance, got %p and %p", first, second)
	}
}

func TestLoadValidatesParameterFieldTypes(t *testing.T) {
	type unsupported struct {
		Limit struct{} `query:"limit"`
	}

	if _, err := Load(reflect.TypeOf(unsupported{})); err == nil {
		t.Fatal("expected unsupported parameter type error")
	}
}

func TestLoadRejectsPointerToSliceParameterFields(t *testing.T) {
	type unsupported struct {
		Tags *[]string `query:"tag"`
	}

	if _, err := Load(reflect.TypeOf(unsupported{})); err == nil {
		t.Fatal("expected unsupported parameter type error")
	}
}

func TestLoadAcceptsSupportedParameterFieldTypes(t *testing.T) {
	type input struct {
		ID      *int       `path:"id"`
		At      time.Time  `query:"at"`
		Tags    []string   `query:"tag"`
		Aliases []testSlug `query:"alias"`
		Slug    testSlug   `query:"slug"`
	}

	if _, err := Load(reflect.TypeOf(input{})); err != nil {
		t.Fatalf("expected supported parameter types, got %v", err)
	}
}

func assertField(t *testing.T, got Field, source string, name string, path string, index []int) {
	t.Helper()

	if got.Source != source || got.Name != name || got.Path != path || !reflect.DeepEqual(got.Index, index) {
		t.Fatalf("unexpected field metadata: %+v", got)
	}
}

func assertLocation(t *testing.T, schema *Schema, namespace string, want Location) {
	t.Helper()

	got, ok := schema.LookupLocation(namespace)
	if !ok {
		t.Fatalf("expected location for %q", namespace)
	}
	if got != want {
		t.Fatalf("unexpected location for %q: got %+v want %+v", namespace, got, want)
	}
}

func assertBodyField(t *testing.T, schema *Schema, name string, want Field) {
	t.Helper()

	got, ok := schema.LookupBodyField(name)
	if !ok {
		t.Fatalf("expected body field %q", name)
	}
	if got.Source != want.Source || got.Name != want.Name || got.Path != want.Path || !reflect.DeepEqual(got.Index, want.Index) {
		t.Fatalf("unexpected body field %q: got %+v want %+v", name, got, want)
	}
}
