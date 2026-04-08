package bind

import (
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/kanata996/chix/errx"
)

// 本文件定义 bind 包的错误模型，负责把 Echo binding 过程中的失败表达成仓库统一可消费的错误类型。
//
// 核心功能：
//   - 定义 `BindingError`，承载字段名、原始输入值和底层 HTTP 错误
//   - 定义当前 body 绑定会用到的错误码：`invalid_json`、`unsupported_media_type`
//   - 提供错误字符串、JSON 序列化和 unwrap 语义，方便上层写错误响应或断言测试
//
// 职责边界：
//   - 这里不参与实际绑定流程，也不做 JSON 解码；它只负责“错误长什么样”
//   - 字段级绑定失败统一走 `BindingError`
//   - body 场景下的协议错误和解码错误统一落到 `errx.HTTPError` 语义上，便于与 `resp.WriteError` 等基础设施协作
//
// 当前实现情况：
//   - `BindingError` 的公开 JSON 形状刻意对齐 Echo，只暴露 `field` 和 `message`，保留 `Values` 供内部断言和调试使用
//   - `invalidJSONError` 与 `unsupportedMediaTypeError` 是当前 JSON-only body binder 的固定错误出口
//   - 后续补单元测试时，这个文件应重点验证错误码、状态码、`Error()` 字符串、`MarshalJSON()` 结果以及 `Unwrap()` 链是否稳定
const (
	CodeInvalidJSON          = "invalid_json"
	CodeUnsupportedMediaType = "unsupported_media_type"
)

// BindingError 表示参数绑定失败。
type BindingError struct {
	Field string `json:"field"`
	// Values 保留原始参数值，和 Echo 一样不直接暴露到 JSON。
	Values []string `json:"-"`
	// HTTPError 提供和仓库其余部分兼容的 400 错误语义。
	*errx.HTTPError
}

// NewBindingError 创建新的 BindingError。
func NewBindingError(sourceParam string, values []string, message string, err error) error {
	clonedValues := append([]string(nil), values...)
	return &BindingError{
		Field:     sourceParam,
		Values:    clonedValues,
		HTTPError: errx.NewHTTPErrorWithCause(http.StatusBadRequest, "", message, err),
	}
}

func invalidJSONError(err error) error {
	return errx.NewHTTPErrorWithCause(
		http.StatusBadRequest,
		CodeInvalidJSON,
		"request body must be valid JSON",
		err,
	)
}

func unsupportedMediaTypeError() error {
	return errx.NewHTTPError(
		http.StatusUnsupportedMediaType,
		CodeUnsupportedMediaType,
		"Content-Type must be application/json",
	)
}

// Error 返回和 Echo 接近的错误字符串。
func (be *BindingError) Error() string {
	if be == nil {
		return ""
	}

	message := be.Detail()
	if cause := be.HTTPError.Unwrap(); cause != nil {
		return fmt.Sprintf("code=%d, message=%v, err=%v, field=%s", be.Status(), message, cause, be.Field)
	}
	return fmt.Sprintf("code=%d, message=%v, field=%s", be.Status(), message, be.Field)
}

// Unwrap 让 BindingError 继续暴露底层 HTTPError。
func (be *BindingError) Unwrap() error {
	if be == nil {
		return nil
	}
	return be.HTTPError
}

// MarshalJSON 对齐 Echo BindingError 的公开 JSON 形状。
func (be *BindingError) MarshalJSON() ([]byte, error) {
	if be == nil {
		return json.Marshal(struct{}{})
	}
	return json.Marshal(struct {
		Field   string `json:"field"`
		Message string `json:"message"`
	}{
		Field:   be.Field,
		Message: be.Detail(),
	})
}
