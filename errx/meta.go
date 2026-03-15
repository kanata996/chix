package errx

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strings"
)

type standardMapping struct {
	match   error
	mapping Mapping
}

var (
	semanticMappings = []standardMapping{
		{match: ErrInvalidRequest, mapping: statusMapping(http.StatusBadRequest, CodeInvalidRequest)},
		{match: ErrUnauthorized, mapping: statusMapping(http.StatusUnauthorized, CodeUnauthorized)},
		{match: ErrForbidden, mapping: statusMapping(http.StatusForbidden, CodeForbidden)},
		{match: ErrNotFound, mapping: statusMapping(http.StatusNotFound, CodeNotFound)},
		{match: ErrConflict, mapping: statusMapping(http.StatusConflict, CodeConflict)},
		{match: ErrUnprocessableEntity, mapping: statusMapping(http.StatusUnprocessableEntity, CodeUnprocessableEntity)},
		{match: ErrTooManyRequests, mapping: statusMapping(http.StatusTooManyRequests, CodeTooManyRequests)},
		{match: ErrServiceUnavailable, mapping: statusMapping(http.StatusServiceUnavailable, CodeServiceUnavailable)},
		{match: ErrTimeout, mapping: statusMapping(http.StatusGatewayTimeout, CodeTimeout)},
	}

	clientClosedMapping = Mapping{
		StatusCode: 499,
		Code:       CodeClientClosed,
		Message:    "Client Closed Request",
	}

	reservedMappingsByCode = map[int64]Mapping{
		CodeInvalidRequest:       statusMapping(http.StatusBadRequest, CodeInvalidRequest),
		CodeUnauthorized:         statusMapping(http.StatusUnauthorized, CodeUnauthorized),
		CodeForbidden:            statusMapping(http.StatusForbidden, CodeForbidden),
		CodeNotFound:             statusMapping(http.StatusNotFound, CodeNotFound),
		CodeConflict:             statusMapping(http.StatusConflict, CodeConflict),
		CodePayloadTooLarge:      statusMapping(http.StatusRequestEntityTooLarge, CodePayloadTooLarge),
		CodeUnsupportedMediaType: statusMapping(http.StatusUnsupportedMediaType, CodeUnsupportedMediaType),
		CodeUnprocessableEntity:  statusMapping(http.StatusUnprocessableEntity, CodeUnprocessableEntity),
		CodeTooManyRequests:      statusMapping(http.StatusTooManyRequests, CodeTooManyRequests),
		CodeClientClosed:         clientClosedMapping,
		CodeInternal:             statusMapping(http.StatusInternalServerError, CodeInternal),
		CodeServiceUnavailable:   statusMapping(http.StatusServiceUnavailable, CodeServiceUnavailable),
		CodeTimeout:              statusMapping(http.StatusGatewayTimeout, CodeTimeout),
	}
)

// Lookup 查询 errx 内建标准语义与 transport 生命周期错误。
// 未识别时返回 ok=false，由 feature mapper 或 fallback 决定后续行为。
func Lookup(err error) (Mapping, bool) {
	switch {
	case errors.Is(err, context.Canceled):
		return clientClosedMapping, true
	case errors.Is(err, context.DeadlineExceeded):
		return statusMapping(http.StatusGatewayTimeout, CodeTimeout), true
	}

	return lookupSemanticMapping(err)
}

func Internal(code int64) Mapping {
	if code <= 0 {
		panic(fmt.Sprintf("errx: internal code must be positive, got %d", code))
	}

	return Mapping{
		StatusCode: http.StatusInternalServerError,
		Code:       code,
		Message:    http.StatusText(http.StatusInternalServerError),
	}
}

func (m Mapping) Validate() error {
	return validateMapping(m)
}

func validateMapping(mapping Mapping) error {
	if !validStatusCode(mapping.StatusCode) {
		return fmt.Errorf("status code must be 4xx/5xx or 499, got %d", mapping.StatusCode)
	}
	if mapping.Code <= 0 {
		return fmt.Errorf("code must be positive, got %d", mapping.Code)
	}
	if strings.TrimSpace(mapping.Message) == "" {
		return errors.New("message must not be blank")
	}
	if reserved, ok := reservedMappingsByCode[mapping.Code]; ok {
		if mapping.StatusCode != reserved.StatusCode {
			return fmt.Errorf("reserved code %d requires status code %d, got %d", mapping.Code, reserved.StatusCode, mapping.StatusCode)
		}
		if mapping.Message != reserved.Message {
			return fmt.Errorf("reserved code %d requires message %q, got %q", mapping.Code, reserved.Message, mapping.Message)
		}
	}

	return nil
}

func lookupSemanticMapping(err error) (Mapping, bool) {
	for _, standard := range semanticMappings {
		if errors.Is(err, standard.match) {
			return standard.mapping, true
		}
	}

	return Mapping{}, false
}

func statusMapping(statusCode int, code int64) Mapping {
	return Mapping{
		StatusCode: statusCode,
		Code:       code,
		Message:    http.StatusText(statusCode),
	}
}
