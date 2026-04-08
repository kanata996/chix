package bind

import (
	"errors"
	"fmt"
	"net/http"
	"net/url"

	echo "github.com/labstack/echo/v5"
)

// 本文件提供泛型 extractor / parser API，补齐 Echo v5 在参数读取层的另一组核心能力。
//
// 核心功能：
//   - 暴露 `PathParam`、`QueryParam`、`FormValue` 等泛型读取入口
//   - 暴露 `ParseValue`、`ParseValues` 及其带默认值版本，直接复用 Echo 的类型解析逻辑
//   - 在读取失败时把底层错误归一成 bind 包自己的 `BindingError`
//
// 职责边界：
//   - 这里不负责 JSON body 绑定；它面向的是 path/query/form 这类“离散字符串值”输入
//   - 它也不重新实现类型转换算法，而是把解析职责继续委托给 Echo v5
//   - 本文件更偏“便捷读取 API”，和 `bind.go` 里的结构体综合绑定入口互补
//
// 当前实现情况：
//   - 大部分函数都是薄包装，目标是让 `*http.Request` 宿主下的行为尽量直接对齐 Echo v5
//   - path 参数读取依赖 `adapter.go` 提供的 path value 适配；query/form 则复用标准库和 Echo 的现成逻辑
//   - 对后续测试来说，这个文件适合做 parity 型用例：缺失键、空值、类型转换失败、默认值分支和时间布局选项
//   - 虽然本文件不直接处理 JSON，但它和 `bind.go` 一起构成 bind 包的核心公开能力，补测试时通常需要一起覆盖
//
// ErrNonExistentKey 表示键不存在。
var ErrNonExistentKey = echo.ErrNonExistentKey

type TimeLayout = echo.TimeLayout
type TimeOpts = echo.TimeOpts

const (
	TimeLayoutUnixTime      = echo.TimeLayoutUnixTime
	TimeLayoutUnixTimeMilli = echo.TimeLayoutUnixTimeMilli
	TimeLayoutUnixTimeNano  = echo.TimeLayoutUnixTimeNano
)

// PathParam 读取并解析 path 参数。
func PathParam[T any](r *http.Request, paramName string, opts ...any) (T, error) {
	for _, pv := range pathValuesFromRequest(r) {
		if pv.Name != paramName {
			continue
		}

		v, err := ParseValue[T](pv.Value, opts...)
		if err != nil {
			return v, NewBindingError(paramName, []string{pv.Value}, "path value", err)
		}
		return v, nil
	}

	var zero T
	return zero, ErrNonExistentKey
}

// PathParamOr 读取并解析 path 参数，不存在或为空时返回默认值。
func PathParamOr[T any](r *http.Request, paramName string, defaultValue T, opts ...any) (T, error) {
	for _, pv := range pathValuesFromRequest(r) {
		if pv.Name != paramName {
			continue
		}

		v, err := ParseValueOr[T](pv.Value, defaultValue, opts...)
		if err != nil {
			return v, NewBindingError(paramName, []string{pv.Value}, "path value", err)
		}
		return v, nil
	}

	return defaultValue, nil
}

// QueryParam 读取并解析单个 query 参数。
func QueryParam[T any](r *http.Request, key string, opts ...any) (T, error) {
	values, ok := queryParams(r)[key]
	if !ok {
		var zero T
		return zero, ErrNonExistentKey
	}

	value := values[0]
	v, err := ParseValue[T](value, opts...)
	if err != nil {
		return v, NewBindingError(key, []string{value}, "query param", err)
	}
	return v, nil
}

// QueryParamOr 读取并解析单个 query 参数，不存在或为空时返回默认值。
func QueryParamOr[T any](r *http.Request, key string, defaultValue T, opts ...any) (T, error) {
	values, ok := queryParams(r)[key]
	if !ok || len(values) == 0 {
		return defaultValue, nil
	}

	value := values[0]
	v, err := ParseValueOr[T](value, defaultValue, opts...)
	if err != nil {
		return v, NewBindingError(key, []string{value}, "query param", err)
	}
	return v, nil
}

// QueryParams 读取并解析多个 query 参数值。
func QueryParams[T any](r *http.Request, key string, opts ...any) ([]T, error) {
	values, ok := queryParams(r)[key]
	if !ok {
		return nil, ErrNonExistentKey
	}

	result, err := ParseValues[T](values, opts...)
	if err != nil {
		return nil, NewBindingError(key, values, "query params", err)
	}
	return result, nil
}

