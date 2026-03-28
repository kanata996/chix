// 本文件职责：把 binding 子系统接入 runtime，并把内部绑定错误归一化为 runtime failure 模型。
// 定位：作为 runtime 与 binding 之间的薄适配层，不扩张公开 API。
package runtime

import (
	"net/http"

	"github.com/kanata996/chix/internal/binding"
	"github.com/kanata996/chix/internal/inputschema"
)

func bindInputWithSchema(r *http.Request, dst any, schema *inputschema.Schema) error {
	return normalizeBindError(binding.BindWithSchema(r, dst, schema))
}

func normalizeBindError(err error) error {
	switch binding.KindOf(err) {
	case binding.ErrorKindRequestShape:
		return newRequestShapeError(err)
	case binding.ErrorKindUnsupportedMediaType:
		return newUnsupportedMediaTypeError(err)
	default:
		return err
	}
}
