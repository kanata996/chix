// Package reqx 提供 HTTP 请求边界最小但统一的一组能力。
//
// 定位：
//   - 解析 JSON body。
//   - 校验 body/query/path DTO。
//   - 统一建模请求侧错误（*Problem）。
//   - 为 resp.Problem 提供稳定、可收口的输入。
//
// 设计目标：
//   - handler 仍然显式掌握请求入口参数，reqx 只负责通用边界动作。
//   - 不再做大而全的 binding 抽象，避免把 DTO、路由参数和业务入口耦合在一起。
//   - 请求错误统一表达为 Problem + Details，便于 resp.Problem 收口。
//
// 边界：
//   - 做媒体类型、大小限制、JSON 语法/类型检查。
//   - 做 validator/v10 校验，并在校验前调用 DTO 的 Normalize。
//   - 不做业务错误映射。
//   - 不写 HTTP 响应。
//   - 不负责 path/query 的提取，参数提取仍放在 handler。
//
// 返回约定：
//   - 正常请求错误返回 *Problem，例如 400/413/415/422。
//   - 编程错误返回普通 error，例如 nil writer / nil request / 非法 decode target /
//     非法 validate target；可通过 errors.Is 判断 ErrNilRequest / ErrNilTarget /
//     ErrInvalidDecodeTarget / ErrInvalidValidateTarget。
//   - handler 拿到 *Problem 时应交给 resp.Problem；拿到普通 error 时仍可继续交给
//     resp.Problem，由边界按 internal 故障收口。
//
// 稳定输入/输出：
//   - DecodeJSON 统一处理 content-type、payload too large、unknown field、
//     trailing data、JSON 语法错误和类型错误；需要非 nil 指针 target。
//   - DecodeJSONWith 可按 endpoint 覆盖默认大小限制、unknown field 策略或
//     content-type 检查。
//   - ValidateBody / ValidateQuery / ValidatePath 只接受非 nil `*struct`。
//     ValidateBody 对 body DTO 返回 422；ValidateQuery / ValidatePath 对 query/path
//     DTO 返回 400。
//   - Detail 只保留 in/field/code 三个稳定字段，不负责展示型 message。
//
// 包间协作：
//   - reqx 只负责“请求是怎么错的”。
//   - errx 只负责“业务/系统错误映射成什么响应语义”。
//   - resp 负责最终把 *Problem 或 errx.Mapping 写回 HTTP。
//
// 推荐用法：
//
//	var body CreateRequest
//
//	if err := reqx.DecodeJSON(w, r, &body); err != nil {
//	    resp.Problem(w, r, err)
//	    return
//	}
//	if err := reqx.ValidateBody(&body); err != nil {
//	    resp.Problem(w, r, err)
//	    return
//	}
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
//	query := listQueryRequest{Limit: limit, Offset: offset}
//	if err := reqx.ValidateQuery(&query); err != nil {
//	    resp.Problem(w, r, err)
//	    return
//	}
//
// 某些 endpoint 需要放宽默认 decode 策略时，可使用 DecodeJSONWith：
//
//	if err := reqx.DecodeJSONWith(w, r, &body, reqx.DecodeOptions{
//	    MaxBytes:             4 << 20,
//	    AllowUnknownFields:   true,
//	    SkipContentTypeCheck: false,
//	}); err != nil {
//	    resp.Problem(w, r, err)
//	    return
//	}
//
// paramx 不覆盖的边界转换失败时，也应继续复用 reqx.Problem：
//
//	rawPublishedAt := strings.TrimSpace(r.URL.Query().Get("published_at"))
//	if rawPublishedAt == "" {
//	    resp.Problem(w, r, reqx.BadRequest(reqx.Required(reqx.InQuery, "published_at")))
//	    return
//	}
//	publishedAt, err := time.Parse(time.RFC3339, rawPublishedAt)
//	if err != nil {
//	    resp.Problem(w, r, reqx.BadRequest(reqx.InvalidValue(reqx.InQuery, "published_at")))
//	    return
//	}
//
// 若 DTO 需要最小规范化，可实现 Normalize：
//
//	type createRequest struct {
//	    Name string `json:"name" validate:"required"`
//	}
//
//	func (r *createRequest) Normalize() {
//	    if r == nil {
//	        return
//	    }
//	    r.Name = strings.TrimSpace(r.Name)
//	}
//
//	var body createRequest
//	if err := reqx.DecodeJSON(w, r, &body); err != nil {
//	    resp.Problem(w, r, err)
//	    return
//	}
//	if err := reqx.ValidateBody(&body); err != nil {
//	    resp.Problem(w, r, err)
//	    return
//	}
//
// 反例：
//   - 不要把业务校验错误塞进 reqx.BadRequest / ValidationFailed。
//   - 不要让 reqx 直接读取 chi.URLParam / r.URL.Query() 之类的业务入口细节。
//   - 不要在 handler 手写一套并行的 details 结构。
package reqx
