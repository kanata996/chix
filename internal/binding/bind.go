package binding

import (
	"fmt"
	"net/http"
	"reflect"

	"github.com/kanata996/chix/internal/schema"
)

func Bind(r *http.Request, dst any) error {
	return bind(r, dst, nil)
}

func BindWithSchema(r *http.Request, dst any, sch *schema.Schema) error {
	return bind(r, dst, sch)
}

func bind(r *http.Request, dst any, sch *schema.Schema) error {
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

	if sch == nil {
		var err error
		sch, err = schema.Load(target.Type())
		if err != nil {
			return err
		}
	}

	if err := bindParameterFields(r, target, sch); err != nil {
		return err
	}
	if err := bindBodyFields(r, target, sch); err != nil {
		return err
	}
	if err := validateStruct(dst, sch); err != nil {
		return err
	}

	return nil
}
