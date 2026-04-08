package bind

import (
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"sync"

	"github.com/kanata996/chix/errx"
	echo "github.com/labstack/echo/v5"
)

// 本文件负责 bind 包内部的宿主适配，把 `*http.Request` 环境转换成 Echo binder 能工作的最小运行时。
//
// 核心功能：
//   - 复用一个共享的 Echo 实例，提供 JSON serializer 和 ValueBinder 所需的上下文能力
//   - 把标准库 request 中的 path pattern / path values 转成 Echo 可识别的 `PathValues`
//   - 统一做入参检查和 Echo 错误到本仓库错误模型的映射
//
// 职责边界：
//   - 这是纯内部基础设施文件，不定义对外 binding 契约
//   - 它不决定 JSON 绑定规则本身，只为 `bind.go`、`value_binder.go`、`generic.go` 提供运行支撑
//   - 它也不做 DTO 级业务判断；这里只处理上下文构造、path 提取、目标合法性检查和错误归一
//
// 当前实现情况：
//   - 通过 `newContext` 构造临时 Echo context，使 bind 包可以直接复用 Echo v5 的大部分 binder 行为
//   - path 名称解析依赖 `http.Request.Pattern` 与 `SetPathValue`，覆盖普通 `{id}`、带正则后缀、以及 `...` 通配写法
//   - `mapEchoError` 会把 Echo 的 `BindingError`、`HTTPError` 映射为 bind/errx 体系，保证外部看到的错误契约稳定
//   - 后续做测试时，这个文件值得单独覆盖的重点是 path name 提取、nil 入参校验、以及 Echo 错误映射是否保持一致
var (
	echoOnce sync.Once
	echoApp  *echo.Echo
)

func sharedEcho() *echo.Echo {
	echoOnce.Do(func() {
		echoApp = echo.New()
	})
	return echoApp
}

func newContext(r *http.Request) *echo.Context {
	req := r
	if req == nil {
		req = httptest.NewRequest(http.MethodGet, "/", nil)
	}

	c := sharedEcho().NewContext(req, httptest.NewRecorder())
	c.SetPathValues(pathValuesFromRequest(req))
	return c
}

func pathValuesFromRequest(r *http.Request) echo.PathValues {
	if r == nil {
		return echo.PathValues{}
	}

	names := pathWildcardNames(r.Pattern)
	if len(names) == 0 {
		return echo.PathValues{}
	}
	values := make(echo.PathValues, 0, len(names))
	for _, name := range names {
		values = append(values, echo.PathValue{
			Name:  name,
			Value: r.PathValue(name),
		})
	}
	return values
}

func pathWildcardNames(pattern string) []string {
	pattern = strings.TrimSpace(pattern)
	if pattern == "" {
		return nil
	}

	names := make([]string, 0, 2)
	for i := 0; i < len(pattern); i++ {
		if pattern[i] != '{' {
			continue
		}

		end := strings.IndexByte(pattern[i+1:], '}')
		if end < 0 {
			break
		}

		token := strings.TrimSpace(pattern[i+1 : i+1+end])
		token = strings.TrimSuffix(token, "...")
		token, _, _ = strings.Cut(token, ":")
		token = strings.TrimSpace(token)
		if token != "" && token != "$" {
			names = append(names, token)
		}

		i += end + 1
	}

	return names
}

func validateRequestAndTarget(r *http.Request, target any) error {
	if r == nil {
		return errorsf("request must not be nil")
	}
	if target == nil {
		return errorsf("target must not be nil")
	}
	value := reflect.ValueOf(target)
	if value.Kind() != reflect.Pointer || value.IsNil() {
		return errorsf("target must be a non-nil pointer")
	}
	return nil
}

func mapEchoError(err error) error {
	if err == nil {
		return nil
	}

	var bindingErr *echo.BindingError
	if errors.As(err, &bindingErr) && bindingErr != nil {
		return NewBindingError(bindingErr.Field, bindingErr.Values, bindingErr.Message, errors.Unwrap(bindingErr))
	}

	var httpErr *echo.HTTPError
	if errors.As(err, &httpErr) && httpErr != nil {
		switch httpErr.Code {
		case http.StatusUnsupportedMediaType:
			return errx.NewHTTPErrorWithCause(httpErr.Code, CodeUnsupportedMediaType, httpErr.Message, errors.Unwrap(httpErr))
		default:
			return errx.NewHTTPErrorWithCause(httpErr.Code, "", httpErr.Message, errors.Unwrap(httpErr))
		}
	}

	return err
}

func errorsf(format string, args ...any) error {
	return fmt.Errorf("bind: "+format, args...)
}
