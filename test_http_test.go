package chix_test

import (
	"net/http"
	"net/http/httptest"
)

func newRequest() *http.Request {
	return httptest.NewRequest(http.MethodGet, "/", nil)
}

func newResponseRecorder() *httptest.ResponseRecorder {
	return httptest.NewRecorder()
}
