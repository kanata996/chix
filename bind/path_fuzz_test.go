package bind

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func FuzzBindPublicContracts(f *testing.F) {
	f.Add(http.MethodGet, "/items?page=1", "", "application/json")
	f.Add(http.MethodPost, "/items", `{"name":"kanata"}`, mimeApplicationJSON)
	f.Add(http.MethodPost, "/items", "", "text/plain")
	f.Add(http.MethodPost, "/items", " \n\t ", mimeApplicationJSON)

	f.Fuzz(func(t *testing.T, method, target, body, contentType string) {
		type request struct {
			ID   string `param:"id" query:"id" json:"id"`
			Page int    `query:"page"`
			Name string `json:"name" header:"x-name"`
		}

		req := httptest.NewRequest(method, target, strings.NewReader(body))
		if contentType != "" {
			req.Header.Set("Content-Type", contentType)
		}
		req.SetPathValue("id", "route-id")
		req.Pattern = "/items/{id}"

		var bound request
		err := Bind(req, &bound)
		if err == nil {
			return
		}

		httpErr := assertHTTPErrorLike(t, err)
		switch httpErr.Status() {
		case http.StatusBadRequest, http.StatusUnsupportedMediaType, http.StatusRequestEntityTooLarge:
		default:
			t.Fatalf("unexpected status %d", httpErr.Status())
		}
	})
}
