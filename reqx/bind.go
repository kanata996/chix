package reqx

import (
	"net/http"
	"strings"
)

// Bind 按 Echo 的默认顺序绑定请求数据：path -> query(GET/DELETE) -> body。
func Bind[T any](r *http.Request, dst *T, opts ...BindOption) error {
	cfg := applyBindOptions(opts...)

	if err := bindTaggedValues(r, dst, pathSource, bindValuesConfig{allowUnknownFields: true}); err != nil {
		return err
	}
	if shouldBindQueryParams(r.Method) {
		if err := bindTaggedValues(r, dst, querySource, cfg.query); err != nil {
			return err
		}
	}

	bodyCfg := cfg.body
	bodyCfg.allowEmptyBody = true

	return bindJSONWithConfig(r, dst, bodyCfg, bodyBindMode{
		ignoreEmptyBody:            true,
		validateContentTypeOnEmpty: false,
	})
}

func shouldBindQueryParams(method string) bool {
	switch strings.ToUpper(strings.TrimSpace(method)) {
	case http.MethodGet, http.MethodDelete:
		return true
	default:
		return false
	}
}
