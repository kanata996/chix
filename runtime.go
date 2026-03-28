package chix

import (
	"context"
	"log/slog"
	"net/http"
	"reflect"
	"strings"

	"github.com/go-chi/chi/v5/middleware"

	"github.com/kanata996/chix/internal/inputschema"
)

type Runtime struct {
	parent *Runtime
	local  scopeConfig
}

type scopeConfig struct {
	errorMappers     []ErrorMapper
	observer         Observer
	hasObserver      bool
	extractor        Extractor
	hasExtractor     bool
	successStatus    int
	hasSuccessStatus bool
}

type Operation[I any, O any] struct {
	Name          string
	Method        string
	Pattern       string
	SuccessStatus int
	Validate      Validator[I]
	ErrorMappers  []ErrorMapper
}

type Handler[I any, O any] func(context.Context, *I) (*O, error)

type Violation struct {
	Source  string `json:"source"`
	Field   string `json:"field"`
	Code    string `json:"code"`
	Message string `json:"message"`
}

type Validator[I any] func(context.Context, *I) []Violation

type Event struct {
	Error      error
	Public     *HTTPError
	RequestID  string
	Method     string
	Target     string
	RemoteAddr string
}

type Observer func(Event)

type Extractor func(*http.Request) RequestContext

type RequestContext struct {
	RequestID  string
	Method     string
	Target     string
	RemoteAddr string
}

type Option interface {
	apply(*scopeConfig)
}

type optionFunc func(*scopeConfig)

func (fn optionFunc) apply(cfg *scopeConfig) {
	fn(cfg)
}

func New(opts ...Option) *Runtime {
	rt := &Runtime{}
	rt.local.extractor = DefaultExtractor
	rt.local.hasExtractor = true
	applyOptions(&rt.local, opts)
	return rt
}

func (rt *Runtime) Scope(opts ...Option) *Runtime {
	if rt == nil {
		panic("chix: runtime must not be nil")
	}

	child := &Runtime{parent: rt}
	applyOptions(&child.local, opts)
	return child
}

func Handle[I any, O any](rt *Runtime, op Operation[I, O], h Handler[I, O]) http.Handler {
	if rt == nil {
		panic("chix: runtime must not be nil")
	}
	if h == nil {
		panic("chix: handler must not be nil")
	}

	inputSchema, err := inputschema.Load(reflect.TypeOf((*I)(nil)).Elem())
	if err != nil {
		panic(err)
	}

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		execute(rt, w, r, op, h, inputSchema)
	})
}

func DefaultExtractor(r *http.Request) RequestContext {
	if r == nil {
		return RequestContext{}
	}

	return RequestContext{
		RequestID:  middleware.GetReqID(r.Context()),
		Method:     r.Method,
		Target:     r.URL.RequestURI(),
		RemoteAddr: r.RemoteAddr,
	}
}

func DefaultLogger(logger *slog.Logger) Observer {
	if logger == nil {
		logger = slog.Default()
	}

	return func(event Event) {
		public := normalizeHTTPError(event.Public)
		logger.Error(
			public.Message,
			"status", public.Status,
			"code", public.Code,
			"request_id", event.RequestID,
			"method", event.Method,
			"target", event.Target,
			"remote_addr", event.RemoteAddr,
			"error", event.Error,
		)
	}
}

func WithErrorMapper(mapper ErrorMapper) Option {
	return optionFunc(func(cfg *scopeConfig) {
		if mapper == nil {
			return
		}
		cfg.errorMappers = append(cfg.errorMappers, mapper)
	})
}

func WithObserver(observer Observer) Option {
	return optionFunc(func(cfg *scopeConfig) {
		cfg.observer = observer
		cfg.hasObserver = true
	})
}

func WithExtractor(extractor Extractor) Option {
	return optionFunc(func(cfg *scopeConfig) {
		cfg.extractor = extractor
		cfg.hasExtractor = true
	})
}

func WithSuccessStatus(status int) Option {
	if status <= 0 {
		panic("chix: success status must be positive")
	}

	return optionFunc(func(cfg *scopeConfig) {
		cfg.successStatus = status
		cfg.hasSuccessStatus = true
	})
}

func applyOptions(cfg *scopeConfig, opts []Option) {
	for _, opt := range opts {
		if opt == nil {
			continue
		}
		opt.apply(cfg)
	}
}

