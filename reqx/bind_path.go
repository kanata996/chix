package reqx

import (
	"fmt"
	"net/http"
	"net/url"
	"reflect"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
)

func BindPathValues[T any](r *http.Request, dst *T) error {
	return bindTaggedValues(r, dst, pathSource, bindValuesConfig{allowUnknownFields: true})
}

func pathValues(r *http.Request) url.Values {
	values := url.Values{}
	if r == nil {
		return values
	}

	rctx := chi.RouteContext(r.Context())
	if rctx == nil {
		return values
	}

	for i := range rctx.URLParams.Keys {
		values.Add(
			rctx.URLParams.Keys[i],
			strings.TrimSpace(rctx.URLParams.Values[i]),
		)
	}

	return values
}

func ParamString(r *http.Request, name string) (string, error) {
	rawValues, err := requiredPathParamValues(r, name)
	if err != nil {
		return "", err
	}

	var value string
	violation, _ := decodeQueryField(reflect.ValueOf(&value).Elem(), rawValues, name)
	if violation != nil {
		return "", invalidFieldError(*violation)
	}
	return value, nil
}

func ParamInt(r *http.Request, name string) (int, error) {
	rawValues, err := requiredPathParamValues(r, name)
	if err != nil {
		return 0, err
	}

	var value int
	violation, _ := decodeQueryField(reflect.ValueOf(&value).Elem(), rawValues, name)
	if violation != nil {
		return 0, invalidFieldError(*violation)
	}
	return value, nil
}

func ParamUUID(r *http.Request, name string) (string, error) {
	raw, err := ParamString(r, name)
	if err != nil {
		return "", err
	}

	parsed, err := uuid.Parse(raw)
	if err != nil {
		return "", invalidFieldError(InvalidField(name))
	}
	return parsed.String(), nil
}

func requiredPathParamValues(r *http.Request, name string) ([]string, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return nil, fmt.Errorf("reqx: path param name must not be empty")
	}

	rawValues, ok := pathValues(r)[name]
	if !ok || len(rawValues) == 0 {
		return nil, invalidFieldError(RequiredField(name))
	}

	if len(rawValues) == 1 && rawValues[0] == "" {
		return nil, invalidFieldError(RequiredField(name))
	}

	return rawValues, nil
}
