package reqx

import (
	"errors"
	"net/http"
	"testing"
)

func TestProblemCopiesDetails(t *testing.T) {
	details := []Detail{Required(InBody, "phone")}
	problem := ValidationFailed(details...)

	details[0].Field = "changed"
	if problem.Details[0].Field != "phone" {
		t.Fatalf("expected copied details, got %#v", problem.Details)
	}
}

func TestAsProblem(t *testing.T) {
	problem, ok := AsProblem(BadRequest(Required(InQuery, "limit")))
	if !ok {
		t.Fatal("expected problem")
	}
	if problem.StatusCode != http.StatusBadRequest {
		t.Fatalf("unexpected status: %d", problem.StatusCode)
	}
}

func TestAsProblem_NoMatch(t *testing.T) {
	if _, ok := AsProblem(errors.New("plain error")); ok {
		t.Fatal("expected no problem match")
	}
}

func TestProblemError(t *testing.T) {
	t.Run("nil receiver", func(t *testing.T) {
		var problem *Problem
		if got := problem.Error(); got != "request problem" {
			t.Fatalf("unexpected error string: %q", got)
		}
	})

	t.Run("validation message", func(t *testing.T) {
		problem := &Problem{StatusCode: http.StatusUnprocessableEntity}
		if got := problem.Error(); got != "Validation Failed" {
			t.Fatalf("unexpected error string: %q", got)
		}
	})

	t.Run("status text", func(t *testing.T) {
		problem := &Problem{StatusCode: http.StatusBadRequest}
		if got := problem.Error(); got != "Bad Request" {
			t.Fatalf("unexpected error string: %q", got)
		}
	})

	t.Run("fallback", func(t *testing.T) {
		problem := &Problem{}
		if got := problem.Error(); got != "request problem" {
			t.Fatalf("unexpected error string: %q", got)
		}
	})
}

func TestProblemConstructors(t *testing.T) {
	t.Run("unsupported media type adds default detail", func(t *testing.T) {
		problem := UnsupportedMediaType()
		if problem.StatusCode != http.StatusUnsupportedMediaType {
			t.Fatalf("unexpected status: %d", problem.StatusCode)
		}
		if len(problem.Details) != 1 || problem.Details[0].Code != DetailCodeUnsupportedMediaType {
			t.Fatalf("unexpected details: %#v", problem.Details)
		}
	})

	t.Run("payload too large adds default detail", func(t *testing.T) {
		problem := PayloadTooLarge()
		if problem.StatusCode != http.StatusRequestEntityTooLarge {
			t.Fatalf("unexpected status: %d", problem.StatusCode)
		}
		if len(problem.Details) != 1 || problem.Details[0].Code != DetailCodePayloadTooLarge {
			t.Fatalf("unexpected details: %#v", problem.Details)
		}
	})

	t.Run("multiple values helper", func(t *testing.T) {
		detail := MultipleValues(InQuery, "limit")
		if detail.In != InQuery || detail.Field != "limit" || detail.Code != DetailCodeMultipleValues {
			t.Fatalf("unexpected detail: %#v", detail)
		}
	})

	t.Run("remaining detail helpers", func(t *testing.T) {
		tests := []Detail{
			InvalidUUID(InPath, "uuid"),
			InvalidInteger(InQuery, "limit"),
			InvalidValue(InBody, "name"),
		}

		want := []Detail{
			{In: InPath, Field: "uuid", Code: DetailCodeInvalidUUID},
			{In: InQuery, Field: "limit", Code: DetailCodeInvalidInteger},
			{In: InBody, Field: "name", Code: DetailCodeInvalidValue},
		}

		for i := range tests {
			if tests[i] != want[i] {
				t.Fatalf("unexpected detail at %d: %#v", i, tests[i])
			}
		}
	})
}
