// 本文件职责：把 binding 子系统接入 runtime，并把内部绑定错误归一化为 runtime failure 模型。
// 定位：作为 runtime 与 binding 之间的薄适配层，不扩张公开 API。
package runtime

import (
	"net/http"

	"github.com/kanata996/chix/internal/binding"
	"github.com/kanata996/chix/internal/schema"
)

// bindInputWithSchema 使用给定 schema 执行输入绑定，并将 binding 子系统返回的错误归一化为 runtime 错误模型。
func bindInputWithSchema(r *http.Request, dst any, schema *schema.Schema) error {
	return normalizeBindError(binding.BindWithSchema(r, dst, schema))
}

// normalizeBindError 将已分类的 binding 错误映射为对应的 runtime 错误；未识别的错误保持原样返回。
func normalizeBindError(err error) error {
	switch binding.KindOf(err) {
	case binding.ErrorKindRequestShape:
		return newRequestShapeError(err)
	case binding.ErrorKindUnsupportedMediaType:
		return newUnsupportedMediaTypeError(err)
	case binding.ErrorKindInvalidRequest:
		return newInvalidRequestError(binding.DetailsOf(err))
	default:
		return err
	}
}
