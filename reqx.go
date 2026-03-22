package chix

import (
	"errors"
	"net/http"

	"github.com/kanata996/chix/reqx"
)

// DecodeOption customizes JSON decoding behavior.
type DecodeOption = reqx.DecodeOption

// QueryOption customizes URL query decoding behavior.
type QueryOption = reqx.QueryOption

// Violation describes a single request field validation problem.
type Violation = reqx.Violation

// ValidateFunc validates a decoded request value.
type ValidateFunc[T any] func(*T) []Violation

// WithMaxBodyBytes limits the number of bytes read from the request body.
func WithMaxBodyBytes(limit int64) DecodeOption {
	return reqx.WithMaxBodyBytes(limit)
}

// AllowUnknownFields disables strict unknown-field rejection for JSON decoding.
func AllowUnknownFields() DecodeOption {
	return reqx.AllowUnknownFields()
}

// AllowEmptyBody permits an empty JSON request body.
func AllowEmptyBody() DecodeOption {
	return reqx.AllowEmptyBody()
}

// AllowUnknownQueryFields disables strict unknown-field rejection for query
// parameters.
func AllowUnknownQueryFields() QueryOption {
	return reqx.AllowUnknownQueryFields()
}

// DecodeJSON decodes a JSON request body into dst and returns chix-compatible
// request errors for request-shape failures.
func DecodeJSON[T any](r *http.Request, dst *T, opts ...DecodeOption) error {
	return adaptReqxProblem(reqx.DecodeJSON(r, dst, opts...))
}

// DecodeAndValidateJSON decodes a JSON request body, then runs validation.
func DecodeAndValidateJSON[T any](r *http.Request, dst *T, fn ValidateFunc[T], opts ...DecodeOption) error {
	return adaptReqxProblem(reqx.DecodeAndValidateJSON(r, dst, reqx.ValidateFunc[T](fn), opts...))
}

// DecodeQuery decodes URL query parameters into `query`-tagged struct fields in
// dst and returns chix-compatible request errors for request-shape failures.
func DecodeQuery[T any](r *http.Request, dst *T, opts ...QueryOption) error {
	return adaptReqxProblem(reqx.DecodeQuery(r, dst, opts...))
}

// DecodeAndValidateQuery decodes URL query parameters, then runs validation.
func DecodeAndValidateQuery[T any](r *http.Request, dst *T, fn ValidateFunc[T], opts ...QueryOption) error {
	return adaptReqxProblem(reqx.DecodeAndValidateQuery(r, dst, reqx.ValidateFunc[T](fn), opts...))
}

// Validate applies a validation function and returns a standardized 422 request
// error when violations are present.
func Validate[T any](dst *T, fn ValidateFunc[T]) error {
	return adaptReqxProblem(reqx.Validate(dst, reqx.ValidateFunc[T](fn)))
}

func adaptReqxProblem(err error) error {
	if err == nil {
		return nil
	}

	var problem *reqx.Problem
	if errors.As(err, &problem) && problem != nil {
		return RequestError(problem.Status(), problem.Code(), problem.Message(), problem.Details()...)
	}

	return err
}
