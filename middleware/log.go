package middleware

import (
	"context"
	"io"
	"log/slog"
	"net/http"
	"os"
	"reflect"
	"strings"

	"github.com/go-chi/chi/v5"
	chimw "github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/httplog/v3"
	"github.com/go-chi/traceid"
	"github.com/golang-cz/devslog"
)

type ctxKeyRequestLogger struct{}
type ctxKeyBaseRequestLogAttrs struct{}

var (
	defaultRequestLogHeaders  = []string{"Content-Type", "Origin"}
	defaultResponseLogHeaders = []string{"Content-Type"}
)

// LoggerOptions controls the slog logger created by NewLogger.
type LoggerOptions struct {
	Output      io.Writer
	Development bool
	Level       slog.Level
	App         string
	Version     string
	Env         string
}

// NewLogger builds a slog logger suitable for RequestLogger middleware usage.
func NewLogger(opts LoggerOptions) *slog.Logger {
	output := opts.Output
	if output == nil {
		output = os.Stdout
	}

	logFormat := httplog.SchemaECS.Concise(opts.Development)
	handlerOpts := &slog.HandlerOptions{
		AddSource:   opts.Development,
		Level:       opts.Level,
		ReplaceAttr: logFormat.ReplaceAttr,
	}

	return slog.New(newHandler(output, opts.Development, handlerOpts)).With(loggerAttrs(opts)...)
}

// RequestLogger returns a chi middleware that standardizes request logging,
// request ids, trace ids, and panic recovery.
func RequestLogger(logger *slog.Logger, level slog.Level) func(http.Handler) http.Handler {
	if logger == nil {
		logger = slog.Default()
	}
	logger = ensureTraceIDLogger(logger)

	var chain chi.Middlewares
	chain = append(chain, chimw.RequestID, traceid.Middleware)
	chain = append(chain,
		httplog.RequestLogger(logger, &httplog.Options{
			Level:              level,
			Schema:             httplog.SchemaECS,
			RecoverPanics:      true,
			LogRequestHeaders:  defaultRequestLogHeaders,
			LogResponseHeaders: defaultResponseLogHeaders,
		}),
		requestLogContextMiddleware(logger),
	)

	return func(next http.Handler) http.Handler {
		return chain.Handler(next)
	}
}

// LoggerFromContext returns the request logger configured by RequestLogger.
// It returns nil when the context did not come through that middleware chain.
func LoggerFromContext(ctx context.Context) *slog.Logger {
	if ctx == nil {
		return nil
	}
	logger, _ := ctx.Value(ctxKeyRequestLogger{}).(*slog.Logger)
	return logger
}

// BaseRequestLogAttrs returns stable request log fields shared across success,
// panic, and error paths.
func BaseRequestLogAttrs(r *http.Request) []slog.Attr {
	if r == nil {
		return nil
	}

	attrs := make([]slog.Attr, 0, 2)
	if requestID := chimw.GetReqID(r.Context()); requestID != "" {
		attrs = append(attrs, slog.String("request.id", requestID))
	}

	rctx := chi.RouteContext(r.Context())
	if rctx != nil {
		if route := strings.TrimSpace(rctx.RoutePattern()); route != "" {
			attrs = append(attrs, slog.String("http.route", route))
		}
	}

	return attrs
}

// HasBaseRequestLogAttrs reports whether the request context came through the
// composed request logging middleware that is responsible for injecting base
// request attrs into the request log.
func HasBaseRequestLogAttrs(ctx context.Context) bool {
	if ctx == nil {
		return false
	}
	applied, _ := ctx.Value(ctxKeyBaseRequestLogAttrs{}).(bool)
	return applied
}

func requestLogContextMiddleware(logger *slog.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ctx := context.WithValue(r.Context(), ctxKeyRequestLogger{}, logger)
			ctx = context.WithValue(ctx, ctxKeyBaseRequestLogAttrs{}, true)
			r = r.WithContext(ctx)

			defer func() {
				attrs := BaseRequestLogAttrs(r)
				if len(attrs) == 0 {
					return
				}
				httplog.SetAttrs(r.Context(), attrs...)
			}()

			next.ServeHTTP(w, r)
		})
	}
}

func loggerAttrs(opts LoggerOptions) []any {
	attrs := make([]any, 0, 3)
	if opts.App != "" {
		attrs = append(attrs, slog.String("app", opts.App))
	}
	if opts.Version != "" {
		attrs = append(attrs, slog.String("version", opts.Version))
	}
	if opts.Env != "" {
		attrs = append(attrs, slog.String("env", opts.Env))
	}
	return attrs
}

func newHandler(output io.Writer, development bool, handlerOpts *slog.HandlerOptions) slog.Handler {
	var handler slog.Handler
	if development {
		handler = devslog.NewHandler(output, &devslog.Options{
			SortKeys:           true,
			MaxErrorStackTrace: 5,
			MaxSlicePrintSize:  20,
			HandlerOptions:     handlerOpts,
		})
	} else {
		handler = slog.NewJSONHandler(output, handlerOpts)
	}

	return traceid.LogHandler(handler)
}

func ensureTraceIDLogger(logger *slog.Logger) *slog.Logger {
	if logger == nil {
		return nil
	}

	handler := logger.Handler()
	if handler == nil || handlerSupportsTraceID(handler) {
		return logger
	}

	return slog.New(traceid.LogHandler(handler))
}

func handlerSupportsTraceID(handler slog.Handler) bool {
	handlerType := reflect.TypeOf(handler)
	if handlerType == nil {
		return false
	}
	if handlerType.Kind() == reflect.Pointer {
		handlerType = handlerType.Elem()
	}

	return handlerType.PkgPath() == "github.com/go-chi/traceid" && handlerType.Name() == "logHandler"
}
