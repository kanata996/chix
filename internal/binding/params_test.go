package binding

import (
	"context"
	"net/http"
	"net/http/httptest"
	"reflect"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
)

type testText string

func (t *testText) UnmarshalText(text []byte) error {
	*t = testText(text)
	return nil
}

func TestAssignValues(t *testing.T) {
	t.Run("pointer scalar", func(t *testing.T) {
		var target *int
		if err := assignValues(reflect.ValueOf(&target).Elem(), []string{"42"}); err != nil {
			t.Fatalf("assign pointer scalar: %v", err)
		}
		if target == nil || *target != 42 {
			t.Fatalf("unexpected pointer scalar value: %+v", target)
		}
	})

	t.Run("time", func(t *testing.T) {
		var target time.Time
		if err := assignValues(reflect.ValueOf(&target).Elem(), []string{"2026-03-28T09:30:00Z"}); err != nil {
			t.Fatalf("assign time: %v", err)
		}
		if target.Format(time.RFC3339) != "2026-03-28T09:30:00Z" {
			t.Fatalf("unexpected time value: %s", target.Format(time.RFC3339))
		}
	})

	t.Run("slice text unmarshaler", func(t *testing.T) {
		var target []testText
		if err := assignValues(reflect.ValueOf(&target).Elem(), []string{"a", "b"}); err != nil {
			t.Fatalf("assign text slice: %v", err)
		}
		if !reflect.DeepEqual(target, []testText{"a", "b"}) {
			t.Fatalf("unexpected slice value: %+v", target)
		}
	})

	t.Run("unsupported", func(t *testing.T) {
		var target struct{}
		if err := assignValues(reflect.ValueOf(&target).Elem(), []string{"x"}); err == nil {
			t.Fatal("expected unsupported type error")
		}
	})
}

func TestBindBindsParameterTypes(t *testing.T) {
	type input struct {
		ID    *int       `path:"id"`
		At    time.Time  `query:"at"`
		Tags  []testText `query:"tag"`
		Alias testText   `query:"alias"`
	}

	req := httptest.NewRequest(http.MethodGet, "/users/42?at=2026-03-28T09:30:00Z&tag=a&tag=b&alias=primary", nil)
	routeContext := chi.NewRouteContext()
	routeContext.URLParams.Add("id", "42")
	req = req.WithContext(contextWithRouteContext(req, routeContext))

	var dst input
	if err := Bind(req, &dst); err != nil {
		t.Fatalf("bind request: %v", err)
	}

	if dst.ID == nil || *dst.ID != 42 {
		t.Fatalf("unexpected id: %+v", dst.ID)
	}
	if dst.At.Format(time.RFC3339) != "2026-03-28T09:30:00Z" {
		t.Fatalf("unexpected time value: %s", dst.At.Format(time.RFC3339))
	}
	if !reflect.DeepEqual(dst.Tags, []testText{"a", "b"}) {
		t.Fatalf("unexpected tags: %+v", dst.Tags)
	}
	if dst.Alias != "primary" {
		t.Fatalf("unexpected alias: %q", dst.Alias)
	}
}

func contextWithRouteContext(req *http.Request, routeContext *chi.Context) context.Context {
	return context.WithValue(req.Context(), chi.RouteCtxKey, routeContext)
}
