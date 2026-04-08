package reqx

import "net/http"

// Bind 按默认顺序绑定请求数据：path -> query(GET/DELETE/HEAD) -> body。
func Bind(r *http.Request, target any) error {
	if r == nil {
		return errorsf("request must not be nil")
	}
	if target == nil {
		return errorsf("destination must not be nil")
	}

	return bindWithConfig(r, target, defaultBindConfig())
}

func bindIntoClone[T any](dst *T, fn func(*T) error) error {
	if dst == nil {
		return errorsf("destination must not be nil")
	}

	bound := cloneBindingTarget(dst)
	if err := fn(bound); err != nil {
		return err
	}

	*dst = *bound
	return nil
}
