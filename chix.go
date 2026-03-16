package chix

import (
	"net/http"

	"github.com/kanata996/chix/internal/paramx"
)

type PathReader struct {
	reader paramx.PathReader
}

type QueryReader struct {
	reader paramx.QueryReader
}

type HeaderReader struct {
	reader paramx.HeaderReader
}

// Path returns a chi-backed path reader.
func Path(r *http.Request) PathReader {
	return PathReader{reader: paramx.Path(r)}
}

// Query returns a query reader with chix's default parsing semantics.
func Query(r *http.Request) QueryReader {
	return QueryReader{reader: paramx.Query(r)}
}

// Header returns a header reader with chix's default parsing semantics.
func Header(r *http.Request) HeaderReader {
	return HeaderReader{reader: paramx.Header(r)}
}

func (p PathReader) String(name string) (string, error) {
	return p.reader.String(name)
}

func (p PathReader) UUID(name string) (string, error) {
	return p.reader.UUID(name)
}

func (p PathReader) Int(name string) (int, error) {
	return p.reader.Int(name)
}

func (q QueryReader) String(name string) (string, bool, error) {
	return q.reader.String(name)
}

func (q QueryReader) Strings(name string) ([]string, bool, error) {
	return q.reader.Strings(name)
}

func (q QueryReader) RequiredString(name string) (string, error) {
	return q.reader.RequiredString(name)
}

func (q QueryReader) RequiredStrings(name string) ([]string, error) {
	return q.reader.RequiredStrings(name)
}

func (q QueryReader) Int(name string) (int, bool, error) {
	return q.reader.Int(name)
}

func (q QueryReader) RequiredInt(name string) (int, error) {
	return q.reader.RequiredInt(name)
}

func (q QueryReader) Int16(name string) (int16, bool, error) {
	return q.reader.Int16(name)
}

func (q QueryReader) RequiredInt16(name string) (int16, error) {
	return q.reader.RequiredInt16(name)
}

func (q QueryReader) UUID(name string) (string, bool, error) {
	return q.reader.UUID(name)
}

func (q QueryReader) UUIDs(name string) ([]string, bool, error) {
	return q.reader.UUIDs(name)
}

func (q QueryReader) RequiredUUID(name string) (string, error) {
	return q.reader.RequiredUUID(name)
}

func (q QueryReader) RequiredUUIDs(name string) ([]string, error) {
	return q.reader.RequiredUUIDs(name)
}

func (q QueryReader) Bool(name string) (bool, bool, error) {
	return q.reader.Bool(name)
}

func (q QueryReader) RequiredBool(name string) (bool, error) {
	return q.reader.RequiredBool(name)
}

func (h HeaderReader) String(name string) (string, bool, error) {
	return h.reader.String(name)
}

func (h HeaderReader) Strings(name string) ([]string, bool, error) {
	return h.reader.Strings(name)
}

func (h HeaderReader) RequiredString(name string) (string, error) {
	return h.reader.RequiredString(name)
}

func (h HeaderReader) RequiredStrings(name string) ([]string, error) {
	return h.reader.RequiredStrings(name)
}

func (h HeaderReader) Int(name string) (int, bool, error) {
	return h.reader.Int(name)
}

func (h HeaderReader) RequiredInt(name string) (int, error) {
	return h.reader.RequiredInt(name)
}

func (h HeaderReader) UUID(name string) (string, bool, error) {
	return h.reader.UUID(name)
}

func (h HeaderReader) RequiredUUID(name string) (string, error) {
	return h.reader.RequiredUUID(name)
}

func (h HeaderReader) Bool(name string) (bool, bool, error) {
	return h.reader.Bool(name)
}

func (h HeaderReader) RequiredBool(name string) (bool, error) {
	return h.reader.RequiredBool(name)
}
