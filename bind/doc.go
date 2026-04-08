// Package bind 为基于 net/http 的 JSON API 提供请求绑定能力。
//
// 它只负责把 HTTP 输入映射到目标值，不处理 Normalize、请求级规则或字段校验。
//
// 公开 API：
//   - 绑定入口：Bind、BindBody、BindQueryParams、BindPathValues、BindHeaders
//   - binder 相关类型：Binder、DefaultBinder、BindUnmarshaler
//   - 公开错误码常量：CodeInvalidJSON、CodeUnsupportedMediaType、CodeRequestTooLarge
//
// 默认 Bind 顺序固定为：path -> query(GET/DELETE/HEAD) -> body。
// header 不参与默认 Bind。
package bind
