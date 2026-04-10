package chix

import (
	"net/http"
)

// WriteError writes a structured error response using the default chi-oriented
// responder preset.
func WriteError(w http.ResponseWriter, r *http.Request, err error) error {
	return defaultErrorResponder.Respond(w, r, err)
}
