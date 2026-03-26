package chix

import (
	"errors"
	"net/http"

	"github.com/kanata996/chix/reqx"
)

func requestDecodeError(err error) error {
	var requestErr *reqx.RequestError
	if !errors.As(err, &requestErr) {
		return err
	}

	switch requestErr.Kind {
	case reqx.KindInvalidDestination:
		return StatusError(http.StatusInternalServerError, requestErr.Message).WithCause(err)
	case reqx.KindValidation:
		return StatusError(http.StatusUnprocessableEntity, requestErr.Message).WithCause(err).WithViolations(requestErr.Violations)
	case reqx.KindMissingBody, reqx.KindInvalidBody, reqx.KindMissingParameter, reqx.KindInvalidParameter:
		return StatusError(http.StatusBadRequest, requestErr.Message).WithCause(err)
	default:
		return StatusError(http.StatusBadRequest, requestErr.Message).WithCause(err)
	}
}
