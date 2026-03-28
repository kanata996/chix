package binding

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"mime"
	"net/http"
	"reflect"
	"strings"

	"github.com/kanata996/chix/internal/schema"
)

func bindBodyFields(r *http.Request, value reflect.Value, schema *schema.Schema) error {
	if !requestHasBody(r) {
		return nil
	}

	if !isJSONContentType(r.Header.Get("Content-Type")) {
		return newUnsupportedMediaTypeError(fmt.Errorf("unsupported content type %q", r.Header.Get("Content-Type")))
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
		if _, ok := schema.LookupBodyField(name); !ok {
			return newRequestShapeError(fmt.Errorf("unknown body field %q", name))
		}
	}

	for name, rawValue := range raw {
		field, _ := schema.LookupBodyField(name)
		target, err := fieldByIndexAlloc(value, field.Index)
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

func unmarshalBodyField(target reflect.Value, raw json.RawMessage) error {
	if !target.CanAddr() {
		return fmt.Errorf("field is not addressable")
	}

	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()

	if err := decoder.Decode(target.Addr().Interface()); err != nil {
		return err
	}

	var extra json.RawMessage
	if err := decoder.Decode(&extra); err != io.EOF {
		return fmt.Errorf("body field must contain a single JSON value")
	}

	return nil
}
