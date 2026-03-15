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

	t.Run("trailing empty object", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(`{} {}`))
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()

		var got struct{}
		err := DecodeJSON(rec, req, &got)
		problem, ok := AsProblem(err)
		if !ok {
			t.Fatalf("expected problem, got %T (%v)", err, err)
		}
		if len(problem.Details) != 1 || problem.Details[0] != TrailingData() {
			t.Fatalf("unexpected details: %#v", problem.Details)
		}
	})

	t.Run("payload too large while checking trailing whitespace", func(t *testing.T) {
		body := `{"name":"alice"}` + strings.Repeat(" ", int(DefaultJSONMaxBytes))
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

	t.Run("boundary errors", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(`{"name":"alice"}`))
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()
		var got payload

		if err := DecodeJSON(nil, req, &got); !errors.Is(err, ErrNilResponseWriter) {
			t.Fatalf("expected ErrNilResponseWriter, got %v", err)
		}
		if err := DecodeJSON(rec, nil, &got); !errors.Is(err, ErrNilRequest) {
			t.Fatalf("expected ErrNilRequest, got %v", err)
		}
		if err := DecodeJSON(rec, req, nil); !errors.Is(err, ErrNilTarget) {
			t.Fatalf("expected ErrNilTarget, got %v", err)
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
		if !errors.Is(err, ErrInvalidDecodeTarget) {
			t.Fatalf("expected ErrInvalidDecodeTarget, got %v", err)
		}
		if _, ok := AsProblem(err); ok {
			t.Fatalf("expected plain error, got problem: %#v", err)
		}

		var nilTarget *payload
		err = DecodeJSON(rec, req, nilTarget)
		if err == nil {
			t.Fatal("expected nil pointer target error")
		}
		if !errors.Is(err, ErrInvalidDecodeTarget) {
			t.Fatalf("expected ErrInvalidDecodeTarget, got %v", err)
		}
		if _, ok := AsProblem(err); ok {
			t.Fatalf("expected plain error, got problem: %#v", err)
		}
	})
}

func TestValidateDecodeTarget(t *testing.T) {
	if err := validateDecodeTarget(nil); !errors.Is(err, ErrNilTarget) {
		t.Fatalf("expected ErrNilTarget, got %v", err)
	}
}

func TestDecodeJSONWith(t *testing.T) {
	type payload struct {
		Name string `json:"name"`
	}

	t.Run("allow unknown fields", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(`{"name":"alice","extra":1}`))
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()

		var got payload
		if err := DecodeJSONWith(rec, req, &got, DecodeOptions{AllowUnknownFields: true}); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if got.Name != "alice" {
			t.Fatalf("unexpected payload: %#v", got)
		}
	})

	t.Run("skip content type check", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(`{"name":"alice"}`))
		rec := httptest.NewRecorder()

		var got payload
		if err := DecodeJSONWith(rec, req, &got, DecodeOptions{SkipContentTypeCheck: true}); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	t.Run("custom max bytes", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(`{"name":"alice"}`))
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()

		var got payload
		err := DecodeJSONWith(rec, req, &got, DecodeOptions{MaxBytes: 8})
		problem, ok := AsProblem(err)
		if !ok {
			t.Fatalf("expected problem, got %T (%v)", err, err)
		}
		if problem.StatusCode != http.StatusRequestEntityTooLarge {
			t.Fatalf("unexpected status: %d", problem.StatusCode)
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
			name: "type error without field falls back to body",
			err:  &json.UnmarshalTypeError{},
			want: InvalidType(InBody, "body"),
		},
		{
			name: "unknown field",
			err:  errors.New(`json: unknown field "extra"`),
			want: UnknownField("extra"),
		},
		{
			name: "unknown field without name falls back to body",
			err:  errors.New(`json: unknown field ""`),
			want: UnknownField("body"),
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
