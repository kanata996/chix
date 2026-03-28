package binding

import (
	"encoding/json"
	"fmt"
	"io"
	"mime"
	"net/http"
	"reflect"
	"strings"
)

type bodyField struct {
	name  string
	index []int
}

func bindBodyFields(r *http.Request, value reflect.Value) error {
	fields, err := collectBodyFields(value.Type())
	if err != nil {
		return err
	}
	if !requestHasBody(r) {
		return nil
	}

	if !isJSONContentType(r.Header.Get("Content-Type")) {
		return newUnsupportedMediaTypeError(fmt.Errorf("unsupported content type %q", r.Header.Get("Content-Type")))
	}

	allowed := make(map[string]bodyField, len(fields))
	for _, field := range fields {
		allowed[field.name] = field
	}

	var raw map[string]json.RawMessage
	decoder := json.NewDecoder(r.Body)
	if err := decoder.Decode(&raw); err != nil {
		if err == io.EOF {
			return nil
		}
		return newRequestShapeError(err)
	}

	var extra json.RawMessage
	if err := decoder.Decode(&extra); err != io.EOF {
		return newRequestShapeError(fmt.Errorf("request body must contain a single JSON value"))
	}

	for name := range raw {
		if _, ok := allowed[name]; !ok {
			return newRequestShapeError(fmt.Errorf("unknown body field %q", name))
		}
	}

	for name, rawValue := range raw {
		field := allowed[name]
		target, err := fieldByIndexAlloc(value, field.index)
		if err != nil {
			return err
		}
		if err := unmarshalBodyField(target, rawValue); err != nil {
			return newRequestShapeError(fmt.Errorf("invalid body field %q: %w", name, err))
		}
	}

	return nil
}

func requestHasBody(r *http.Request) bool {
	if r == nil || r.Body == nil {
		return false
	}
	return r.ContentLength > 0 || len(r.TransferEncoding) > 0
}

func isJSONContentType(raw string) bool {
	if raw == "" {
		return false
	}

	mediaType, _, err := mime.ParseMediaType(raw)
	if err != nil {
		return false
	}
	return mediaType == "application/json" || strings.HasSuffix(mediaType, "+json")
}

func collectBodyFields(t reflect.Type) ([]bodyField, error) {
	seen := map[string]struct{}{}
	var fields []bodyField
	var collectErr error

	walkStructFields(t, func(field reflect.StructField, index []int) bool {
		if collectErr != nil {
			return false
		}
		if field.PkgPath != "" && !field.Anonymous {
			return true
		}

		source, _, ok, err := parameterSource(field)
		if err != nil {
			collectErr = err
			return false
		}
		if ok {
			if explicitBodyTag(field) {
				collectErr = fmt.Errorf("chix: field %q must declare a single input source, found %s and json", field.Name, source)
				return false
			}
			return true
		}

		name, ok := bodyFieldName(field)
		if !ok {
			return true
		}
		if _, exists := seen[name]; exists {
			collectErr = fmt.Errorf("chix: duplicate body field %q", name)
			return false
		}

		seen[name] = struct{}{}
		fields = append(fields, bodyField{name: name, index: index})
		return true
	})

	return fields, collectErr
}

func unmarshalBodyField(target reflect.Value, raw json.RawMessage) error {
	if target.CanAddr() {
		return json.Unmarshal(raw, target.Addr().Interface())
	}
	return fmt.Errorf("field is not addressable")
}
