package paramx

import (
	"errors"
	"github.com/kanata996/chix/reqx"
	"net/http"
	"strconv"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
)

type PathReader struct {
	request *http.Request
}

type QueryReader struct {
	request *http.Request
}

func Path(r *http.Request) PathReader {
	return PathReader{request: r}
}

func Query(r *http.Request) QueryReader {
	return QueryReader{request: r}
}

func (p PathReader) String(name string) (string, error) {
	name, err := normalizeName(name)
	if err != nil {
		return "", err
	}
	if p.request == nil {
		return "", errors.New("paramx: request must not be nil")
	}

	value := strings.TrimSpace(chi.URLParam(p.request, name))
	if value == "" {
		return "", reqx.BadRequest(reqx.Required(reqx.InPath, name))
	}
	return value, nil
}

func (p PathReader) UUID(name string) (string, error) {
	value, err := p.String(name)
	if err != nil {
		return "", err
	}

	parsed, err := uuid.Parse(value)
	if err != nil {
		return "", reqx.BadRequest(reqx.InvalidUUID(reqx.InPath, name))
	}
	return parsed.String(), nil
}

func (p PathReader) Int(name string) (int, error) {
	value, err := p.String(name)
	if err != nil {
		return 0, err
	}

	parsed, err := strconv.Atoi(value)
	if err != nil {
		return 0, reqx.BadRequest(reqx.InvalidInteger(reqx.InPath, name))
	}
	return parsed, nil
}

func (q QueryReader) String(name string) (string, bool, error) {
	value, err := q.scalar(name)
	if err != nil {
		return "", false, err
	}
	if !value.present {
		return "", false, nil
	}
	return value.raw, true, nil
}

func (q QueryReader) Strings(name string) ([]string, bool, error) {
	values, err := q.list(name)
	if err != nil {
		return nil, false, err
	}
	if len(values) == 0 {
		return nil, false, nil
	}
	return values, true, nil
}

func (q QueryReader) RequiredString(name string) (string, error) {
	value, err := q.scalar(name)
	if err != nil {
		return "", err
	}
	if !value.present || value.raw == "" {
		return "", reqx.BadRequest(reqx.Required(reqx.InQuery, name))
	}
	return value.raw, nil
}

func (q QueryReader) RequiredStrings(name string) ([]string, error) {
	values, err := q.list(name)
	if err != nil {
		return nil, err
	}
	if len(values) == 0 {
		return nil, reqx.BadRequest(reqx.Required(reqx.InQuery, name))
	}
	return values, nil
}

func (q QueryReader) Int(name string) (int, bool, error) {
	value, err := q.scalar(name)
	if err != nil {
		return 0, false, err
	}
	if !value.present {
		return 0, false, nil
	}
	if value.raw == "" {
		return 0, false, reqx.BadRequest(reqx.InvalidInteger(reqx.InQuery, name))
	}

	parsed, err := strconv.Atoi(value.raw)
	if err != nil {
		return 0, false, reqx.BadRequest(reqx.InvalidInteger(reqx.InQuery, name))
	}
	return parsed, true, nil
}

func (q QueryReader) RequiredInt(name string) (int, error) {
	value, err := q.scalar(name)
	if err != nil {
		return 0, err
	}
	if !value.present {
		return 0, reqx.BadRequest(reqx.Required(reqx.InQuery, name))
	}
	if value.raw == "" {
		return 0, reqx.BadRequest(reqx.InvalidInteger(reqx.InQuery, name))
	}

	parsed, err := strconv.Atoi(value.raw)
	if err != nil {
		return 0, reqx.BadRequest(reqx.InvalidInteger(reqx.InQuery, name))
	}
	return parsed, nil
}

func (q QueryReader) Int16(name string) (int16, bool, error) {
	value, err := q.scalar(name)
	if err != nil {
		return 0, false, err
	}
	if !value.present {
		return 0, false, nil
	}
	if value.raw == "" {
		return 0, false, reqx.BadRequest(reqx.InvalidInteger(reqx.InQuery, name))
	}

	parsed, err := strconv.ParseInt(value.raw, 10, 16)
	if err != nil {
		return 0, false, reqx.BadRequest(reqx.InvalidInteger(reqx.InQuery, name))
	}
	return int16(parsed), true, nil
}

func (q QueryReader) RequiredInt16(name string) (int16, error) {
	value, err := q.scalar(name)
	if err != nil {
		return 0, err
	}
	if !value.present {
		return 0, reqx.BadRequest(reqx.Required(reqx.InQuery, name))
	}
	if value.raw == "" {
		return 0, reqx.BadRequest(reqx.InvalidInteger(reqx.InQuery, name))
	}

	parsed, err := strconv.ParseInt(value.raw, 10, 16)
	if err != nil {
		return 0, reqx.BadRequest(reqx.InvalidInteger(reqx.InQuery, name))
	}
	return int16(parsed), nil
}

