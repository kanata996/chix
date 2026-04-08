// Package bind 提供独立的请求绑定能力。
//
// 它的目标是把 binding 从 reqx 中拆出，作为单独的职责层：
//   - 负责把 path/query/header/body 输入映射到目标值
//   - 提供 Echo v5 风格的 ValueBinder 与泛型 extractor
//   - 不负责 Normalize、RequestValidator 或字段校验
//
// 当前实现基于 *http.Request 做宿主适配，并对齐 Echo v5.1.0 的核心绑定行为；
// 默认 body binder 在当前版本刻意收窄为 JSON-only。
package bind