// QueryParamsOr 读取并解析多个 query 参数值，不存在时返回默认值。
func QueryParamsOr[T any](r *http.Request, key string, defaultValue []T, opts ...any) ([]T, error) {
	values, ok := queryParams(r)[key]
	if !ok {
		return defaultValue, nil
	}

	result, err := ParseValuesOr[T](values, defaultValue, opts...)
	if err != nil {
		return nil, NewBindingError(key, values, "query params", err)
	}
	return result, nil
}

// FormValue 读取并解析单个 form 值。
func FormValue[T any](r *http.Request, key string, opts ...any) (T, error) {
	formValues, err := formValues(r)
	if err != nil {
		var zero T
		return zero, fmt.Errorf("failed to parse form value, key: %s, err: %w", key, err)
	}

	values, ok := formValues[key]
	if !ok {
		var zero T
		return zero, ErrNonExistentKey
	}
	if len(values) == 0 {
		var zero T
		return zero, nil
	}

	value := values[0]
	v, err := ParseValue[T](value, opts...)
	if err != nil {
		return v, NewBindingError(key, []string{value}, "form value", err)
	}
	return v, nil
}

// FormValueOr 读取并解析单个 form 值，不存在或为空时返回默认值。
func FormValueOr[T any](r *http.Request, key string, defaultValue T, opts ...any) (T, error) {
	formValues, err := formValues(r)
	if err != nil {
		var zero T
		return zero, fmt.Errorf("failed to parse form value, key: %s, err: %w", key, err)
	}

	values, ok := formValues[key]
	if !ok || len(values) == 0 {
		return defaultValue, nil
	}

	value := values[0]
	v, err := ParseValueOr[T](value, defaultValue, opts...)
	if err != nil {
		return v, NewBindingError(key, []string{value}, "form value", err)
	}
	return v, nil
}

// FormValues 读取并解析多个 form 值。
func FormValues[T any](r *http.Request, key string, opts ...any) ([]T, error) {
	formValues, err := formValues(r)
	if err != nil {
		return nil, fmt.Errorf("failed to parse form values, key: %s, err: %w", key, err)
	}

	values, ok := formValues[key]
	if !ok {
		return nil, ErrNonExistentKey
	}

	result, err := ParseValues[T](values, opts...)
	if err != nil {
		return nil, NewBindingError(key, values, "form values", err)
	}
	return result, nil
}

// FormValuesOr 读取并解析多个 form 值，不存在时返回默认值。
func FormValuesOr[T any](r *http.Request, key string, defaultValue []T, opts ...any) ([]T, error) {
	formValues, err := formValues(r)
	if err != nil {
		return nil, fmt.Errorf("failed to parse form values, key: %s, err: %w", key, err)
	}

	values, ok := formValues[key]
	if !ok {
		return defaultValue, nil
	}

	result, err := ParseValuesOr[T](values, defaultValue, opts...)
	if err != nil {
		return nil, NewBindingError(key, values, "form values", err)
	}
	return result, nil
}

// ParseValue 解析单个值。
func ParseValue[T any](value string, opts ...any) (T, error) {
	return echo.ParseValue[T](value, opts...)
}

// ParseValueOr 解析单个值；为空时返回默认值。
func ParseValueOr[T any](value string, defaultValue T, opts ...any) (T, error) {
	return echo.ParseValueOr[T](value, defaultValue, opts...)
}

// ParseValues 解析多个值。
func ParseValues[T any](values []string, opts ...any) ([]T, error) {
	return echo.ParseValues[T](values, opts...)
}

// ParseValuesOr 解析多个值；为空时返回默认值。
func ParseValuesOr[T any](values []string, defaultValue []T, opts ...any) ([]T, error) {
	return echo.ParseValuesOr[T](values, defaultValue, opts...)
}

func queryParams(r *http.Request) url.Values {
	if r == nil || r.URL == nil {
		return url.Values{}
	}
	return r.URL.Query()
}

func formValues(r *http.Request) (url.Values, error) {
	if r == nil {
		return nil, errorsf("request must not be nil")
	}
	return newContext(r).FormValues()
}

func bindingErrorDetails(err error) (field string, values []string, message string, ok bool) {
	if err == nil {
		return "", nil, "", false
	}

	var bindingErr *BindingError
	if errors.As(err, &bindingErr) && bindingErr != nil {
		return bindingErr.Field, bindingErr.Values, bindingErr.Detail(), true
	}
	return "", nil, "", false
}
