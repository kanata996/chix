package chix

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"

	"github.com/go-chi/chi/v5/middleware"

	"github.com/kanata996/chix/reqx"
)

type Problem struct {
	Status     int              `json:"status"`
	Title      string           `json:"title"`
	Detail     string           `json:"detail,omitempty"`
	RequestID  string           `json:"requestID,omitempty"`
	Violations []reqx.Violation `json:"violations,omitempty"`
}

type HTTPError struct {
	Status     int
	Title      string
	Detail     string
	Headers    http.Header
	Violations []reqx.Violation
	Cause      error
}

func (e *HTTPError) Error() string {
	if e == nil {
		return ""
	}
	if e.Detail != "" {
		return e.Detail
	}
	return http.StatusText(e.Status)
}

func (e *HTTPError) Unwrap() error {
	if e == nil {
		return nil
	}
	return e.Cause
}

func (e *HTTPError) WithCause(err error) *HTTPError {
	e.Cause = err
	return e
}

func (e *HTTPError) WithViolations(violations []reqx.Violation) *HTTPError {
	e.Violations = violations
	return e
}

func (e *HTTPError) WithHeader(name, value string) *HTTPError {
	if e.Headers == nil {
		e.Headers = http.Header{}
	}
	e.Headers.Add(name, value)
	return e
}

func (e *HTTPError) WithHeaders(headers http.Header) *HTTPError {
	if e.Headers == nil {
		e.Headers = http.Header{}
	}
	applyResponseHeaders(e.Headers, headers)
	return e
}

func (e *HTTPError) ResponseHeaders() http.Header {
	if e == nil {
		return nil
	}
	return e.Headers
}

func StatusError(status int, detail string) *HTTPError {
	return &HTTPError{
		Status: status,
		Title:  http.StatusText(status),
		Detail: detail,
	}
}

func writeJSON(w http.ResponseWriter, status int, value any) error {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	return json.NewEncoder(w).Encode(value)
}

func writeError(w http.ResponseWriter, r *http.Request, err error) {
	var httpErr *HTTPError
	if !errors.As(err, &httpErr) {
		httpErr = StatusError(http.StatusInternalServerError, http.StatusText(http.StatusInternalServerError)).WithCause(err)
	}
	var headersProvider ResponseHeadersProvider
	if errors.As(err, &headersProvider) {
		applyResponseHeaders(w.Header(), headersProvider.ResponseHeaders())
	}

	problem := Problem{
		Status:     httpErr.Status,
		Title:      titleOrStatus(httpErr.Title, httpErr.Status),
		Detail:     httpErr.Detail,
		RequestID:  middleware.GetReqID(r.Context()),
		Violations: httpErr.Violations,
	}

	w.Header().Set("Content-Type", "application/problem+json; charset=utf-8")
	w.WriteHeader(httpErr.Status)
	if encodeErr := json.NewEncoder(w).Encode(problem); encodeErr != nil {
		http.Error(w, fmt.Sprintf(`{"status":500,"title":"%s"}`, http.StatusText(http.StatusInternalServerError)), http.StatusInternalServerError)
	}
}

func titleOrStatus(title string, status int) string {
	if title != "" {
		return title
	}
	return http.StatusText(status)
}
