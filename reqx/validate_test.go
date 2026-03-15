package reqx

import (
	"net/http"
	"reflect"
	"strings"
	"testing"
)

type validateLoginDTO struct {
	Phone    string `json:"phone" validate:"required,len=11,numeric"`
	Password string `json:"password" validate:"required,min=8"`
}

type normalizedJSONPayload struct {
	Name string `json:"name" validate:"required"`
}

func (p *normalizedJSONPayload) Normalize() {
	if p == nil {
		return
	}
	p.Name = strings.TrimSpace(p.Name)
}

type queryDTO struct {
	Limit        int    `query:"limit" validate:"omitempty,min=1,max=100"`
	CategoryUUID string `query:"category_uuid" validate:"omitempty,uuid"`
	Status       *int16 `query:"status" validate:"omitempty,oneof=0 1 2"`
}

type normalizedQueryDTO struct {
	Keyword string `query:"keyword" validate:"required"`
}

func (q *normalizedQueryDTO) Normalize() {
	if q == nil {
		return
	}
	q.Keyword = strings.TrimSpace(q.Keyword)
}

type pathDTO struct {
	UUID string `param:"uuid" validate:"required,uuid"`
}

func TestValidateBody(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		payload := normalizedJSONPayload{Name: " raw "}
		if err := ValidateBody(&payload); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if payload.Name != "raw" {
			t.Fatalf("expected normalized payload, got %#v", payload)
		}
	})

	t.Run("validation failed", func(t *testing.T) {
		payload := normalizedJSONPayload{}
		err := ValidateBody(&payload)
		problem, ok := AsProblem(err)
		if !ok {
			t.Fatalf("expected problem, got %T (%v)", err, err)
		}
		if problem.StatusCode != http.StatusUnprocessableEntity {
			t.Fatalf("unexpected status: %d", problem.StatusCode)
		}
		if len(problem.Details) != 1 || problem.Details[0].Field != "name" {
			t.Fatalf("unexpected details: %#v", problem.Details)
		}
	})

	t.Run("uses json tag", func(t *testing.T) {
		type payload struct {
			Name string `json:"body_name" validate:"required"`
		}

		err := ValidateBody(&payload{})
		problem, ok := AsProblem(err)
		if !ok {
			t.Fatalf("expected problem, got %T (%v)", err, err)
		}
		if len(problem.Details) != 1 || problem.Details[0].Field != "body_name" || problem.Details[0].Code != DetailCodeRequired {
			t.Fatalf("unexpected details: %#v", problem.Details)
		}
	})

	t.Run("nil target is boundary error", func(t *testing.T) {
		if err := ValidateBody(nil); err == nil {
			t.Fatal("expected error")
		}
	})

	t.Run("invalid target is boundary error", func(t *testing.T) {
		err := ValidateBody(123)
		if err == nil {
			t.Fatal("expected error")
		}
		if _, ok := AsProblem(err); ok {
			t.Fatalf("expected plain error, got problem: %#v", err)
		}

		var payload *normalizedJSONPayload
		err = ValidateBody(payload)
		if err == nil {
			t.Fatal("expected nil pointer error")
		}
		if _, ok := AsProblem(err); ok {
			t.Fatalf("expected plain error, got problem: %#v", err)
		}
	})

}

