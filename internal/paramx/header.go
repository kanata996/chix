package paramx

import (
	"errors"
	"net/http"
	"strconv"
	"strings"

	"github.com/google/uuid"

	"github.com/kanata996/chix/reqx"
)

type HeaderReader struct {
	request *http.Request
}

func Header(r *http.Request) HeaderReader {
	return HeaderReader{request: r}
}

func (h HeaderReader) String(name string) (string, bool, error) {
	value, err := h.scalar(name)
	if err != nil {
		return "", false, err
	}
	if !value.present {
		return "", false, nil
	}
	return value.raw, true, nil
}

func (h HeaderReader) Strings(name string) ([]string, bool, error) {
	values, err := h.list(name)
	if err != nil {
		return nil, false, err
	}
	if len(values) == 0 {
		return nil, false, nil
	}
	return values, true, nil
}

func (h HeaderReader) RequiredString(name string) (string, error) {
	value, err := h.scalar(name)
	if err != nil {
		return "", err
	}
	if !value.present || value.raw == "" {
		return "", reqx.BadRequest(reqx.Required(reqx.InHeader, name))
	}
	return value.raw, nil
}

func (h HeaderReader) RequiredStrings(name string) ([]string, error) {
	values, err := h.list(name)
	if err != nil {
		return nil, err
	}
	if len(values) == 0 {
		return nil, reqx.BadRequest(reqx.Required(reqx.InHeader, name))
	}
	return values, nil
}

func (h HeaderReader) Int(name string) (int, bool, error) {
	value, err := h.scalar(name)
	if err != nil {
		return 0, false, err
	}
	if !value.present {
		return 0, false, nil
	}
	if value.raw == "" {
		return 0, false, reqx.BadRequest(reqx.InvalidInteger(reqx.InHeader, name))
	}

	parsed, err := strconv.Atoi(value.raw)
	if err != nil {
		return 0, false, reqx.BadRequest(reqx.InvalidInteger(reqx.InHeader, name))
	}
	return parsed, true, nil
}

func (h HeaderReader) RequiredInt(name string) (int, error) {
	value, err := h.scalar(name)
	if err != nil {
		return 0, err
	}
	if !value.present {
		return 0, reqx.BadRequest(reqx.Required(reqx.InHeader, name))
	}
	if value.raw == "" {
		return 0, reqx.BadRequest(reqx.InvalidInteger(reqx.InHeader, name))
	}

	parsed, err := strconv.Atoi(value.raw)
	if err != nil {
		return 0, reqx.BadRequest(reqx.InvalidInteger(reqx.InHeader, name))
	}
	return parsed, nil
}

func (h HeaderReader) UUID(name string) (string, bool, error) {
	value, err := h.scalar(name)
	if err != nil {
		return "", false, err
	}
	if !value.present {
		return "", false, nil
	}
	if value.raw == "" {
		return "", false, reqx.BadRequest(reqx.InvalidUUID(reqx.InHeader, name))
	}

	parsed, err := uuid.Parse(value.raw)
	if err != nil {
		return "", false, reqx.BadRequest(reqx.InvalidUUID(reqx.InHeader, name))
	}
	return parsed.String(), true, nil
}

func (h HeaderReader) RequiredUUID(name string) (string, error) {
	value, err := h.scalar(name)
	if err != nil {
		return "", err
	}
	if !value.present {
		return "", reqx.BadRequest(reqx.Required(reqx.InHeader, name))
	}
	if value.raw == "" {
		return "", reqx.BadRequest(reqx.InvalidUUID(reqx.InHeader, name))
	}

	parsed, err := uuid.Parse(value.raw)
	if err != nil {
		return "", reqx.BadRequest(reqx.InvalidUUID(reqx.InHeader, name))
	}
	return parsed.String(), nil
}

func (h HeaderReader) Bool(name string) (bool, bool, error) {
	value, err := h.scalar(name)
	if err != nil {
		return false, false, err
	}
	if !value.present {
		return false, false, nil
	}

	switch value.raw {
	case "true":
		return true, true, nil
	case "false":
		return false, true, nil
	default:
		return false, false, reqx.BadRequest(reqx.InvalidValue(reqx.InHeader, name))
	}
}

func (h HeaderReader) RequiredBool(name string) (bool, error) {
	value, err := h.scalar(name)
	if err != nil {
		return false, err
	}
	if !value.present {
		return false, reqx.BadRequest(reqx.Required(reqx.InHeader, name))
	}

	switch value.raw {
	case "true":
		return true, nil
	case "false":
		return false, nil
	default:
		return false, reqx.BadRequest(reqx.InvalidValue(reqx.InHeader, name))
	}
}

func (h HeaderReader) list(name string) ([]string, error) {
	name, err := normalizeName(name)
	if err != nil {
		return nil, err
	}
	if h.request == nil {
		return nil, errors.New("paramx: request must not be nil")
	}

	rawValues := h.request.Header.Values(name)
	if len(rawValues) == 0 {
		return nil, nil
	}

	values := make([]string, 0, len(rawValues))
	for _, rawValue := range rawValues {
		value := strings.TrimSpace(rawValue)
		if value == "" {
			return nil, reqx.BadRequest(reqx.InvalidValue(reqx.InHeader, name))
		}
		values = append(values, value)
	}
	return values, nil
}

func (h HeaderReader) scalar(name string) (scalarValue, error) {
	name, err := normalizeName(name)
	if err != nil {
		return scalarValue{}, err
	}
	if h.request == nil {
		return scalarValue{}, errors.New("paramx: request must not be nil")
	}

	values := h.request.Header.Values(name)
	switch len(values) {
	case 0:
		return scalarValue{}, nil
	case 1:
		return scalarValue{
			raw:     strings.TrimSpace(values[0]),
			present: true,
		}, nil
	default:
		return scalarValue{}, reqx.BadRequest(reqx.MultipleValues(reqx.InHeader, name))
	}
}
