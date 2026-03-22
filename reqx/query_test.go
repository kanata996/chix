package reqx_test

import (
	"encoding"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/kanata996/chix"
	"github.com/kanata996/chix/reqx"
)

type listUsersQuery struct {
	ID        string    `query:"id"`
	Page      int       `query:"page"`
	Active    bool      `query:"active"`
	Tags      []string  `query:"tag"`
	Limit     *int      `query:"limit"`
	CreatedAt time.Time `query:"created_at"`
	Mode      queryMode `query:"mode"`
	Ignored   string
}

type queryMode string

var _ encoding.TextUnmarshaler = (*queryMode)(nil)

func (m *queryMode) UnmarshalText(text []byte) error {
	switch value := queryMode(text); value {
	case "basic", "full":
		*m = value
		return nil
	default:
		return simpleError("invalid mode")
	}
}

func TestDecodeQuerySuccess(t *testing.T) {
	h := chix.Wrap(func(w http.ResponseWriter, r *http.Request) error {
		var query listUsersQuery
		if err := reqx.DecodeQuery(r, &query); err != nil {
			return err
		}

		limit := 0
		if query.Limit != nil {
			limit = *query.Limit
		}

		return chix.Write(w, http.StatusOK, map[string]any{
			"id":         query.ID,
			"page":       query.Page,
			"active":     query.Active,
			"tags":       query.Tags,
			"limit":      limit,
			"created_at": query.CreatedAt.Format(time.RFC3339),
			"mode":       string(query.Mode),
			"ignored":    query.Ignored,
		})
	})

	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/users?id=u_1&page=2&active=true&tag=a&tag=b&limit=50&created_at=2024-01-02T03:04:05Z&mode=full", nil)
	h.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rr.Code, http.StatusOK)
	}

	payload := decodePayload(t, rr)
	data, ok := payload["data"].(map[string]any)
	if !ok {
		t.Fatalf("data = %#v, want object", payload["data"])
	}
	if got := data["id"]; got != "u_1" {
		t.Fatalf("data.id = %#v, want u_1", got)
	}
	if got := data["page"]; got != float64(2) {
		t.Fatalf("data.page = %#v, want 2", got)
	}
	if got := data["active"]; got != true {
		t.Fatalf("data.active = %#v, want true", got)
	}
	rawTags, ok := data["tags"].([]any)
	if !ok || len(rawTags) != 2 || rawTags[0] != "a" || rawTags[1] != "b" {
		t.Fatalf("data.tags = %#v, want [a b]", data["tags"])
	}
	if got := data["limit"]; got != float64(50) {
		t.Fatalf("data.limit = %#v, want 50", got)
	}
	if got := data["created_at"]; got != "2024-01-02T03:04:05Z" {
		t.Fatalf("data.created_at = %#v, want RFC3339 value", got)
	}
	if got := data["mode"]; got != "full" {
		t.Fatalf("data.mode = %#v, want full", got)
	}
	if got := data["ignored"]; got != "" {
		t.Fatalf("data.ignored = %#v, want empty string", got)
	}
}

func TestDecodeQueryLeavesMissingValuesZeroed(t *testing.T) {
	var query struct {
		Page  int  `query:"page"`
		Limit *int `query:"limit"`
	}

	req := httptest.NewRequest(http.MethodGet, "/users", nil)
	if err := reqx.DecodeQuery(req, &query); err != nil {
		t.Fatalf("DecodeQuery() error = %v, want nil", err)
	}
	if query.Page != 0 {
		t.Fatalf("query.Page = %d, want 0", query.Page)
	}
	if query.Limit != nil {
		t.Fatalf("query.Limit = %#v, want nil", query.Limit)
	}
}

func TestDecodeAndValidateQueryRejectsViolations(t *testing.T) {
	h := chix.Wrap(func(w http.ResponseWriter, r *http.Request) error {
		var query struct {
			Page int `query:"page"`
		}
		return reqx.DecodeAndValidateQuery(r, &query, func(value *struct {
			Page int `query:"page"`
		}) []reqx.Violation {
			if value.Page < 1 {
				return []reqx.Violation{{Field: "page", Code: "min", Message: "must be at least 1"}}
			}
			return nil
		})
	})

	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/users?page=0", nil)
	h.ServeHTTP(rr, req)

	assertErrorResponse(t, rr, http.StatusUnprocessableEntity, "invalid_request", "request contains invalid fields")
	assertFirstDetail(t, rr, "page", "min", "must be at least 1")
}

func TestDecodeAndValidateQueryReturnsDecodeError(t *testing.T) {
	h := chix.Wrap(func(w http.ResponseWriter, r *http.Request) error {
		var query struct {
			Page int `query:"page"`
		}
		return reqx.DecodeAndValidateQuery(r, &query, func(value *struct {
			Page int `query:"page"`
		}) []reqx.Violation {
			return nil
		})
	})

	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/users?page=abc", nil)
	h.ServeHTTP(rr, req)

	assertErrorResponse(t, rr, http.StatusUnprocessableEntity, "invalid_request", "request contains invalid fields")
	assertFirstDetail(t, rr, "page", "type", "must be number")
}

