package binding

import (
	"fmt"
	"net/http"
	"reflect"

	"github.com/kanata996/chix/internal/inputschema"
)

func Bind(r *http.Request, dst any) error {
	return bind(r, dst, nil)
}

func BindWithSchema(r *http.Request, dst any, schema *inputschema.Schema) error {
	return bind(r, dst, schema)
}

func bind(r *http.Request, dst any, schema *inputschema.Schema) error {
	if dst == nil {
		return nil
	}

	value := reflect.ValueOf(dst)
	if value.Kind() != reflect.Pointer || value.IsNil() {
		return fmt.Errorf("chix: input destination must be a non-nil pointer")
	}

	target := value.Elem()
	if target.Kind() != reflect.Struct {
		return fmt.Errorf("chix: input destination must point to a struct")
	}

	if schema == nil {
		var err error
		schema, err = inputschema.Load(target.Type())
		if err != nil {
			return err
		}
	}

	if err := bindParameterFields(r, target, schema); err != nil {
		return err
	}
	if err := bindBodyFields(r, target, schema); err != nil {
		return err
	}

	return nil
}
