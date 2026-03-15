package resp

import (
	"context"
	"encoding/json"
	"errors"
	"github.com/kanata996/chix/errx"
	"github.com/kanata996/chix/reqx"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestProblem_WritesReqxProblem(t *testing.T) {
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/", nil)

	Problem(rec, req, &reqx.Problem{
		StatusCode: http.StatusBadRequest,
		Details:    []reqx.Detail{reqx.Required(reqx.InQuery, "limit")},
	})

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status mismatch: want %d, got %d", http.StatusBadRequest, rec.Code)
	}

	var body envelope
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if body.Code != errx.CodeInvalidRequest || body.Message != http.StatusText(http.StatusBadRequest) {
		t.Fatalf("unexpected body: %#v", body)
	}
	if len(body.Details) != 1 || body.Details[0].Field != "limit" {
		t.Fatalf("unexpected details: %#v", body.Details)
	}
}

func TestProblem_InvalidReqxProblemFallsBackInternal(t *testing.T) {
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/", nil)

	Problem(rec, req, &reqx.Problem{StatusCode: http.StatusConflict})

	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("status mismatch: want %d, got %d", http.StatusInternalServerError, rec.Code)
	}

	var body envelope
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if body.Code != errx.CodeInternal || body.Message != http.StatusText(http.StatusInternalServerError) {
		t.Fatalf("unexpected body: %#v", body)
	}
}

func TestProblem_PassesThroughDetails(t *testing.T) {
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/", nil)

	Problem(rec, req, &reqx.Problem{
		StatusCode: http.StatusBadRequest,
		Details: []reqx.Detail{
			reqx.Required(reqx.InQuery, "limit"),
			reqx.OutOfRange(reqx.InQuery, "limit"),
		},
	})

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status mismatch: want %d, got %d", http.StatusBadRequest, rec.Code)
	}
	var body envelope
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if len(body.Details) != 2 {
		t.Fatalf("unexpected details: %#v", body.Details)
	}
}

func TestProblem_UnprocessableEntityUsesReqxMessage(t *testing.T) {
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/", nil)

	Problem(rec, req, &reqx.Problem{
		StatusCode: http.StatusUnprocessableEntity,
		Details:    []reqx.Detail{reqx.Required(reqx.InBody, "phone")},
	})

	if rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("status mismatch: want %d, got %d", http.StatusUnprocessableEntity, rec.Code)
	}

	var body envelope
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if body.Code != errx.CodeUnprocessableEntity || body.Message != "Validation Failed" {
		t.Fatalf("unexpected body: %#v", body)
	}
	if len(body.Details) != 1 || body.Details[0].Field != "phone" {
		t.Fatalf("unexpected details: %#v", body.Details)
	}
}

func TestProblem_UnexpectedNonProblemFailsClosed(t *testing.T) {
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/", nil)

	Problem(rec, req, errors.New("boom"))

	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("status mismatch: want %d, got %d", http.StatusInternalServerError, rec.Code)
	}
}

func TestProblem_UsesRequestContextForLogs(t *testing.T) {
	prev := slog.Default()
	defer slog.SetDefault(prev)

	counter := &countingHandler{}
	slog.SetDefault(slog.New(counter))

	type contextKey string

	ctx := context.WithValue(context.Background(), contextKey("request_id"), "req-123")
	req := httptest.NewRequest(http.MethodGet, "/", nil).WithContext(ctx)
	rec := httptest.NewRecorder()

	Problem(rec, req, &reqx.Problem{StatusCode: http.StatusConflict})

	if counter.Count() != 1 {
		t.Fatalf("expected one error log, got %d", counter.Count())
	}
	if got := counter.LastContext().Value(contextKey("request_id")); got != "req-123" {
		t.Fatalf("request context not forwarded to logger: got %#v", got)
	}
}

func TestProblem_InvalidReqxProblemLogsStatus(t *testing.T) {
	prev := slog.Default()
	defer slog.SetDefault(prev)

	counter := &countingHandler{}
	slog.SetDefault(slog.New(counter))

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/", nil)

	Problem(rec, req, &reqx.Problem{
		StatusCode: http.StatusConflict,
	})

	if counter.Count() != 1 {
		t.Fatalf("expected one error log, got %d", counter.Count())
	}
	switch got := counter.LastAttr("status_code").(type) {
	case int:
		if got != http.StatusConflict {
			t.Fatalf("expected status_code attr, got %#v", got)
		}
	case int64:
		if got != int64(http.StatusConflict) {
			t.Fatalf("expected status_code attr, got %#v", got)
		}
	default:
		t.Fatalf("unexpected status_code attr type: %T (%#v)", got, got)
	}
}
