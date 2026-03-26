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

		status := successStatus(method, operation.SuccessStatus)
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

func successStatus(method string, explicit int) int {
	if explicit > 0 {
		return explicit
	}
	if method == http.MethodPost {
		return http.StatusCreated
	}
	return http.StatusOK
}
