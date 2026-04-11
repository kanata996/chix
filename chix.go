package chix

import (
	"net/http"
)

// WriteError 使用默认的 chi 风格响应器预设写出结构化错误响应。
func WriteError(w http.ResponseWriter, r *http.Request, err error) error {
	return defaultErrorResponder.Respond(w, r, err)
}
