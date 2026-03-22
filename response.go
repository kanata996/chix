package chix

import (
	"github.com/kanata996/chix/internal/core"
	"net/http"
)

// Write writes a success envelope without meta.
func Write(w http.ResponseWriter, status int, data any) error {
	return core.WriteSuccess(w, status, data, nil, false)
}

// WriteMeta writes a success envelope with explicit meta.
func WriteMeta(w http.ResponseWriter, status int, data any, meta any) error {
	return core.WriteSuccess(w, status, data, meta, true)
}

// WriteEmpty writes a body-less successful response.
func WriteEmpty(w http.ResponseWriter, status int) error {
	return core.WriteEmpty(w, status)
}

// WriteError writes the standardized error envelope.
func WriteError(w http.ResponseWriter, r *http.Request, err error, opts ...Option) {
	cfg := buildConfig(opts...)
	writeErrorWithConfig(w, r, err, cfg)
}

func writeErrorWithConfig(w http.ResponseWriter, _ *http.Request, err error, cfg config) {
	if core.ResponseStarted(w) {
		return
	}

	mapped := mapError(err, cfg)
	core.WriteError(w, core.ErrorPayload{
		Status:  mapped.Status(),
		Code:    mapped.Code(),
		Message: mapped.Message(),
		Details: mapped.Details(),
	})
}
