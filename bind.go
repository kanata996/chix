package chix

import (
	"net/http"

	"github.com/kanata996/chix/internal/binding"
)

func bindInput(r *http.Request, dst any) error {
	err := binding.Bind(r, dst)
	switch binding.KindOf(err) {
	case binding.ErrorKindRequestShape:
		return newRequestShapeError(err)
	case binding.ErrorKindUnsupportedMediaType:
		return newUnsupportedMediaTypeError(err)
	default:
		return err
	}
}
