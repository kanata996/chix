package reqx

import "net/http"

func BindHeaders(r *http.Request, target any) error {
	if r == nil {
		return errorsf("request must not be nil")
	}
	if target == nil {
		return errorsf("destination must not be nil")
	}
	return bindHeadersDefault(r, target)
}
