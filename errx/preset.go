package errx

import "net/http"

func AsUnauthorized(code int64, message string) Mapping {
	return customMapping(http.StatusUnauthorized, code, message)
}

func AsForbidden(code int64, message string) Mapping {
	return customMapping(http.StatusForbidden, code, message)
}

func AsNotFound(code int64, message string) Mapping {
	return customMapping(http.StatusNotFound, code, message)
}

func AsConflict(code int64, message string) Mapping {
	return customMapping(http.StatusConflict, code, message)
}

func AsUnprocessable(code int64, message string) Mapping {
	return customMapping(http.StatusUnprocessableEntity, code, message)
}

func AsTooManyRequests(code int64, message string) Mapping {
	return customMapping(http.StatusTooManyRequests, code, message)
}

func AsServiceUnavailable(code int64, message string) Mapping {
	return customMapping(http.StatusServiceUnavailable, code, message)
}

func AsTimeout(code int64, message string) Mapping {
	return customMapping(http.StatusGatewayTimeout, code, message)
}

func customMapping(statusCode int, code int64, message string) Mapping {
	return Mapping{
		StatusCode: statusCode,
		Code:       code,
		Message:    message,
	}
}
