# Request Binding

本文档描述当前公开的请求绑定契约。

- 核心 binding API 位于 `bind` 包
- 根包 `chix` re-export 最常用的 `Bind*`
- `reqx.BindAndValidate*` 是组合层：`Bind -> Normalize -> ValidateRequest -> validator/v10`

## 入口选择

- 只做 binding：`bind.Bind(...)`、`bind.BindBody(...)`、`bind.BindQueryParams(...)`、`bind.BindPathValues(...)`、`bind.BindHeaders(...)`
- 根包薄封装：`chix.Bind(...)`、`chix.BindBody(...)` 等
- binding + request-rules + validate：`chix.BindAndValidate(...)` / `reqx.BindAndValidate(...)`
- 高级 binding 能力：`bind.ValueBinder`、`bind.QueryParam(...)` 等

binding 层默认直接对齐 Echo 的 empty-body no-op 语义；是否“必须提交 body”不再由 binding API 定义，而是交给 `RequestValidator` 或业务层规则。

## 综合绑定顺序

`Bind(...)` 的顺序固定：

1. path
2. query，仅 `GET` / `DELETE` / `HEAD`
3. body

后阶段可以覆盖前阶段写入到同一字段的值。

任一阶段失败时，前面已经成功写入的阶段结果会保留；默认 binder 不做整对象回滚。

## Body 契约

### `BindBody(...)`

- 只要 `Content-Length == 0`，直接视为 no-op
- 这个 no-op 发生在 `Content-Type` 检查之前
- 非空 body 当前只支持 `application/json`
- 默认 binder 不接受 `application/*+json`
- `application/json` 的解码语义跟随 Echo 默认 JSON serializer / 标准 `json.Decoder`
- `BindBody(...)` 可以绑定到标准 decoder 支持的非 nil 指针目标，包括 struct、slice、map 等

### `BindAndValidateBody(...)`

- 先执行 `BindBody(...)`
- 然后执行 `Normalize() -> ValidateRequest() -> validator/v10`
- 因为组合层要运行 DTO hook 和结构校验，目标必须是非 nil 的 `*struct`

如果 DTO 自身有必填字段，空 body 最终通常会在字段校验阶段返回字段缺失错误；如果业务要求“即使字段全是 optional 也必须提交 body”，请实现 `ValidateRequest(*http.Request) error` 并在其中调用 `RequireBody(r)`。

## Query / Path / Header 契约

- 只绑定显式声明了对应 tag 的字段
- 缺失输入不会清空目标对象已有值
- 默认绑定错误返回 `400 bad_request`
- 同一输入源重复给单值字段时，默认取首值
- 目标支持 Echo 同等 map 语义：`map[string]string`、`map[string][]string`、`map[string]any`
- 对不受支持的 scalar 指针目标，query/path/header 单源 binder 会像 Echo 一样直接 no-op

## Request Rules / 校验 / Normalize

- `BindAndValidate*` 会按 `Bind -> Normalize() -> ValidateRequest() -> validator/v10` 的顺序执行
- 绑定失败时不会进入校验阶段
- `ValidateRequest()` 返回的错误会直接终止后续字段校验
- mixed-source 的 `BindAndValidate(...)` 字段校验错误统一标记为 `errors[].in = "request"`

示例：

```go
type createAccountRequest struct {
	OrgID string `param:"org_id"`
	Name  string `json:"name" validate:"required"`
}

func (*createAccountRequest) ValidateRequest(r *http.Request) error {
	return chix.RequireBody(r)
}

func handler(w http.ResponseWriter, r *http.Request) {
	var req createAccountRequest
	if err := chix.BindAndValidate(r, &req); err != nil {
		_ = chix.WriteError(w, r, err)
		return
	}
}
```

## 错误语义

常见 body 错误：

- `Content-Length == 0`：binding 层 no-op；是否报错取决于后续 `RequestValidator`、字段校验或业务规则
- 非法 JSON：`400 invalid_json`
- 非支持的 `Content-Type`：`415 unsupported_media_type`

binding 错误默认不输出 `errors[]` violation 列表；`errors[]` 主要由请求规则层和字段校验层产出。
