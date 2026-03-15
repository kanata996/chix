package reqx

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestDecodeJSON(t *testing.T) {
	type payload struct {
		Name string `json:"name"`
	}

	t.Run("success", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(`{"name":" alice "}`))
		req.Header.Set("Content-Type", "application/json; charset=utf-8")
		rec := httptest.NewRecorder()

		var got payload
		if err := DecodeJSON(rec, req, &got); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if got.Name != " alice " {
			t.Fatalf("unexpected payload: %#v", got)
		}
	})

	t.Run("unsupported media type", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(`{"name":"alice"}`))
		rec := httptest.NewRecorder()

		var got payload
		err := DecodeJSON(rec, req, &got)
		problem, ok := AsProblem(err)
		if !ok {
			t.Fatalf("expected problem, got %T (%v)", err, err)
		}
		if problem.StatusCode != http.StatusUnsupportedMediaType {
			t.Fatalf("unexpected status: %d", problem.StatusCode)
		}
	})

	t.Run("unknown field", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(`{"name":"alice","extra":1}`))
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()

		var got payload
		err := DecodeJSON(rec, req, &got)
		problem, ok := AsProblem(err)
		if !ok {
			t.Fatalf("expected problem, got %T (%v)", err, err)
		}
		if problem.StatusCode != http.StatusBadRequest {
			t.Fatalf("unexpected status: %d", problem.StatusCode)
		}
		if len(problem.Details) != 1 || problem.Details[0] != UnknownField("extra") {
			t.Fatalf("unexpected details: %#v", problem.Details)
		}
	})

	t.Run("empty body", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/", http.NoBody)
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()

		var got payload
		err := DecodeJSON(rec, req, &got)
		problem, ok := AsProblem(err)
		if !ok {
			t.Fatalf("expected problem, got %T (%v)", err, err)
		}
		if problem.StatusCode != http.StatusBadRequest {
			t.Fatalf("unexpected status: %d", problem.StatusCode)
		}
		if len(problem.Details) != 1 || problem.Details[0] != Required(InBody, "body") {
			t.Fatalf("unexpected details: %#v", problem.Details)
		}
	})

	t.Run("invalid type", func(t *testing.T) {
		type countPayload struct {
			Count int `json:"count"`
		}

		req := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(`{"count":"bad"}`))
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()

		var got countPayload
		err := DecodeJSON(rec, req, &got)
		problem, ok := AsProblem(err)
		if !ok {
			t.Fatalf("expected problem, got %T (%v)", err, err)
		}
		if problem.StatusCode != http.StatusBadRequest {
			t.Fatalf("unexpected status: %d", problem.StatusCode)
		}
		if len(problem.Details) != 1 || problem.Details[0] != InvalidType(InBody, "count") {
			t.Fatalf("unexpected details: %#v", problem.Details)
		}
	})

	t.Run("malformed json", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(`{"name":`))
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()

		var got payload
		err := DecodeJSON(rec, req, &got)
		problem, ok := AsProblem(err)
		if !ok {
			t.Fatalf("expected problem, got %T (%v)", err, err)
		}
		if problem.StatusCode != http.StatusBadRequest {
			t.Fatalf("unexpected status: %d", problem.StatusCode)
		}
		if len(problem.Details) != 1 || problem.Details[0] != MalformedJSON() {
			t.Fatalf("unexpected details: %#v", problem.Details)
		}
	})

	t.Run("payload too large", func(t *testing.T) {
		body := `{"name":"` + strings.Repeat("a", int(DefaultJSONMaxBytes)) + `"}`
		req := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()

		var got payload
		err := DecodeJSON(rec, req, &got)
		problem, ok := AsProblem(err)
		if !ok {
			t.Fatalf("expected problem, got %T (%v)", err, err)
		}
		if problem.StatusCode != http.StatusRequestEntityTooLarge {
			t.Fatalf("unexpected status: %d", problem.StatusCode)
		}
	})

	t.Run("trailing data", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(`{"name":"alice"}{"name":"bob"}`))
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()

		var got payload
		err := DecodeJSON(rec, req, &got)
		problem, ok := AsProblem(err)
		if !ok {
			t.Fatalf("expected problem, got %T (%v)", err, err)
		}
		if len(problem.Details) != 1 || problem.Details[0] != TrailingData() {
			t.Fatalf("unexpected details: %#v", problem.Details)
		}
	})

	t.Run("boundary errors", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(`{"name":"alice"}`))
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()
		var got payload

		if err := DecodeJSON(nil, req, &got); err == nil {
			t.Fatal("expected nil writer error")
		}
		if err := DecodeJSON(rec, nil, &got); err == nil {
			t.Fatal("expected nil request error")
		}
		if err := DecodeJSON(rec, req, nil); err == nil {
			t.Fatal("expected nil target error")
		}
	})

	t.Run("invalid decode target is boundary error", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(`{"name":"alice"}`))
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()

		err := DecodeJSON(rec, req, payload{})
		if err == nil {
			t.Fatal("expected invalid target error")
		}
		if _, ok := AsProblem(err); ok {
			t.Fatalf("expected plain error, got problem: %#v", err)
		}

		var nilTarget *payload
		err = DecodeJSON(rec, req, nilTarget)
		if err == nil {
			t.Fatal("expected nil pointer target error")
		}
		if _, ok := AsProblem(err); ok {
			t.Fatalf("expected plain error, got problem: %#v", err)
		}
	})
}

func TestMapJSONError(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want Detail
	}{
		{
			name: "payload too large",
			err:  &http.MaxBytesError{Limit: 8},
			want: Detail{In: InBody, Field: "body", Code: DetailCodePayloadTooLarge},
		},
		{
			name: "empty body",
			err:  io.EOF,
			want: Required(InBody, "body"),
		},
		{
			name: "syntax error",
			err:  &json.SyntaxError{},
			want: MalformedJSON(),
		},
		{
			name: "type error",
			err:  &json.UnmarshalTypeError{Field: "count"},
			want: InvalidType(InBody, "count"),
		},
		{
			name: "unknown field",
			err:  errors.New(`json: unknown field "extra"`),
			want: UnknownField("extra"),
		},
		{
			name: "fallback",
			err:  errors.New("unexpected"),
			want: MalformedJSON(),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			problem, ok := AsProblem(mapJSONError(tt.err))
			if !ok {
				t.Fatalf("expected problem")
			}
			if tt.want.Code == DetailCodePayloadTooLarge {
				if problem.StatusCode != http.StatusRequestEntityTooLarge {
					t.Fatalf("unexpected status: %d", problem.StatusCode)
				}
			} else if problem.StatusCode != http.StatusBadRequest {
				t.Fatalf("unexpected status: %d", problem.StatusCode)
			}
			if len(problem.Details) != 1 || problem.Details[0] != tt.want {
				t.Fatalf("unexpected details: %#v", problem.Details)
			}
		})
	}
}
