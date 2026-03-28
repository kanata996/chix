package binding

import (
	"fmt"
	"net/http"
	"reflect"
)

func Bind(r *http.Request, dst any) error {
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

	if err := bindParameterFields(r, target); err != nil {
		return err
	}
	if err := bindBodyFields(r, target); err != nil {
		return err
	}

	return nil
}
