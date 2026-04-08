# Request Binding

本文档描述 `chix` / `reqx` 当前公开的请求绑定契约，重点覆盖输入来源、执行顺序，以及默认 Echo 风格 body 语义。

## 入口选择

- `Bind(...)`：综合绑定，顺序为 `path -> query(GET/DELETE/HEAD) -> body`
- `Binder` / `DefaultBinder`：默认 binder 的接口与实现
- `BindUnmarshaler`：字段级单值自定义解码扩展点
- `RequestValidator`：DTO 级请求规则扩展点
- `RequireBody(...)`：内置的 body-required helper
- `BindAndValidate(...)`：综合绑定后再执行请求级规则与 `validator/v10` 校验
- `BindBody(...)` / `BindAndValidateBody(...)`：只处理 body
- `BindQueryParams(...)` / `BindAndValidateQuery(...)`：只处理 query
- `BindPathValues(...)` / `BindAndValidatePath(...)`：只处理 path
- `BindHeaders(...)` / `BindAndValidateHeaders(...)`：只处理 header

binding 层默认直接对齐 Echo 的 empty-body no-op 语义；是否“必须提交 body”不再由 binding API 定义，而是交给 `RequestValidator` 或业务层规则。`BindAndValidate*` 属于组合层便利 API，不属于核心 binding 标准面。

## 综合绑定顺序

`Bind(...)` 与 `BindAndValidate(...)` 的顺序固定：

1. path
2. query，仅 `GET` / `DELETE` / `HEAD`
3. body

后阶段可以覆盖前阶段写入到同一字段的值。

任一阶段失败时，前面已经成功写入的阶段结果会保留；默认 binder 不再做整对象回滚。

## Body 契约

### `BindBody(...)` 和 `BindAndValidateBody(...)`

- 默认会在 `Content-Length == 0` 时把 body 视为 no-op
- 非空 body 当前只支持 `application/json`
- `application/json` 默认遵循标准 decoder 语义；默认 binder 不接受 `application/*+json`
- body 超过大小限制时返回 `413 request_too_large`

如果 DTO 自身有必填字段，空 body 最终通常会在字段校验阶段返回字段缺失错误；如果业务要求“即使字段全是 optional 也必须提交 body”，请实现 `ValidateRequest(*http.Request) error` 并在其中调用 `RequireBody(r)` 或自定义请求规则。

### `Bind(...)` 和 `BindAndValidate(...)` 中的 body 阶段

- 只要 `Content-Length == 0`，body 阶段就会视为 no-op
- 这个 no-op 发生在 `Content-Type` 校验之前
- mixed-source 路由的字段是否必填，交给字段校验层；如果还需要更严格的“body 必须存在”业务语义，需通过 `RequestValidator` 单独实现

## Query / Path / Header 契约

- query、path、header 只绑定显式声明了对应 tag 的字段
- 缺失输入不会清空目标对象已有值
- 默认绑定错误返回 `400 bad_request`
- 同一输入源重复给单值字段时，默认取首值

## Request Rules / 校验 / Normalize

- `BindAndValidate*` 会按 `Bind -> Normalize() -> ValidateRequest() -> validator/v10` 的顺序执行
- 绑定失败时不会进入校验阶段
- `ValidateRequest()` 返回的错误会直接终止后续字段校验
- 字段校验错误会尽量使用对应来源的 tag 名作为 `errors[].field`

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
- body 超过大小限制：`413 request_too_large`

binding 错误默认不再输出 `errors[]` violation 列表；`errors[]` 主要由请求规则层和字段校验层产出。
