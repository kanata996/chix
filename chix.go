package chix

import (
	"net/http"

	hah "github.com/kanata996/hah"
)

// WriteError writes a structured error response using the default chi-oriented
// responder preset.
func WriteError(w http.ResponseWriter, r *http.Request, err error) error {
	return defaultErrorResponder.Respond(w, r, err)
}

// JSON writes a JSON response via hah's canonical responder implementation.
// The request is retained for chi-style handler symmetry and API stability.
func JSON(w http.ResponseWriter, _ *http.Request, status int, data any) error {
	return hah.JSON(w, status, data)
}

// JSONBlob writes a raw JSON response body via hah's canonical responder implementation.
func JSONBlob(w http.ResponseWriter, _ *http.Request, status int, body []byte) error {
	return hah.JSONBlob(w, status, body)
}

// OK writes a 200 JSON success response via hah's canonical responder implementation.
func OK(w http.ResponseWriter, _ *http.Request, data any) error {
	return hah.OK(w, data)
}

// Created writes a 201 JSON success response via hah's canonical responder implementation.
func Created(w http.ResponseWriter, _ *http.Request, data any) error {
	return hah.Created(w, data)
}

// NoContent writes a 204 response via hah's canonical responder implementation.
func NoContent(w http.ResponseWriter, _ *http.Request) error {
	return hah.NoContent(w)
}
