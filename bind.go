package chix

import (
	"net/http"

	"github.com/kanata996/chix/internal/binding"
	"github.com/kanata996/chix/internal/inputschema"
)

func bindInput(r *http.Request, dst any) error {
	return normalizeBindError(binding.Bind(r, dst))
}

func bindInputWithSchema(r *http.Request, dst any, schema *inputschema.Schema) error {
	return normalizeBindError(binding.BindWithSchema(r, dst, schema))
}

func normalizeBindError(err error) error {
	switch binding.KindOf(err) {
	case binding.ErrorKindRequestShape:
		return newRequestShapeError(err)
	case binding.ErrorKindUnsupportedMediaType:
		return newUnsupportedMediaTypeError(err)
	default:
		return err
	}
}
