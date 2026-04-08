package reqx

import (
	"errors"
	"io"
	"net/http"
)

var errRequestTooLarge = errors.New("reqx: request body too large")

func BindBody(r *http.Request, target any) error {
	if r == nil {
		return errorsf("request must not be nil")
	}
	if target == nil {
		return errorsf("destination must not be nil")
	}

	return bindBodyDefault(r, target, defaultBindConfig().body)
}

func readBody(body io.ReadCloser, maxBytes int64) ([]byte, error) {
	if body == nil {
		return nil, nil
	}
	if maxBytes <= 0 {
		maxBytes = defaultMaxBodyBytes
	}

	data, err := io.ReadAll(io.LimitReader(body, maxBytes+1))
	if err != nil {
		return nil, err
	}
	if int64(len(data)) > maxBytes {
		return nil, errRequestTooLarge
	}
	return data, nil
}
