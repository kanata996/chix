package chix

import (
	"errors"
	"net/http"

	"github.com/kanata996/chix/reqx"
)

// ErrorKind is the boundary-level classification of an exported HTTP error.
type ErrorKind string

const (
	KindRequest  ErrorKind = "request"
	KindDomain   ErrorKind = "domain"
	KindInternal ErrorKind = "internal"
)

// Error is the standardized public error representation used at the HTTP
// boundary.
type Error struct {
	kind    ErrorKind
	status  int
	code    string
	message string
	details []any
}

// ErrorMapper maps an application error into a standardized boundary error.
type ErrorMapper func(err error) *Error

// RequestError constructs a request-level HTTP error.
func RequestError(status int, code, message string, details ...any) *Error {
	return newError(KindRequest, status, code, message, details)
}

// DomainError constructs a domain-level HTTP error.
func DomainError(status int, code, message string, details ...any) *Error {
	return newError(KindDomain, status, code, message, details)
}

// InternalError constructs an internal HTTP error.
func InternalError(status int, code, message string, details ...any) *Error {
	return newError(KindInternal, status, code, message, details)
}

func newError(kind ErrorKind, status int, code, message string, details []any) *Error {
	return &Error{
		kind:    kind,
		status:  normalizeStatus(kind, status),
		code:    normalizeCode(kind, code),
		message: normalizeMessage(kind, message),
		details: cloneDetails(details),
	}
}

func normalizeStatus(kind ErrorKind, status int) int {
	switch kind {
	case KindRequest:
		if status < 400 || status > 499 {
			return http.StatusBadRequest
		}
	case KindDomain:
		if status < 400 || status > 499 {
			return http.StatusUnprocessableEntity
		}
	case KindInternal:
		if status < 500 || status > 599 {
			return http.StatusInternalServerError
		}
	default:
		return http.StatusInternalServerError
	}

	return status
}

func normalizeCode(kind ErrorKind, code string) string {
	if code != "" {
		return code
	}

	switch kind {
	case KindRequest:
		return "request_error"
	case KindDomain:
		return "domain_error"
	default:
		return "internal_error"
	}
}

func normalizeMessage(kind ErrorKind, message string) string {
	if message != "" {
		return message
	}

	switch kind {
	case KindRequest:
		return "request error"
	case KindDomain:
		return "domain error"
	default:
		return "internal server error"
	}
}

func cloneDetails(details []any) []any {
	if len(details) == 0 {
		return nil
	}

	cloned := make([]any, len(details))
	copy(cloned, details)
	return cloned
}

// Error implements the error interface.
func (e *Error) Error() string {
	if e == nil {
		return ""
	}
	return e.message
}

// Kind returns the boundary-level classification.
func (e *Error) Kind() ErrorKind {
	if e == nil {
		return KindInternal
	}
	return e.kind
}

// Status returns the public HTTP status code.
func (e *Error) Status() int {
	if e == nil {
		return http.StatusInternalServerError
	}
	return e.status
}

// Code returns the stable machine-readable error code.
func (e *Error) Code() string {
	if e == nil {
		return normalizeCode(KindInternal, "")
	}
	return e.code
}

// Message returns the safe public error message.
func (e *Error) Message() string {
	if e == nil {
		return normalizeMessage(KindInternal, "")
	}
	return e.message
}

// Details returns a copy of the structured error details.
func (e *Error) Details() []any {
	if e == nil || len(e.details) == 0 {
		return nil
	}
	return cloneDetails(e.details)
}

func mapError(err error, cfg config) *Error {
	if err == nil {
		return defaultInternalError()
	}

	var boundaryErr *Error
	if errors.As(err, &boundaryErr) && boundaryErr != nil {
		return boundaryErr
	}

	var problem *reqx.Problem
	if errors.As(err, &problem) && problem != nil {
		return RequestError(problem.Status(), problem.Code(), problem.Message(), problem.Details()...)
	}

	for _, mapper := range cfg.mappers {
		if mapper == nil {
			continue
		}
		if mapped := mapper(err); mapped != nil {
			return mapped
		}
	}

	return defaultInternalError()
}

func defaultInternalError() *Error {
	return InternalError(
		http.StatusInternalServerError,
		"internal_error",
		"internal server error",
	)
}