func (q QueryReader) UUID(name string) (string, bool, error) {
	value, err := q.scalar(name)
	if err != nil {
		return "", false, err
	}
	if !value.present {
		return "", false, nil
	}
	if value.raw == "" {
		return "", false, reqx.BadRequest(reqx.InvalidUUID(reqx.InQuery, name))
	}

	parsed, err := uuid.Parse(value.raw)
	if err != nil {
		return "", false, reqx.BadRequest(reqx.InvalidUUID(reqx.InQuery, name))
	}
	return parsed.String(), true, nil
}

func (q QueryReader) UUIDs(name string) ([]string, bool, error) {
	values, err := q.list(name)
	if err != nil {
		return nil, false, err
	}
	if len(values) == 0 {
		return nil, false, nil
	}

	parsedValues := make([]string, 0, len(values))
	for _, value := range values {
		parsed, err := uuid.Parse(value)
		if err != nil {
			return nil, false, reqx.BadRequest(reqx.InvalidUUID(reqx.InQuery, name))
		}
		parsedValues = append(parsedValues, parsed.String())
	}
	return parsedValues, true, nil
}

func (q QueryReader) RequiredUUID(name string) (string, error) {
	value, err := q.scalar(name)
	if err != nil {
		return "", err
	}
	if !value.present {
		return "", reqx.BadRequest(reqx.Required(reqx.InQuery, name))
	}
	if value.raw == "" {
		return "", reqx.BadRequest(reqx.InvalidUUID(reqx.InQuery, name))
	}

	parsed, err := uuid.Parse(value.raw)
	if err != nil {
		return "", reqx.BadRequest(reqx.InvalidUUID(reqx.InQuery, name))
	}
	return parsed.String(), nil
}

func (q QueryReader) RequiredUUIDs(name string) ([]string, error) {
	values, err := q.list(name)
	if err != nil {
		return nil, err
	}
	if len(values) == 0 {
		return nil, reqx.BadRequest(reqx.Required(reqx.InQuery, name))
	}

	parsedValues := make([]string, 0, len(values))
	for _, value := range values {
		parsed, err := uuid.Parse(value)
		if err != nil {
			return nil, reqx.BadRequest(reqx.InvalidUUID(reqx.InQuery, name))
		}
		parsedValues = append(parsedValues, parsed.String())
	}
	return parsedValues, nil
}

func (q QueryReader) Bool(name string) (bool, bool, error) {
	value, err := q.scalar(name)
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
		return false, false, reqx.BadRequest(reqx.InvalidValue(reqx.InQuery, name))
	}
}

func (q QueryReader) RequiredBool(name string) (bool, error) {
	value, err := q.scalar(name)
	if err != nil {
		return false, err
	}
	if !value.present {
		return false, reqx.BadRequest(reqx.Required(reqx.InQuery, name))
	}

	switch value.raw {
	case "true":
		return true, nil
	case "false":
		return false, nil
	default:
		return false, reqx.BadRequest(reqx.InvalidValue(reqx.InQuery, name))
	}
}

type scalarValue struct {
	raw     string
	present bool
}

func (q QueryReader) list(name string) ([]string, error) {
	name, err := normalizeName(name)
	if err != nil {
		return nil, err
	}
	if q.request == nil {
		return nil, errors.New("paramx: request must not be nil")
	}

	rawValues := q.request.URL.Query()[name]
	if len(rawValues) == 0 {
		return nil, nil
	}

	values := make([]string, 0, len(rawValues))
	for _, rawValue := range rawValues {
		value := strings.TrimSpace(rawValue)
		if value == "" {
			return nil, reqx.BadRequest(reqx.InvalidValue(reqx.InQuery, name))
		}
		values = append(values, value)
	}
	return values, nil
}

func (q QueryReader) scalar(name string) (scalarValue, error) {
	name, err := normalizeName(name)
	if err != nil {
		return scalarValue{}, err
	}
	if q.request == nil {
		return scalarValue{}, errors.New("paramx: request must not be nil")
	}

	values := q.request.URL.Query()[name]
	switch len(values) {
	case 0:
		return scalarValue{}, nil
	case 1:
		return scalarValue{
			raw:     strings.TrimSpace(values[0]),
			present: true,
		}, nil
	default:
		return scalarValue{}, reqx.BadRequest(reqx.MultipleValues(reqx.InQuery, name))
	}
}

func normalizeName(name string) (string, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return "", errors.New("paramx: name must not be blank")
	}
	return name, nil
}
