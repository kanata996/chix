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

type ctxKeyRequestLogContext struct{}

type requestLogContext struct {
	logger          *slog.Logger
	hasBaseLogAttrs bool
}

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

// RequestLogger returns the supported request-logging middleware for chix. It
// is an opinionated chi middleware that standardizes request logging, request
// ids, trace ids, and panic recovery.
//
// It composes chi's RequestID, traceid.Middleware, and httplog.RequestLogger
// with RecoverPanics enabled, so callers should not mount chi's Logger,
// chi's Recoverer, traceid.Middleware, chi's RequestID, or another
// httplog.RequestLogger on the same route chain. chix does not define a
// separate supported mode where callers assemble that chain themselves.
//
// For fully ECS-aligned output, prefer NewLogger or another slog.Logger whose
// handler uses httplog.SchemaECS.ReplaceAttr.
func RequestLogger(logger *slog.Logger, level slog.Level) func(http.Handler) http.Handler {
	if logger == nil {
		logger = slog.Default()
	}
	logger = ensureTraceIDLogger(logger)

	return chi.Chain(
		chimw.RequestID,
		traceid.Middleware,
		httplog.RequestLogger(logger, &httplog.Options{
			Level:              level,
			Schema:             httplog.SchemaECS,
			RecoverPanics:      true,
			LogRequestHeaders:  defaultRequestLogHeaders,
			LogResponseHeaders: defaultResponseLogHeaders,
		}),
		requestLogContextMiddleware(logger),
	).Handler
}

// LoggerFromContext returns the request logger configured by RequestLogger.
// It returns nil when the context did not come through that middleware chain.
func LoggerFromContext(ctx context.Context) *slog.Logger {
	return requestLogContextFrom(ctx).logger
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
	return requestLogContextFrom(ctx).hasBaseLogAttrs
}

func requestLogContextMiddleware(logger *slog.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ctx := context.WithValue(r.Context(), ctxKeyRequestLogContext{}, requestLogContext{
				logger:          logger,
				hasBaseLogAttrs: true,
			})
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

func requestLogContextFrom(ctx context.Context) requestLogContext {
	if ctx == nil {
		return requestLogContext{}
	}

	value, _ := ctx.Value(ctxKeyRequestLogContext{}).(requestLogContext)
	return value
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
