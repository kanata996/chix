package chix

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"

	"github.com/go-chi/chi/v5"
)

var (
	benchmarkStringSink string
	benchmarkBytesSink  int
)

type benchmarkResponseWriter struct {
	header http.Header
	status int
	bytes  int
}

func newBenchmarkResponseWriter() *benchmarkResponseWriter {
	return &benchmarkResponseWriter{
		header: make(http.Header),
	}
}

func (w *benchmarkResponseWriter) Header() http.Header {
	return w.header
}

func (w *benchmarkResponseWriter) WriteHeader(status int) {
	w.status = status
}

func (w *benchmarkResponseWriter) Write(payload []byte) (int, error) {
	w.bytes += len(payload)
	return len(payload), nil
}

func (w *benchmarkResponseWriter) Reset() {
	clear(w.header)
	w.status = 0
	w.bytes = 0
}

func benchmarkRequest(method string, target string, body []byte) *http.Request {
	var reader io.Reader
	if body != nil {
		reader = bytes.NewReader(body)
	}

	req := httptest.NewRequest(method, target, reader)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	return req
}

func benchmarkRequestWithRouteParams(method string, target string, body []byte, params map[string]string) *http.Request {
	req := benchmarkRequest(method, target, body)
	routeContext := chi.NewRouteContext()
	for key, value := range params {
		routeContext.URLParams.Add(key, value)
	}
	return req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, routeContext))
}

type benchmarkDeleteInput struct {
	ID string `path:"id" validate:"required"`
}

type benchmarkGetInput struct {
	ID      string `path:"id" validate:"required"`
	Verbose bool   `query:"verbose"`
}

type benchmarkPostInput struct {
	ID      string `path:"id" validate:"required,min=4"`
	Verbose bool   `query:"verbose"`
	Name    string `json:"name" validate:"required,min=3"`
	Age     int    `json:"age" validate:"gte=0,lte=150"`
}

type benchmarkUserOutput struct {
	ID      string `json:"id"`
	Name    string `json:"name"`
	Age     int    `json:"age"`
	Verbose bool   `json:"verbose"`
}

func BenchmarkChiVsChixMinimal204(b *testing.B) {
	chiHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		id := chi.URLParam(r, "id")
		if id == "" {
			w.WriteHeader(http.StatusBadRequest)
			return
		}

		benchmarkStringSink = id
		w.WriteHeader(http.StatusNoContent)
	})

	chixHandler := Handle(New(), Operation[benchmarkDeleteInput, struct{}]{
		Method:        http.MethodDelete,
		SuccessStatus: http.StatusNoContent,
	}, func(_ context.Context, input *benchmarkDeleteInput) (*struct{}, error) {
		benchmarkStringSink = input.ID
		return &struct{}{}, nil
	})

	req := benchmarkRequestWithRouteParams(http.MethodDelete, "/users/1234", nil, map[string]string{
		"id": "1234",
	})

	b.Run("chi", func(b *testing.B) {
		w := newBenchmarkResponseWriter()
		b.ReportAllocs()
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			w.Reset()
			chiHandler.ServeHTTP(w, req)
		}
	})

	b.Run("chix", func(b *testing.B) {
		w := newBenchmarkResponseWriter()
		b.ReportAllocs()
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			w.Reset()
			chixHandler.ServeHTTP(w, req)
		}
	})
}

func BenchmarkChiVsChixGetJSON(b *testing.B) {
	chiHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		id := chi.URLParam(r, "id")
		if id == "" {
			w.WriteHeader(http.StatusBadRequest)
			return
		}

		verbose, err := strconv.ParseBool(r.URL.Query().Get("verbose"))
		if err != nil {
			w.WriteHeader(http.StatusBadRequest)
			return
		}

		payload, err := json.Marshal(benchmarkUserOutput{
			ID:      id,
			Name:    "kanata",
			Age:     18,
			Verbose: verbose,
		})
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		w.WriteHeader(http.StatusOK)
		n, _ := w.Write(payload)
		benchmarkBytesSink = n
	})

	chixHandler := Handle(New(), Operation[benchmarkGetInput, benchmarkUserOutput]{
		Method: http.MethodGet,
	}, func(_ context.Context, input *benchmarkGetInput) (*benchmarkUserOutput, error) {
		return &benchmarkUserOutput{
			ID:      input.ID,
			Name:    "kanata",
			Age:     18,
			Verbose: input.Verbose,
		}, nil
	})

	req := benchmarkRequestWithRouteParams(http.MethodGet, "/users/1234?verbose=true", nil, map[string]string{
		"id": "1234",
	})

	b.Run("chi", func(b *testing.B) {
		w := newBenchmarkResponseWriter()
		b.ReportAllocs()
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			w.Reset()
			chiHandler.ServeHTTP(w, req)
		}
	})

	b.Run("chix", func(b *testing.B) {
		w := newBenchmarkResponseWriter()
		b.ReportAllocs()
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			w.Reset()
			chixHandler.ServeHTTP(w, req)
		}
	})
}

func BenchmarkChiVsChixPostJSON(b *testing.B) {
	body := []byte(`{"name":"kanata","age":18}`)

	chiHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		id := chi.URLParam(r, "id")
		if len(id) < 4 {
			w.WriteHeader(http.StatusBadRequest)
			return
		}

		verbose, err := strconv.ParseBool(r.URL.Query().Get("verbose"))
		if err != nil {
			w.WriteHeader(http.StatusBadRequest)
			return
		}

		var input struct {
			Name string `json:"name"`
			Age  int    `json:"age"`
		}

		decoder := json.NewDecoder(r.Body)
		decoder.DisallowUnknownFields()
		if err := decoder.Decode(&input); err != nil {
			w.WriteHeader(http.StatusBadRequest)
			return
		}

		var extra json.RawMessage
		if err := decoder.Decode(&extra); err != io.EOF {
			w.WriteHeader(http.StatusBadRequest)
			return
		}

		if len(input.Name) < 3 || input.Age < 0 || input.Age > 150 {
			w.WriteHeader(http.StatusUnprocessableEntity)
			return
		}

		payload, err := json.Marshal(benchmarkUserOutput{
			ID:      id,
			Name:    input.Name,
			Age:     input.Age,
			Verbose: verbose,
		})
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		w.WriteHeader(http.StatusCreated)
		n, _ := w.Write(payload)
		benchmarkBytesSink = n
	})

	chixHandler := Handle(New(), Operation[benchmarkPostInput, benchmarkUserOutput]{
		Method: http.MethodPost,
	}, func(_ context.Context, input *benchmarkPostInput) (*benchmarkUserOutput, error) {
		return &benchmarkUserOutput{
			ID:      input.ID,
			Name:    input.Name,
			Age:     input.Age,
			Verbose: input.Verbose,
		}, nil
	})

	b.Run("chi", func(b *testing.B) {
		w := newBenchmarkResponseWriter()
		b.ReportAllocs()
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			req := benchmarkRequestWithRouteParams(http.MethodPost, "/users/1234?verbose=true", body, map[string]string{
				"id": "1234",
			})
			w.Reset()
			chiHandler.ServeHTTP(w, req)
		}
	})

	b.Run("chix", func(b *testing.B) {
		w := newBenchmarkResponseWriter()
		b.ReportAllocs()
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			req := benchmarkRequestWithRouteParams(http.MethodPost, "/users/1234?verbose=true", body, map[string]string{
				"id": "1234",
			})
			w.Reset()
			chixHandler.ServeHTTP(w, req)
		}
	})
}
