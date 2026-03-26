package chix

import (
	"context"
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"
)

type Operation struct {
	Method             string
	Path               string
	OperationID        string
	Summary            string
	Description        string
	Tags               []string
	Middlewares        []func(http.Handler) http.Handler
	SuccessStatus      int
	SuccessDescription string
	Responses          []OperationResponse
}

type OperationResponse struct {
	Status      int
	Description string
	Headers     map[string]HeaderDoc
	NoBody      bool
}

type ResponseStatusProvider interface {
	ResponseStatus() int
}

type ResponseHeadersProvider interface {
	ResponseHeaders() http.Header
}

type Handler[In any, Out any] func(ctx context.Context, input *In) (*Out, error)

func Register[In any, Out any](app *App, operation Operation, handler Handler[In, Out]) {
	if app == nil {
		panic("chix: app must not be nil")
	}

	method := strings.ToUpper(operation.Method)
	if method == "" {
		panic("chix: operation method must not be empty")
	}
	if operation.Path == "" {
		panic("chix: operation path must not be empty")
	}
	for _, response := range operation.Responses {
		if response.Status <= 0 {
			panic("chix: operation response status must be positive")
		}
	}

	runtimeHandler := http.Handler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var input In
		if err := app.reqDecoder.Decode(r, &input); err != nil {
			writeError(w, r, requestDecodeError(err))
			return
		}

		output, err := handler(r.Context(), &input)
		if err != nil {
			writeError(w, r, err)
			return
		}

		status := successStatus(method, operation.SuccessStatus, operation.Responses)
		if output != nil {
			status = responseStatus(output, status)
			applyResponseHeaders(w.Header(), responseHeaders(output))
		}
		if status == http.StatusNoContent || output == nil {
			w.WriteHeader(status)
			return
		}

		if err := writeJSON(w, status, output); err != nil {
			writeError(w, r, err)
		}
	}))

	if len(operation.Middlewares) > 0 {
		runtimeHandler = chi.Chain(operation.Middlewares...).Handler(runtimeHandler)
	}

	app.registerOperation(method, operation.Path, runtimeHandler, newOperationDoc[In, Out](app.doc, operation))
}

func successStatus(method string, explicit int, responses []OperationResponse) int {
	if explicit > 0 {
		return explicit
	}
	explicitSuccessStatus := 0
	for _, response := range responses {
		if !isSuccessStatus(response.Status) {
			continue
		}
		if explicitSuccessStatus != 0 {
			explicitSuccessStatus = 0
			break
		}
		explicitSuccessStatus = response.Status
	}
	if explicitSuccessStatus > 0 {
		return explicitSuccessStatus
	}
	if method == http.MethodPost {
		return http.StatusCreated
	}
	return http.StatusOK
}

func isSuccessStatus(status int) bool {
	return status >= 200 && status < 300
}

func responseStatus(value any, fallback int) int {
	provider, ok := value.(ResponseStatusProvider)
	if !ok {
		return fallback
	}
	status := provider.ResponseStatus()
	if status <= 0 {
		return fallback
	}
	return status
}

func responseHeaders(value any) http.Header {
	provider, ok := value.(ResponseHeadersProvider)
	if !ok {
		return nil
	}
	return provider.ResponseHeaders()
}

func applyResponseHeaders(dst, src http.Header) {
	if len(src) == 0 {
		return
	}
	for name, values := range src {
		if len(values) == 0 {
			continue
		}
		dst.Set(name, values[0])
		for _, value := range values[1:] {
			dst.Add(name, value)
		}
	}
}
