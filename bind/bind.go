package bind

import (
	"errors"
	"net/http"
	"strings"

	echo "github.com/labstack/echo/v5"
)

// 本文件是 bind 包的主入口，定义默认 binder 的公开 API 和当前版本的 body 绑定策略。
//
// 核心功能：
//   - 暴露 `Bind`、`BindBody`、`BindPathValues`、`BindQueryParams`、`BindHeaders`
//   - 定义默认综合绑定顺序：path -> query(GET/DELETE/HEAD) -> body
//   - 承载当前版本唯一的 body 绑定实现 `bindJSONBody`
//
// 职责边界：
//   - 这里只负责“把 HTTP 输入写入目标对象”，不负责 Normalize、请求规则校验或字段校验
//   - path/query/header 的具体字段写入逻辑直接复用 Echo v5 binder
//   - body 绑定在当前版本刻意收窄为 JSON-only，不处理 XML、form、multipart 等其它 body 语义
//
// 当前实现情况：
//   - 综合绑定顺序和覆盖关系对齐 Echo v5.1.0：后阶段允许覆盖前阶段已写入的字段
//   - `BindBody` 先遵循 Echo 的 empty-body no-op 语义，再检查 `Content-Type`
//   - 非空 body 仅接受严格的 `application/json`；非法 JSON 统一映射为 `400 invalid_json`
//   - 这里是后续补齐 JSON 相关单元测试时最核心的行为基线，应优先覆盖顺序、空 body、媒体类型和解码失败几类场景
//
// Binder 定义默认请求绑定器接口。
type Binder interface {
	Bind(r *http.Request, target any) error
}

// DefaultBinder 是默认绑定器实现。
type DefaultBinder struct{}

// BindUnmarshaler 允许字段从单个字符串值自定义解码。
type BindUnmarshaler = echo.BindUnmarshaler

// BindPathValues 只绑定 path 参数。
func BindPathValues(r *http.Request, target any) error {
	if err := validateRequestAndTarget(r, target); err != nil {
		return err
	}
	return mapEchoError(echo.BindPathValues(newContext(r), target))
}

// BindQueryParams 只绑定 query 参数。
func BindQueryParams(r *http.Request, target any) error {
	if err := validateRequestAndTarget(r, target); err != nil {
		return err
	}
	return mapEchoError(echo.BindQueryParams(newContext(r), target))
}

// BindBody 只绑定请求体。
func BindBody(r *http.Request, target any) error {
	if err := validateRequestAndTarget(r, target); err != nil {
		return err
	}
	return bindJSONBody(newContext(r), target)
}

// BindHeaders 只绑定 header。
func BindHeaders(r *http.Request, target any) error {
	if err := validateRequestAndTarget(r, target); err != nil {
		return err
	}
	return mapEchoError(echo.BindHeaders(newContext(r), target))
}

// Bind 按默认顺序绑定：path -> query(GET/DELETE/HEAD) -> body。
func Bind(r *http.Request, target any) error {
	if err := validateRequestAndTarget(r, target); err != nil {
		return err
	}
	if err := BindPathValues(r, target); err != nil {
		return err
	}

	method := r.Method
	if method == http.MethodGet || method == http.MethodDelete || method == http.MethodHead {
		if err := BindQueryParams(r, target); err != nil {
			return err
		}
	}

	return BindBody(r, target)
}

// Bind 实现 Binder 接口。
func (b *DefaultBinder) Bind(r *http.Request, target any) error {
	return Bind(r, target)
}

func bindJSONBody(c *echo.Context, target any) error {
	req := c.Request()
	if req.ContentLength == 0 {
		return nil
	}

	base, _, _ := strings.Cut(req.Header.Get(echo.HeaderContentType), ";")
	if strings.TrimSpace(base) != echo.MIMEApplicationJSON {
		return unsupportedMediaTypeError()
	}

	if err := c.Echo().JSONSerializer.Deserialize(c, target); err != nil {
		var httpErr *echo.HTTPError
		if errors.As(err, &httpErr) && httpErr != nil && httpErr.Code == http.StatusBadRequest {
			if cause := errors.Unwrap(httpErr); cause != nil {
				return invalidJSONError(cause)
			}
		}
		return invalidJSONError(err)
	}

	return nil
}
