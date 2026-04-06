package middleware

import (
	"context"
	"log/slog"
	"net/http"

	chimw "github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/httplog/v3"
	"github.com/go-chi/traceid"
)

// RequestLogAttrs appends request correlation attrs to the current httplog
// access log. Mount it after RequestID, traceid.Middleware, and
// httplog.RequestLogger.
func RequestLogAttrs() func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			next.ServeHTTP(w, r)

			attrs := requestContextAttrs(r.Context())
			if len(attrs) > 0 {
				httplog.SetAttrs(r.Context(), attrs...)
			}
		})
	}
}

func requestContextAttrs(ctx context.Context) []slog.Attr {
	if ctx == nil {
		return nil
	}

	attrs := make([]slog.Attr, 0, 3)
	if traceID := traceid.FromContext(ctx); traceID != "" {
		attrs = append(attrs, slog.String(traceid.LogKey, traceID))
	}
	if requestID := chimw.GetReqID(ctx); requestID != "" {
		attrs = append(attrs, slog.String("request.id", requestID))
	}

	return attrs
}
