package middleware

import (
	"log"
	"net/http"
	"os"
	"runtime/debug"
	"strings"

	"github.com/kanata996/chix"
	"github.com/kanata996/chix/internal/core"
)

var recovererLogger = log.New(os.Stderr, "", log.LstdFlags)
var requestIDHeader = "X-Request-Id"

type config struct {
	reporter PanicReporter
}

// Option customizes middleware.Recoverer behavior.
type Option func(*config)

// PanicReport is the structured panic observation emitted by Recoverer.
type PanicReport struct {
	Request         *http.Request
	Panic           any
	Stack           []byte
	ResponseStarted bool
	Upgrade         bool
}

// PanicReporter receives panic information captured by Recoverer.
type PanicReporter func(PanicReport)

// Recoverer recovers panics from the wrapped handler and, when possible,
// writes the standardized internal error envelope.
func Recoverer(opts ...Option) func(http.Handler) http.Handler {
	cfg := config{
		reporter: defaultPanicReporter,
	}
	for _, opt := range opts {
		if opt != nil {
			opt(&cfg)
		}
	}
	internalErr := chix.InternalError(
		http.StatusInternalServerError,
		"internal_error",
		"internal server error",
	)

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			tw := core.NewTrackingResponseWriter(w)

			defer func() {
				if rec := recover(); rec != nil {
					if rec == http.ErrAbortHandler {
						panic(rec)
					}
					started := core.ResponseStarted(tw)
					upgrade := isUpgradeRequest(r)
					if cfg.reporter != nil {
						cfg.reporter(PanicReport{
							Request:         r,
							Panic:           rec,
							Stack:           debug.Stack(),
							ResponseStarted: started,
							Upgrade:         upgrade,
						})
					}
					if started {
						return
					}
					if upgrade {
						return
					}
					chix.WriteError(tw, r, internalErr)
				}
			}()

			if next == nil {
				if !core.ResponseStarted(tw) {
					chix.WriteError(tw, r, internalErr)
				}
				return
			}

			next.ServeHTTP(tw, r)
		})
	}
}

func defaultPanicReporter(report PanicReport) {
	method := ""
	target := ""
	remoteAddr := ""
	requestID := ""
	if report.Request != nil {
		method = report.Request.Method
		if report.Request.URL != nil {
			target = report.Request.URL.RequestURI()
		}
		remoteAddr = report.Request.RemoteAddr
		requestID = strings.TrimSpace(report.Request.Header.Get(requestIDHeader))
	}
	recovererLogger.Printf(
		"chix/middleware: panic recovered: %v method=%s target=%s remote=%s request_id=%s started=%t upgrade=%t\n%s",
		report.Panic,
		method,
		target,
		remoteAddr,
		requestID,
		report.ResponseStarted,
		report.Upgrade,
		report.Stack,
	)
}

// WithPanicReporter replaces the default panic reporter. Passing nil disables
// panic reporting.
func WithPanicReporter(reporter PanicReporter) Option {
	return func(cfg *config) {
		cfg.reporter = reporter
	}
}

func isUpgradeRequest(r *http.Request) bool {
	if r == nil {
		return false
	}

	// Treat the request as an upgrade only when the client is explicitly
	// attempting an HTTP/1.1 protocol switch. A bare Connection: upgrade token
	// is not enough; otherwise ordinary requests can be misclassified and skip
	// the fail-closed JSON 500 path.
	if !headerHasToken(r.Header.Get("Connection"), "upgrade") {
		return false
	}

	return strings.TrimSpace(r.Header.Get("Upgrade")) != ""
}

func headerHasToken(value string, token string) bool {
	for _, part := range strings.Split(value, ",") {
		if strings.EqualFold(strings.TrimSpace(part), token) {
			return true
		}
	}
	return false
}
