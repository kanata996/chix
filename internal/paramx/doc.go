// Package paramx 提供基于 chi 的 path/query/header 参数最小读取器。
//
// 说明：
//   - 该包位于 internal 下，作为 chix 根包 facade 背后的 transport helper。
//   - 外部调用方默认应优先使用 chix.Path(r) / chix.Query(r) / chix.Header(r)，而不是直接依赖
//     internal 布局。
//
// 定位：
//   - 读取 chi 路由中的 path 参数。
//   - 读取 URL query 中的标量或列表参数。
//   - 读取 HTTP header 中的标量或列表参数。
//   - 对读取到的原始值做最小规范化（trim 空白）。
//   - 做基础类型解析，如 string / int / int16 / uuid / bool。
//   - 将 transport 层参数错误统一建模为 *reqx.Problem，供 resp.Problem 收口。
//
// 设计目标：
//   - handler 仍然显式掌握 path/query 参数入口，paramx 不接管 handler。
//   - 不做类似 echo binding 的大而全绑定，避免 DTO、路由参数和业务入口耦合。
//   - 让 path/query/header 参数的 trim、重复值处理和基础类型转换有稳定语义。
//   - 为 reqx.ValidatePath / ValidateQuery 提供更干净、更稳定的输入。
//   - 列表 query 统一采用重复 key 形式，例如 ?tag=a&tag=b。
//   - 列表 header 统一采用重复字段形式，不做逗号分隔拆分。
//
// 边界：
//   - 不做 struct binding，不使用反射自动填充 DTO。
//   - 不做 path/query/header/body 混绑。
//   - 不做业务校验，例如 oneof、范围校验、互斥关系、跨字段规则。
//   - 不写 HTTP 响应，不记录日志。
//   - 不替代 reqx.ValidateQuery / ValidatePath 的结构校验。
//   - 不负责默认值注入；默认值应由 handler 显式处理。
//
// 返回约定：
//   - 正常参数错误返回 *reqx.Problem，通常是 400。
//   - 编程错误返回普通 error，例如 nil request、空参数名。
//   - handler 拿到 *reqx.Problem 后应交给 resp.Problem；拿到普通 error 时也可继续
//     交给 resp.Problem，由边界按 internal 故障收口。
//
// 稳定语义：
//   - Path(r).Xxx(...) 默认是 required 语义；缺失或空值直接返回请求错误。
//   - Query(r).Xxx(...) 默认是 optional 语义；参数缺失时返回 ok=false。
//   - Header(r).Xxx(...) 默认是 optional 语义；参数缺失时返回 ok=false。
//   - optional query 参数若“出现但无效”，仍视为请求错误，不会偷偷按缺失处理。
//   - optional header 参数若“出现但无效”，仍视为请求错误，不会偷偷按缺失处理。
//   - scalar query 参数若重复出现（如 ?limit=10&limit=20），统一返回
//     multiple_values。
//   - scalar header 若重复出现（如 X-Limit: 10 / X-Limit: 20），统一返回
//     multiple_values。
//   - list query 参数使用重复 key 读取为切片（如 ?tag=a&tag=b -> []string{"a",
//     "b"}）。
//   - list header 使用重复字段读取为切片。
//   - bool 只接受小写 true / false。
//   - UUID 会在解析成功后统一规范化为 canonical 字符串。
//
// 包间协作：
//   - paramx 只负责“参数怎么取、怎么做最小解析、请求侧怎么错”。
//   - reqx 负责 body 解码、DTO 校验和更结构化的请求错误表达。
//   - resp 负责最终把 *reqx.Problem 写回 HTTP。
//
// 推荐用法：
//
//	path := uuidPathRequest{}
//	path.UUID, err := chix.Path(r).UUID("uuid")
//	if err != nil {
//	    resp.Problem(w, r, err)
//	    return
//	}
//	if err := reqx.ValidatePath(&path); err != nil {
//	    resp.Problem(w, r, err)
//	    return
//	}
//
//	query := listQueryRequest{}
//
//	limit, ok, err := chix.Query(r).Int("limit")
//	if err != nil {
//	    resp.Problem(w, r, err)
//	    return
//	}
//	if ok {
//	    query.Limit = &limit
//	}
//
//	tagUUIDs, ok, err := chix.Query(r).UUIDs("tag_uuid")
//	if err != nil {
//	    resp.Problem(w, r, err)
//	    return
//	}
//	if ok {
//	    query.TagUUIDs = tagUUIDs
//	}
//
//	if err := reqx.ValidateQuery(&query); err != nil {
//	    resp.Problem(w, r, err)
//	    return
//	}
//
// 反例：
//   - 不要把 paramx 做成反射式自动灌 DTO 的 binding 工具。
//   - 不要让 paramx 负责业务规则，例如 category_uuid 和 tag_uuid 互斥。
//   - 不要在 handler 中混用 paramx 和另一套并行的 path/query 错误结构。
//   - 不要把 list query 写成 CSV 语义后再假设 paramx 会自动拆分，例如
//     ?tag=a,b,c；当前列表语义是重复 key。
package paramx