func TestValidateQuery(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		status := int16(1)
		dto := queryDTO{
			Limit:        50,
			CategoryUUID: "550e8400-e29b-41d4-a716-446655440000",
			Status:       &status,
		}
		if err := ValidateQuery(&dto); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	t.Run("embedded struct", func(t *testing.T) {
		type pageQuery struct {
			Limit  *int `query:"limit" validate:"omitempty,min=1,max=100"`
			Offset *int `query:"offset" validate:"omitempty,min=0"`
		}
		type request struct {
			pageQuery
			Category string `query:"category" validate:"required"`
		}

		limit := 30
		offset := 12
		dto := request{
			pageQuery: pageQuery{
				Limit:  &limit,
				Offset: &offset,
			},
			Category: "book",
		}
		if err := ValidateQuery(&dto); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	t.Run("invalid uuid", func(t *testing.T) {
		err := ValidateQuery(&queryDTO{CategoryUUID: "bad"})
		problem, ok := AsProblem(err)
		if !ok {
			t.Fatalf("expected problem, got %T (%v)", err, err)
		}
		if len(problem.Details) != 1 || problem.Details[0].Field != "category_uuid" || problem.Details[0].Code != DetailCodeInvalidUUID {
			t.Fatalf("unexpected details: %#v", problem.Details)
		}
	})

	t.Run("out of range", func(t *testing.T) {
		err := ValidateQuery(&queryDTO{Limit: 101})
		problem, ok := AsProblem(err)
		if !ok {
			t.Fatalf("expected problem, got %T (%v)", err, err)
		}
		if len(problem.Details) != 1 || problem.Details[0].Field != "limit" || problem.Details[0].Code != DetailCodeOutOfRange {
			t.Fatalf("unexpected details: %#v", problem.Details)
		}
	})

	t.Run("invalid value", func(t *testing.T) {
		status := int16(8)
		err := ValidateQuery(&queryDTO{Status: &status})
		problem, ok := AsProblem(err)
		if !ok {
			t.Fatalf("expected problem, got %T (%v)", err, err)
		}
		if len(problem.Details) != 1 || problem.Details[0].Field != "status" || problem.Details[0].Code != DetailCodeInvalidValue {
			t.Fatalf("unexpected details: %#v", problem.Details)
		}
	})

	t.Run("uses query tag", func(t *testing.T) {
		type mixedQueryDTO struct {
			Value string `json:"body_name" query:"query_name" validate:"required"`
		}

		err := ValidateQuery(&mixedQueryDTO{})
		problem, ok := AsProblem(err)
		if !ok {
			t.Fatalf("expected problem, got %T (%v)", err, err)
		}
		if problem.StatusCode != http.StatusBadRequest {
			t.Fatalf("unexpected status: %d", problem.StatusCode)
		}
		if len(problem.Details) != 1 || problem.Details[0].Field != "query_name" || problem.Details[0].Code != DetailCodeRequired {
			t.Fatalf("unexpected details: %#v", problem.Details)
		}
	})

	t.Run("normalizes before validation", func(t *testing.T) {
		dto := normalizedQueryDTO{Keyword: " books "}
		if err := ValidateQuery(&dto); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if dto.Keyword != "books" {
			t.Fatalf("unexpected normalized query: %#v", dto)
		}
	})
}

func TestValidatePath(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		dto := pathDTO{UUID: "550e8400-e29b-41d4-a716-446655440000"}
		if err := ValidatePath(&dto); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	t.Run("required", func(t *testing.T) {
		err := ValidatePath(&pathDTO{})
		problem, ok := AsProblem(err)
		if !ok {
			t.Fatalf("expected problem, got %T (%v)", err, err)
		}
		if len(problem.Details) != 1 || problem.Details[0].Field != "uuid" || problem.Details[0].Code != DetailCodeRequired {
			t.Fatalf("unexpected details: %#v", problem.Details)
		}
	})

	t.Run("invalid uuid", func(t *testing.T) {
		err := ValidatePath(&pathDTO{UUID: "bad"})
		problem, ok := AsProblem(err)
		if !ok {
			t.Fatalf("expected problem, got %T (%v)", err, err)
		}
		if len(problem.Details) != 1 || problem.Details[0].Code != DetailCodeInvalidUUID {
			t.Fatalf("unexpected details: %#v", problem.Details)
		}
	})

	t.Run("uses param tag", func(t *testing.T) {
		type mixedPathDTO struct {
			Value string `json:"body_name" param:"path_name" validate:"required"`
		}

		err := ValidatePath(&mixedPathDTO{})
		problem, ok := AsProblem(err)
		if !ok {
			t.Fatalf("expected problem, got %T (%v)", err, err)
		}
		if problem.StatusCode != http.StatusBadRequest {
			t.Fatalf("unexpected status: %d", problem.StatusCode)
		}
		if len(problem.Details) != 1 || problem.Details[0].Field != "path_name" || problem.Details[0].Code != DetailCodeRequired {
			t.Fatalf("unexpected details: %#v", problem.Details)
		}
	})
}

func TestValidateStructForSource(t *testing.T) {
	t.Run("returns sorted details", func(t *testing.T) {
		details := validateStructForSource(validateLoginDTO{
			Phone:    "123",
			Password: "123",
		}, sourceJSON)

		if len(details) != 2 {
			t.Fatalf("expected 2 details, got %#v", details)
		}
		if details[0].In != InBody || details[0].Field != "password" || details[0].Code != DetailCodeOutOfRange {
			t.Fatalf("unexpected first detail: %#v", details[0])
		}
		if details[1].In != InBody || details[1].Field != "phone" || details[1].Code != DetailCodeOutOfRange {
			t.Fatalf("unexpected second detail: %#v", details[1])
		}
	})

	t.Run("invalid payload", func(t *testing.T) {
		details := validateStructForSource(123, sourceJSON)

		if len(details) != 1 {
			t.Fatalf("expected 1 detail, got %#v", details)
		}
		if details[0].In != InBody || details[0].Field != "body" || details[0].Code != DetailCodeInvalidValue {
			t.Fatalf("unexpected detail: %#v", details[0])
		}
	})

	t.Run("required code", func(t *testing.T) {
		type dto struct {
			Name string `json:"name" validate:"required"`
		}

		details := validateStructForSource(dto{}, sourceJSON)
		if len(details) != 1 {
			t.Fatalf("expected 1 detail, got %#v", details)
		}
		if details[0].Code != DetailCodeRequired {
			t.Fatalf("expected code %q, got %q", DetailCodeRequired, details[0].Code)
		}
	})

	t.Run("nested field uses inner tag", func(t *testing.T) {
		type item struct {
			Name string `json:"item_name" validate:"required"`
		}
		type dto struct {
			Items []item `json:"items" validate:"dive"`
		}

		details := validateStructForSource(dto{
			Items: []item{{}},
		}, sourceJSON)

		if len(details) != 1 {
			t.Fatalf("expected 1 detail, got %#v", details)
		}
		if details[0].Field != "item_name" || details[0].Code != DetailCodeRequired {
			t.Fatalf("unexpected detail: %#v", details[0])
		}
	})

	t.Run("validation codes", func(t *testing.T) {
		type dto struct {
			Phone string `json:"phone" validate:"numeric"`
			Code  string `json:"code" validate:"startswith=SKU-"`
			Role  string `json:"role" validate:"oneof=admin editor"`
			ID    string `json:"id" validate:"uuid"`
			Name  string `json:"name" validate:"max=2"`
		}

		details := validateStructForSource(dto{
			Phone: "12ab",
			Code:  "demo",
			Role:  "viewer",
			ID:    "bad",
			Name:  "long",
		}, sourceJSON)

		if len(details) != 5 {
			t.Fatalf("expected 5 details, got %#v", details)
		}

		got := map[string]Detail{}
		for _, detail := range details {
			got[detail.Field] = detail
		}

		if got["phone"].Code != DetailCodeInvalidValue {
			t.Fatalf("unexpected phone detail: %#v", got["phone"])
		}
		if got["code"].Code != DetailCodeInvalidValue {
			t.Fatalf("unexpected code detail: %#v", got["code"])
		}
		if got["role"].Code != DetailCodeInvalidValue {
			t.Fatalf("unexpected role detail: %#v", got["role"])
		}
		if got["id"].Code != DetailCodeInvalidUUID {
			t.Fatalf("unexpected id detail: %#v", got["id"])
		}
		if got["name"].Code != DetailCodeOutOfRange {
			t.Fatalf("unexpected name detail: %#v", got["name"])
		}
	})
}

func TestValidationHelpers(t *testing.T) {
	t.Run("problem detail helpers", func(t *testing.T) {
		if got := OutOfRange(InQuery, "limit"); got != (Detail{In: InQuery, Field: "limit", Code: DetailCodeOutOfRange}) {
			t.Fatalf("unexpected out_of_range detail: %#v", got)
		}
	})

	t.Run("source kind in", func(t *testing.T) {
		tests := []struct {
			source sourceKind
			want   string
		}{
			{source: sourceJSON, want: InBody},
			{source: sourceQuery, want: InQuery},
			{source: sourcePath, want: InPath},
			{source: sourceKind("other"), want: InBody},
		}

		for _, tt := range tests {
			if got := tt.source.in(); got != tt.want {
				t.Fatalf("source %q: want %q, got %q", tt.source, tt.want, got)
			}
		}
	})

	t.Run("tag value and source aliases", func(t *testing.T) {
		type dto struct {
			Name   string `json:"json_name,omitempty" query:"query_name" param:"path_name"`
			Hidden string `query:"-"`
			Plain  string
		}

		typ := reflect.TypeOf(dto{})
		nameField, _ := typ.FieldByName("Name")
		hiddenField, _ := typ.FieldByName("Hidden")
		plainField, _ := typ.FieldByName("Plain")

		if got := tagValue(nameField, "json"); got != "json_name" {
			t.Fatalf("unexpected tag value: %q", got)
		}
		if got := tagValue(hiddenField, "query"); got != "" {
			t.Fatalf("expected omitted tag, got %q", got)
		}
		if got := fieldAlias(nameField, sourceJSON); got != "json_name" {
			t.Fatalf("unexpected json alias: %q", got)
		}
		if got := fieldAlias(nameField, sourceQuery); got != "query_name" {
			t.Fatalf("unexpected query alias: %q", got)
		}
		if got := fieldAlias(nameField, sourcePath); got != "path_name" {
			t.Fatalf("unexpected path alias: %q", got)
		}
		if got := fieldAlias(plainField, sourceQuery); got != "Plain" {
			t.Fatalf("unexpected plain fallback: %q", got)
		}
	})
}