func TestDecodeAndValidateQuerySuccess(t *testing.T) {
	type queryInput struct {
		Page int `query:"page"`
	}

	h := chix.Wrap(func(w http.ResponseWriter, r *http.Request) error {
		var query queryInput
		if err := reqx.DecodeAndValidateQuery(r, &query, func(value *queryInput) []reqx.Violation {
			if value.Page < 1 {
				return []reqx.Violation{{Field: "page", Code: "min", Message: "must be at least 1"}}
			}
			return nil
		}); err != nil {
			return err
		}

		return chix.Write(w, http.StatusOK, map[string]any{"page": query.Page})
	})

	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/users?page=2", nil)
	h.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rr.Code, http.StatusOK)
	}

	payload := decodePayload(t, rr)
	data, ok := payload["data"].(map[string]any)
	if !ok {
		t.Fatalf("data = %#v, want object", payload["data"])
	}
	if got := data["page"]; got != float64(2) {
		t.Fatalf("data.page = %#v, want 2", got)
	}
}

func TestDecodeQueryRejectsUnknownFields(t *testing.T) {
	h := chix.Wrap(func(w http.ResponseWriter, r *http.Request) error {
		var query struct {
			ID string `query:"id"`
		}
		return reqx.DecodeQuery(r, &query)
	})

	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/users?id=u_1&extra=yes", nil)
	h.ServeHTTP(rr, req)

	assertErrorResponse(t, rr, http.StatusUnprocessableEntity, "invalid_request", "request contains invalid fields")
	assertFirstDetail(t, rr, "extra", "unknown", "unknown field")
}

func TestDecodeQueryCanAllowUnknownFields(t *testing.T) {
	h := chix.Wrap(func(w http.ResponseWriter, r *http.Request) error {
		var query struct {
			ID string `query:"id"`
		}
		if err := reqx.DecodeQuery(r, &query, reqx.AllowUnknownQueryFields()); err != nil {
			return err
		}
		return chix.Write(w, http.StatusOK, map[string]any{"id": query.ID})
	})

	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/users?id=u_1&extra=yes", nil)
	h.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rr.Code, http.StatusOK)
	}
}

func TestDecodeQueryRejectsInvalidScalarType(t *testing.T) {
	h := chix.Wrap(func(w http.ResponseWriter, r *http.Request) error {
		var query struct {
			Page int `query:"page"`
		}
		return reqx.DecodeQuery(r, &query)
	})

	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/users?page=abc", nil)
	h.ServeHTTP(rr, req)

	assertErrorResponse(t, rr, http.StatusUnprocessableEntity, "invalid_request", "request contains invalid fields")
	assertFirstDetail(t, rr, "page", "type", "must be number")
}

func TestDecodeQueryRejectsRepeatedScalarField(t *testing.T) {
	h := chix.Wrap(func(w http.ResponseWriter, r *http.Request) error {
		var query struct {
			Page int `query:"page"`
		}
		return reqx.DecodeQuery(r, &query)
	})

	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/users?page=1&page=2", nil)
	h.ServeHTTP(rr, req)

	assertErrorResponse(t, rr, http.StatusUnprocessableEntity, "invalid_request", "request contains invalid fields")
	assertFirstDetail(t, rr, "page", "multiple", "must not be repeated")
}

func TestDecodeQueryRejectsInvalidTextUnmarshalerValue(t *testing.T) {
	h := chix.Wrap(func(w http.ResponseWriter, r *http.Request) error {
		var query struct {
			Mode queryMode `query:"mode"`
		}
		return reqx.DecodeQuery(r, &query)
	})

	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/users?mode=broken", nil)
	h.ServeHTTP(rr, req)

	assertErrorResponse(t, rr, http.StatusUnprocessableEntity, "invalid_request", "request contains invalid fields")
	assertFirstDetail(t, rr, "mode", "invalid", "is invalid")
}

func TestDecodeQuerySupportsPointerTextUnmarshaler(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/users?created_at=2024-01-02T03:04:05Z", nil)

	var query struct {
		CreatedAt *time.Time `query:"created_at"`
	}
	if err := reqx.DecodeQuery(req, &query); err != nil {
		t.Fatalf("DecodeQuery() error = %v, want nil", err)
	}
	if query.CreatedAt == nil {
		t.Fatal("query.CreatedAt = nil, want non-nil")
	}
	if got := query.CreatedAt.Format(time.RFC3339); got != "2024-01-02T03:04:05Z" {
		t.Fatalf("query.CreatedAt = %q, want RFC3339 value", got)
	}
}

