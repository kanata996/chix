package chix

import (
	"github.com/kanata996/chix/internal/paramx"
	"net/http"
)

type PathReader = paramx.PathReader
type QueryReader = paramx.QueryReader
type HeaderReader = paramx.HeaderReader

// Path returns a chi-backed path reader.
func Path(r *http.Request) PathReader {
	return paramx.Path(r)
}

// Query returns a query reader with chix's default parsing semantics.
func Query(r *http.Request) QueryReader {
	return paramx.Query(r)
}

// Header returns a header reader with chix's default parsing semantics.
func Header(r *http.Request) HeaderReader {
	return paramx.Header(r)
}
