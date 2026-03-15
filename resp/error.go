package resp

import (
	"context"
	"github.com/kanata996/chix/errx"
	"log/slog"
	"net/http"
)

// Error 写业务/系统错误响应。
// 它优先保留 errx 内建语义；只有未识别时才进入 feature mapper。
func Error(w http.ResponseWriter, r *http.Request, err error, mapper errx.Mapper) {
	mapping, invalidMappingErr := resolveErrorMapping(err, mapper)

	if mapping.StatusCode >= http.StatusInternalServerError {
		attrs := []any{
			"error", err,
			"error_chain", errx.FormatChain(err),
		}
		if invalidMappingErr != nil {
			attrs = append(attrs, "invalid_mapping", true)
			attrs = append(attrs, "invalid_mapping_error", invalidMappingErr)
		}
		slog.ErrorContext(requestContext(r), "internal error", attrs...)
	}

	writeErrorEnvelope(w, mapping, nil)
}

// requestContext 统一兜底 nil request，保证边界日志始终有合法 context。
func requestContext(r *http.Request) context.Context {
	if r == nil {
		return context.Background()
	}
	return r.Context()
}

// resolveErrorMapping 统一处理 Error 的映射优先级：
// errx 内建语义 -> feature mapper -> internal fallback。
func resolveErrorMapping(err error, mapper errx.Mapper) (errx.Mapping, error) {
	if mapping, ok := errx.Lookup(err); ok {
		return mapping, nil
	}
	if mapper == nil {
		return internalMapping, nil
	}

	mapping := mapper.Map(err)
	if validationErr := mapping.Validate(); validationErr != nil {
		return internalMapping, validationErr
	}
	return mapping, nil
}
