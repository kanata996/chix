package chix

import (
	"context"
	"log/slog"
	"net/http"

	"github.com/go-chi/httplog/v3"
	"github.com/go-chi/traceid"
	"github.com/kanata996/chix/resp"
)

// ErrorResponder is the chi-oriented preset alias of resp.ErrorResponder.
type ErrorResponder = resp.ErrorResponder

var defaultErrorResponder = NewErrorResponder()

// NewErrorResponder returns a responder preconfigured for chi + httplog style
// services. Callers may mutate the returned value to customize strategy while
// preserving the package default behavior as a baseline.
func NewErrorResponder() *ErrorResponder {
	return &resp.ErrorResponder{
		ContextAttrs: chixErrorContextAttrs,
		AnnotateRequestLog: func(r *http.Request, attrs []slog.Attr) {
			if r == nil || len(attrs) == 0 {
				return
			}
			httplog.SetAttrs(r.Context(), attrs...)
		},
	}
}

func chixErrorContextAttrs(ctx context.Context) []slog.Attr {
	if ctx == nil {
		return nil
	}

	attrs := make([]slog.Attr, 0, 1)
	if traceID := traceid.FromContext(ctx); traceID != "" {
		attrs = append(attrs, slog.String(traceid.LogKey, traceID))
	}

	return attrs
}
