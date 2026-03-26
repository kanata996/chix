package reqx

type Kind string

const (
	KindInvalidDestination Kind = "invalid_destination"
	KindMissingBody        Kind = "missing_body"
	KindInvalidBody        Kind = "invalid_body"
	KindMissingParameter   Kind = "missing_parameter"
	KindInvalidParameter   Kind = "invalid_parameter"
	KindValidation         Kind = "validation"
)

type Violation struct {
	Source  string `json:"source,omitempty"`
	Field   string `json:"field"`
	Rule    string `json:"rule"`
	Param   string `json:"param,omitempty"`
	Message string `json:"message,omitempty"`
}

type RequestError struct {
	Kind       Kind
	Message    string
	Source     string
	Name       string
	Violations []Violation
	Cause      error
}

func (e *RequestError) Error() string {
	if e == nil {
		return ""
	}
	return e.Message
}

func (e *RequestError) Unwrap() error {
	if e == nil {
		return nil
	}
	return e.Cause
}

func (e *RequestError) WithCause(err error) *RequestError {
	e.Cause = err
	return e
}

func (e *RequestError) WithViolations(violations []Violation) *RequestError {
	e.Violations = violations
	return e
}

func NewError(kind Kind, message string) *RequestError {
	return &RequestError{
		Kind:    kind,
		Message: message,
	}
}
