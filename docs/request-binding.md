# Request Binding

本文档描述 `chix` / `reqx` 当前公开的请求绑定契约，重点覆盖输入来源、执行顺序，以及 JSON body 的严格语义。

## 入口选择

- `Bind(...)`：综合绑定，顺序为 `path -> query(GET/DELETE) -> body`
- `BindAndValidate(...)`：综合绑定后再执行 `validator/v10` 校验
- `BindBody(...)` / `BindAndValidateBody(...)`：只处理 JSON body
- `BindQueryParams(...)` / `BindAndValidateQuery(...)`：只处理 query
- `BindPathValues(...)` / `BindAndValidatePath(...)`：只处理 path
- `BindHeaders(...)` / `BindAndValidateHeaders(...)`：只处理 header

如果路由明确要求客户端必须提交 JSON body，优先使用 `BindBody(...)` 或 `BindAndValidateBody(...)`。

如果路由允许 body 缺省，或你要复用 Echo 风格的综合绑定顺序，使用 `Bind(...)` 或 `BindAndValidate(...)`。

## 综合绑定顺序

`Bind(...)` 与 `BindAndValidate(...)` 的顺序固定：

1. path
2. query，仅 `GET` / `DELETE`
3. JSON body

后阶段可以覆盖前阶段写入到同一字段的值。

任一阶段失败时，目标对象保持原值，不会留下部分更新。

## JSON Body 契约

### `BindBody(...)` 和 `BindAndValidateBody(...)`

- body 必须非空
- 空 body 或纯空白 body 返回 `400 invalid_json`
- 如果空 body 同时显式声明了非 JSON `Content-Type`，优先返回 `415 unsupported_media_type`
- 非空 body 只接受 `application/json` 和 `application/*+json`
- 顶层 body 必须是 JSON object
- 顶层 `null`、数组、数字、字符串、布尔值都会被拒绝
- body 超过大小限制时返回 `413 request_too_large`

这两个入口适合“请求体是必需项”的写接口。

### `Bind(...)` 和 `BindAndValidate(...)` 中的 body 阶段

- 空 body 会被视为 no-op
- 空 body 不会触发该阶段的 `Content-Type` 校验
- 非空 body 仍然遵守 JSON `Content-Type` 和顶层 object 规则

这两个入口适合“body 可选”的综合绑定场景。

## Query / Path / Header 契约

- query、path、header 只绑定显式声明了对应 tag 的字段
- 缺失输入不会清空目标对象已有值
- 同一输入源重复给单值字段时返回 `422 invalid_request`
- path / header 默认忽略未声明字段
- query 当前也默认忽略未声明字段

## 校验与 Normalize

- `BindAndValidate*` 会先绑定，再调用 `Normalize()`，最后执行校验
- 绑定失败时不会进入校验阶段
- 校验错误会尽量使用对应来源的 tag 名作为 `errors[].field`

示例：

```go
type createAccountRequest struct {
	OrgID string `param:"org_id"`
	Name  string `json:"name" validate:"required"`
}

func handler(w http.ResponseWriter, r *http.Request) {
	var req createAccountRequest
	if err := chix.BindAndValidate(r, &req); err != nil {
		_ = chix.WriteError(w, r, err)
		return
	}
}
```

如果这个路由要求 body 必填，改成：

```go
type createAccountBody struct {
	Name string `json:"name" validate:"required"`
}

func handler(w http.ResponseWriter, r *http.Request) {
	var body createAccountBody
	if err := chix.BindAndValidateBody(r, &body); err != nil {
		_ = chix.WriteError(w, r, err)
		return
	}
}
```

## 错误语义

常见 body 错误：

- 空 body：`400 invalid_json`
- 非法 JSON：`400 invalid_json`
- 多个顶层 JSON 值：`400 invalid_json`
- 非 JSON `Content-Type`：`415 unsupported_media_type`
- 顶层不是 object：`422 invalid_request`

字段级错误会出现在 `errors[]` 中，字段为 `field`、`in`、`code`、`detail`。
