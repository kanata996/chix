package chix

import (
	"errors"
	"github.com/kanata996/chix/internal/core"
	"net/http"
)

// Handler is the error-returning handler form that Wrap adapts to net/http.
type Handler func(w http.ResponseWriter, r *http.Request) error

// Wrap adapts an error-returning handler into a standard http.HandlerFunc.
func Wrap(h Handler, opts ...Option) http.HandlerFunc {
	cfg := buildConfig(opts...)

	return func(w http.ResponseWriter, r *http.Request) {
		tw := core.NewTrackingResponseWriter(w)

		if h == nil {
			if !core.ResponseStarted(tw) {
				writeErrorWithConfig(tw, r, errors.New("chix: nil handler"), cfg)
			}
			return
		}

		if err := h(tw, r); err != nil {
			if core.ResponseStarted(tw) {
				return
			}
			writeErrorWithConfig(tw, r, err, cfg)
		}
	}
}
