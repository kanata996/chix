package middleware

import (
	"context"
	"log/slog"
	"net/http"

	chimw "github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/httplog/v3"
	"github.com/go-chi/traceid"
)

// RequestLogAttrs 将请求关联属性追加到当前 httplog 访问日志中。
// 应挂载在 RequestID、traceid.Middleware 和 httplog.RequestLogger 之后。
func RequestLogAttrs() func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			attrs := requestContextAttrs(r.Context())
			if len(attrs) > 0 {
				httplog.SetAttrs(r.Context(), attrs...)
			}

			next.ServeHTTP(w, r)
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
