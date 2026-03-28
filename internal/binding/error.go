package binding

import "errors"

type ErrorKind uint8

const (
	ErrorKindUnknown ErrorKind = iota
	ErrorKindRequestShape
	ErrorKindUnsupportedMediaType
)

type bindError struct {
	kind  ErrorKind
	cause error
}

func (e *bindError) Error() string {
	if e == nil {
		return ""
	}
	if e.cause != nil {
		return e.cause.Error()
	}

	switch e.kind {
	case ErrorKindRequestShape:
		return "bad request"
	case ErrorKindUnsupportedMediaType:
		return "unsupported media type"
	default:
		return "binding error"
	}
}

func (e *bindError) Unwrap() error {
	if e == nil {
		return nil
	}
	return e.cause
}

func KindOf(err error) ErrorKind {
	var bindErr *bindError
	if errors.As(err, &bindErr) {
		return bindErr.kind
	}
	return ErrorKindUnknown
}

func newRequestShapeError(cause error) error {
	return &bindError{
		kind:  ErrorKindRequestShape,
		cause: cause,
	}
}

func newUnsupportedMediaTypeError(cause error) error {
	return &bindError{
		kind:  ErrorKindUnsupportedMediaType,
		cause: cause,
	}
}