func TestDecodeQueryRejectsNilRequest(t *testing.T) {
	var query struct {
		ID string `query:"id"`
	}

	err := reqx.DecodeQuery(nil, &query)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if got := err.Error(); got != "chix/reqx: request must not be nil" {
		t.Fatalf("error = %q, want request must not be nil", got)
	}
}

func TestDecodeQueryRejectsNilDestination(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/users?id=u_1", nil)

	err := reqx.DecodeQuery[struct {
		ID string `query:"id"`
	}](req, nil)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if got := err.Error(); got != "chix/reqx: destination must not be nil" {
		t.Fatalf("error = %q, want destination must not be nil", got)
	}
}

func TestDecodeQueryRejectsNonStructDestination(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/users?page=1", nil)

	var page int
	err := reqx.DecodeQuery(req, &page)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if got := err.Error(); got != "chix/reqx: destination must point to a struct" {
		t.Fatalf("error = %q, want destination must point to a struct", got)
	}
}

func TestDecodeQueryRejectsUnsupportedFieldType(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/users", nil)

	var query struct {
		Meta map[string]string `query:"meta"`
	}
	err := reqx.DecodeQuery(req, &query)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if got := err.Error(); got != `chix/reqx: field "Meta" has unsupported query type map[string]string` {
		t.Fatalf("error = %q, want unsupported field type", got)
	}
}

func TestDecodeQueryRejectsDuplicateQueryTags(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/users", nil)

	var query struct {
		ID    string `query:"id"`
		Other string `query:"id"`
	}
	err := reqx.DecodeQuery(req, &query)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if got := err.Error(); got != `chix/reqx: duplicate query field "id" on ID and Other` {
		t.Fatalf("error = %q, want duplicate query field", got)
	}
}

func TestDecodeQueryRejectsUnexportedTaggedField(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/users", nil)

	var query struct {
		id string `query:"id"`
	}
	err := reqx.DecodeQuery(req, &query)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if got := err.Error(); got != `chix/reqx: query field "id" must be exported` {
		t.Fatalf("error = %q, want tagged field must be exported", got)
	}
}

func TestDecodeQueryAggregatesMultipleViolations(t *testing.T) {
	h := chix.Wrap(func(w http.ResponseWriter, r *http.Request) error {
		var query struct {
			Page int `query:"page"`
		}
		return reqx.DecodeQuery(r, &query)
	})

	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/users?page=abc&extra=yes", nil)
	h.ServeHTTP(rr, req)

	payload := assertErrorResponse(t, rr, http.StatusUnprocessableEntity, "invalid_request", "request contains invalid fields")
	rawError := payload["error"].(map[string]any)
	details, ok := rawError["details"].([]any)
	if !ok || len(details) != 2 {
		t.Fatalf("error.details = %#v, want two details", rawError["details"])
	}
}

func TestDecodeQueryIgnoresDashTaggedFields(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/users?ignored=value", nil)

	var query struct {
		Ignored string `query:"-"`
	}
	err := reqx.DecodeQuery(req, &query, reqx.AllowUnknownQueryFields())
	if err != nil {
		t.Fatalf("DecodeQuery() error = %v, want nil", err)
	}
	if query.Ignored != "" {
		t.Fatalf("query.Ignored = %q, want empty string", query.Ignored)
	}
}

func TestDecodeQueryTrimsNothingFromStringValues(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/users?name=+alice+smith+", nil)

	var query struct {
		Name string `query:"name"`
	}
	if err := reqx.DecodeQuery(req, &query); err != nil {
		t.Fatalf("DecodeQuery() error = %v, want nil", err)
	}
	if got := strings.TrimSpace(query.Name); got != "alice smith" {
		t.Fatalf("trimmed query.Name = %q, want alice smith", got)
	}
	if query.Name != " alice smith " {
		t.Fatalf("query.Name = %q, want spaces preserved", query.Name)
	}
}

func TestDecodeQueryHandlesNilURLAndNilOption(t *testing.T) {
	req := &http.Request{}

	var query struct {
		ID string `query:"id"`
	}
	if err := reqx.DecodeQuery(req, &query, nil); err != nil {
		t.Fatalf("DecodeQuery() error = %v, want nil", err)
	}
	if query.ID != "" {
		t.Fatalf("query.ID = %q, want empty string", query.ID)
	}
}

func TestDecodeQueryReturnsDecoderError(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/users?mode=full", nil)

	var query struct {
		Mode encoding.TextUnmarshaler `query:"mode"`
	}
	err := reqx.DecodeQuery(req, &query)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if got := err.Error(); got != "chix/reqx: unsupported text unmarshaler destination type encoding.TextUnmarshaler" {
		t.Fatalf("error = %q, want unsupported text unmarshaler destination type", got)
	}
}
