package binding

import (
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

	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()

	token, err := decoder.Token()
	if err != nil {
		if err == io.EOF {
			return nil
		}
		return newRequestShapeError(err)
	}

	delim, ok := token.(json.Delim)
	if !ok || delim != '{' {
		return newRequestShapeError(fmt.Errorf("request body must be a JSON object"))
	}

	for decoder.More() {
		token, err := decoder.Token()
		if err != nil {
			return newRequestShapeError(err)
		}

		name, ok := token.(string)
		if !ok {
			return newRequestShapeError(fmt.Errorf("request body must be a JSON object"))
		}

		field, ok := schema.LookupBodyField(name)
		if !ok {
			return newRequestShapeError(fmt.Errorf("unknown body field %q", name))
		}

		target, err := fieldByIndexAlloc(value, field.Index)
		if err != nil {
			return err
		}

		if err := decodeBodyField(decoder, target); err != nil {
			return newRequestShapeError(fmt.Errorf("invalid body field %q: %w", name, err))
		}
	}

	token, err = decoder.Token()
	if err != nil {
		return newRequestShapeError(err)
	}
	delim, ok = token.(json.Delim)
	if !ok || delim != '}' {
		return newRequestShapeError(fmt.Errorf("request body must be a JSON object"))
	}

	var extra json.RawMessage
	if err := decoder.Decode(&extra); err != io.EOF {
		return newRequestShapeError(fmt.Errorf("request body must contain a single JSON value"))
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
	if raw == "application/json" || strings.HasPrefix(raw, "application/json;") {
		return true
	}

	mediaType, _, err := mime.ParseMediaType(raw)
	if err != nil {
		return false
	}
	return mediaType == "application/json" ||
		(strings.HasPrefix(mediaType, "application/") && strings.HasSuffix(mediaType, "+json"))
}

func decodeBodyField(decoder *json.Decoder, target reflect.Value) error {
	if !target.CanAddr() {
		return fmt.Errorf("field is not addressable")
	}

	if err := decoder.Decode(target.Addr().Interface()); err != nil {
		return err
	}
	return nil
}
