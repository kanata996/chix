package binding

import (
	"fmt"
	"net/http"
	"reflect"

	"github.com/kanata996/chix/internal/schema"
)

// Bind 将请求中的参数和请求体绑定到目标结构体，并使用目标类型推导出的 schema 执行校验；dst 必须是指向结构体的非 nil 指针。
func Bind(r *http.Request, dst any) error {
	return bind(r, dst, nil)
}

// BindWithSchema 将请求中的参数和请求体绑定到目标结构体，并使用给定 schema 执行校验；sch 为空时会按目标类型加载 schema，dst 必须是指向结构体的非 nil 指针。
func BindWithSchema(r *http.Request, dst any, sch *schema.Schema) error {
	return bind(r, dst, sch)
}

// bind 将请求参数与请求体写入目标结构体，并在绑定后按 schema 执行校验；dst 为空时直接返回，sch 为空时按目标类型加载 schema。
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
