package reqx

import (
	"fmt"
	"net/http"
	"strings"
)

// Bind 按 Echo 的默认顺序绑定请求数据：path -> query(GET/DELETE) -> body。
func Bind[T any](r *http.Request, dst *T, opts ...BindOption) error {
	if r == nil {
		return fmt.Errorf("reqx: request must not be nil")
	}
	if dst == nil {
		return fmt.Errorf("reqx: destination must not be nil")
	}

	cfg := applyBindOptions(opts...)
	bound := cloneBindingTarget(dst)

	if err := bindTaggedValuesInPlace(r, bound, pathSource, bindValuesConfig{allowUnknownFields: true}); err != nil {
		return err
	}
	if shouldBindQueryParams(r.Method) {
		if err := bindTaggedValuesInPlace(r, bound, querySource, cfg.query); err != nil {
			return err
		}
	}

	bodyCfg := cfg.body
	bodyCfg.allowEmptyBody = true

	if err := bindJSONWithConfig(r, bound, bodyCfg, bodyBindMode{
		ignoreEmptyBody:            true,
		validateContentTypeOnEmpty: false,
	}); err != nil {
		return err
	}

	*dst = *bound
	return nil
}

func shouldBindQueryParams(method string) bool {
	switch strings.ToUpper(strings.TrimSpace(method)) {
	case http.MethodGet, http.MethodDelete:
		return true
	default:
		return false
	}
}
