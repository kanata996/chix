package chix

import (
	"encoding/json"
	"html/template"
	"net/http"
	"sync"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"

	"github.com/kanata996/chix/reqx"
)

type Config struct {
	Title                    string
	Version                  string
	Description              string
	DocsPath                 string
	OpenAPIPath              string
	RequestDecoder           *reqx.Decoder
	DisableDefaultMiddleware bool
	Middlewares              []func(http.Handler) http.Handler
}

type App struct {
	router      chi.Router
	docsPath    string
	openAPIPath string
	reqDecoder  *reqx.Decoder

	mu  sync.RWMutex
	doc *Document
}

func New(config Config) *App {
	docsPath := config.DocsPath
	if docsPath == "" {
		docsPath = "/docs"
	}

	openAPIPath := config.OpenAPIPath
	if openAPIPath == "" {
		openAPIPath = "/openapi.json"
	}

	router := chi.NewRouter()
	if !config.DisableDefaultMiddleware {
		router.Use(
			middleware.RequestID,
			middleware.RealIP,
			middleware.Recoverer,
			middleware.StripSlashes,
		)
	}
	if len(config.Middlewares) > 0 {
		router.Use(config.Middlewares...)
	}

	app := &App{
		router:      router,
		docsPath:    docsPath,
		openAPIPath: openAPIPath,
		reqDecoder:  requestDecoder(config.RequestDecoder),
		doc:         newDocument(config),
	}
	app.installInternalRoutes()

	return app
}

func requestDecoder(decoder *reqx.Decoder) *reqx.Decoder {
	if decoder != nil {
		return decoder
	}
	return reqx.New()
}

func (a *App) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	a.router.ServeHTTP(w, r)
}

func (a *App) Router() chi.Router {
	return a.router
}

func (a *App) Use(middlewares ...func(http.Handler) http.Handler) {
	a.router.Use(middlewares...)
}

func (a *App) OpenAPIDocument() Document {
	a.mu.RLock()
	defer a.mu.RUnlock()

	raw, _ := json.Marshal(a.doc)
	var out Document
	_ = json.Unmarshal(raw, &out)
	return out
}

func (a *App) registerOperation(method, path string, handler http.Handler, operation *OperationDoc) {
	a.mu.Lock()
	a.doc.addOperation(method, path, operation)
	a.mu.Unlock()

	a.router.Method(method, path, handler)
}

func (a *App) installInternalRoutes() {
	a.router.Get(a.openAPIPath, a.serveOpenAPI)
	a.router.Get(a.docsPath, a.serveDocs)
}

func (a *App) serveOpenAPI(w http.ResponseWriter, _ *http.Request) {
	a.mu.RLock()
	defer a.mu.RUnlock()

	w.Header().Set("Content-Type", "application/vnd.oai+json")
	encoder := json.NewEncoder(w)
	encoder.SetIndent("", "  ")
	if err := encoder.Encode(a.doc); err != nil {
		http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
	}
}

var docsTemplate = template.Must(template.New("docs").Parse(`<!doctype html>
<html lang="en">
<head>
  <meta charset="utf-8">
  <meta name="viewport" content="width=device-width,initial-scale=1">
  <title>{{ .Title }}</title>
  <link rel="stylesheet" href="https://unpkg.com/swagger-ui-dist@5/swagger-ui.css">
  <style>
    body { margin: 0; background: #0b1220; }
    #app { min-height: 100vh; }
  </style>
</head>
<body>
  <div id="app"></div>
  <script src="https://unpkg.com/swagger-ui-dist@5/swagger-ui-bundle.js"></script>
  <script>
    window.ui = SwaggerUIBundle({
      url: {{ .SpecURL }},
      dom_id: '#app',
      deepLinking: true,
      docExpansion: 'list'
    });
  </script>
</body>
</html>`))

func (a *App) serveDocs(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	data := struct {
		Title   string
		SpecURL template.JS
	}{
		Title:   "chix API Docs",
		SpecURL: template.JS(strconvQuote(a.openAPIPath)),
	}

	if err := docsTemplate.Execute(w, data); err != nil {
		http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
	}
}

func strconvQuote(value string) string {
	raw, _ := json.Marshal(value)
	return string(raw)
}
