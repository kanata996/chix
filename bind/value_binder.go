package bind

import (
	"net/http"

	echo "github.com/labstack/echo/v5"
)

// 本文件暴露 Echo 风格的 fluent ValueBinder，让调用方在 `*http.Request` 上按字段逐个绑定值。
//
// 核心功能：
//   - 直接复用 Echo v5 的 `ValueBinder`
//   - 提供 query/path/form 三类输入源的 binder 构造函数
//   - 统一把 ValueBinder 内部的错误工厂替换为 `NewBindingError`
//
// 职责边界：
//   - 这里不处理 request body，更不参与 JSON 反序列化
//   - 它的职责是把 Echo 已有的值级绑定能力适配到 bind 包的 `*http.Request` 宿主上
//   - 具体值解析、FailFast、Must/非 Must 行为都继续由 Echo 原生实现负责
//
// 当前实现情况：
//   - 这是一个非常薄的适配层，核心行为基本等价于 Echo v5，只改了上下文来源和错误类型
//   - 因为 `ValueBinder` 直接是类型别名，后续补测试时应把重点放在“构造出来的 binder 行为是否与 Echo 一致”，而不是重复测试 Echo 自身实现
//   - 如果未来要扩大 body 支持范围，这个文件通常不需要改；它主要验证单值/多值输入源和错误归一是否稳定
//
// ValueBinder 复用 Echo v5 的值级 binder。
type ValueBinder = echo.ValueBinder

// QueryParamsBinder 创建 query 参数 binder。
func QueryParamsBinder(r *http.Request) *ValueBinder {
	b := echo.QueryParamsBinder(newContext(r))
	b.ErrorFunc = NewBindingError
	return b
}

// PathValuesBinder 创建 path 参数 binder。
func PathValuesBinder(r *http.Request) *ValueBinder {
	b := echo.PathValuesBinder(newContext(r))
	b.ErrorFunc = NewBindingError
	return b
}

// FormFieldBinder 创建 form field binder。
func FormFieldBinder(r *http.Request) *ValueBinder {
	b := echo.FormFieldBinder(newContext(r))
	b.ErrorFunc = NewBindingError
	return b
}
