package errx

import (
	"errors"
)

const (
	CodeInvalidRequest       int64 = 400000
	CodeUnauthorized         int64 = 400001
	CodeForbidden            int64 = 400003
	CodeNotFound             int64 = 400004
	CodeConflict             int64 = 400009
	CodePayloadTooLarge      int64 = 400013
	CodeUnsupportedMediaType int64 = 400015
	CodeUnprocessableEntity  int64 = 400022
	CodeTooManyRequests      int64 = 400029
	CodeClientClosed         int64 = 499000
	CodeInternal             int64 = 500000
	CodeServiceUnavailable   int64 = 500003
	CodeTimeout              int64 = 500004
)

var (
	ErrInvalidRequest      = errors.New("invalid request")
	ErrUnauthorized        = errors.New("unauthorized")
	ErrForbidden           = errors.New("forbidden")
	ErrNotFound            = errors.New("not found")
	ErrConflict            = errors.New("conflict")
	ErrUnprocessableEntity = errors.New("unprocessable entity")
	ErrTooManyRequests     = errors.New("too many requests")
	ErrServiceUnavailable  = errors.New("service unavailable")
	ErrTimeout             = errors.New("timeout")
)

type Mapping struct {
	StatusCode int
	Code       int64
	Message    string
}