func execute[I any, O any](
	rt *Runtime,
	w http.ResponseWriter,
	r *http.Request,
	op Operation[I, O],
	h Handler[I, O],
	inputSchema *inputschema.Schema,
) {
	requestContext := rt.extractRequestContext(r)

	var input I
	if err := bindInputWithSchema(r, &input, inputSchema); err != nil {
		rt.writeFailure(w, requestContext, err, op.ErrorMappers)
		return
	}

	if op.Validate != nil {
		if violations := op.Validate(r.Context(), &input); len(violations) > 0 {
			rt.writeFailure(w, requestContext, newInvalidRequestError(violations), op.ErrorMappers)
			return
		}
	}

	output, err := h(r.Context(), &input)
	if err != nil {
		rt.writeFailure(w, requestContext, err, op.ErrorMappers)
		return
	}

	status := resolveSuccessStatus(op.Method, r.Method, op.SuccessStatus, rt.successStatus())
	if status == http.StatusNoContent {
		w.WriteHeader(status)
		return
	}

	payload, err := marshalSuccessEnvelope(output)
	if err != nil {
		rt.writeFailure(w, requestContext, err, op.ErrorMappers)
		return
	}

	if err := writeJSONResponse(w, status, payload); err != nil {
		rt.observe(requestContext, err, internalHTTPError())
	}
}

func resolveSuccessStatus(opMethod string, requestMethod string, explicit int, inherited int) int {
	if explicit > 0 {
		return explicit
	}
	if inherited > 0 {
		return inherited
	}

	method := strings.ToUpper(opMethod)
	if method == "" {
		method = strings.ToUpper(requestMethod)
	}
	if method == http.MethodPost {
		return http.StatusCreated
	}
	return http.StatusOK
}

func (rt *Runtime) writeFailure(w http.ResponseWriter, requestContext RequestContext, raw error, opMappers []ErrorMapper) {
	public := rt.resolvePublicError(raw, opMappers)
	rt.observe(requestContext, raw, public)

	payload, err := marshalErrorEnvelope(public)
	if err != nil {
		internal := internalHTTPError()
		rt.observe(requestContext, err, internal)

		fallbackPayload, fallbackErr := marshalErrorEnvelope(internal)
		if fallbackErr != nil {
			return
		}
		if writeErr := writeJSONResponse(w, internal.Status, fallbackPayload); writeErr != nil {
			rt.observe(requestContext, writeErr, internal)
		}
		return
	}

	if err := writeJSONResponse(w, public.Status, payload); err != nil {
		rt.observe(requestContext, err, internalHTTPError())
	}
}

func (rt *Runtime) resolvePublicError(raw error, opMappers []ErrorMapper) *HTTPError {
	public := publicErrorFromRuntime(raw)
	if public != nil {
		return public
	}

	public = publicErrorFromValue(raw)
	if public != nil {
		return public
	}

	for _, mapper := range opMappers {
		if mapper == nil {
			continue
		}
		if mapped := mapper(raw); mapped != nil {
			return normalizeHTTPError(mapped)
		}
	}

	for _, mapper := range rt.errorMappers() {
		if mapper == nil {
			continue
		}
		if mapped := mapper(raw); mapped != nil {
			return normalizeHTTPError(mapped)
		}
	}

	return internalHTTPError()
}

func (rt *Runtime) observe(requestContext RequestContext, raw error, public *HTTPError) {
	observer := rt.observer()
	if observer == nil {
		return
	}

	observer(Event{
		Error:      raw,
		Public:     normalizeHTTPError(public),
		RequestID:  requestContext.RequestID,
		Method:     requestContext.Method,
		Target:     requestContext.Target,
		RemoteAddr: requestContext.RemoteAddr,
	})
}

func (rt *Runtime) extractRequestContext(r *http.Request) RequestContext {
	extractor := rt.extractor()
	if extractor == nil {
		return RequestContext{}
	}
	return extractor(r)
}

func (rt *Runtime) extractor() Extractor {
	for current := rt; current != nil; current = current.parent {
		if current.local.hasExtractor {
			return current.local.extractor
		}
	}
	return DefaultExtractor
}

func (rt *Runtime) observer() Observer {
	for current := rt; current != nil; current = current.parent {
		if current.local.hasObserver {
			return current.local.observer
		}
	}
	return nil
}

func (rt *Runtime) successStatus() int {
	for current := rt; current != nil; current = current.parent {
		if current.local.hasSuccessStatus {
			return current.local.successStatus
		}
	}
	return 0
}

func (rt *Runtime) errorMappers() []ErrorMapper {
	var chain []ErrorMapper
	for current := rt; current != nil; current = current.parent {
		chain = append(chain, current.local.errorMappers...)
	}
	return chain
}
