package reqx

import (
	"net/http"
)

// DecodeValidateJSON 是常见 body DTO 流程的组合 helper。
// 它会先执行 DecodeJSON，再执行 ValidateBody。
// target 需要同时满足 DecodeJSON 和 ValidateBody 的约束，即非 nil `*struct`。
func DecodeValidateJSON(w http.ResponseWriter, r *http.Request, target any) error {
	if err := DecodeJSON(w, r, target); err != nil {
		return err
	}
	return ValidateBody(target)
}
