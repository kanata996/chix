// Package resp 提供 HTTP 响应边界最小但统一的一组能力。
//
// 定位：
//   - 写统一成功包络。
//   - 写请求错误响应（基于 reqx.Problem）。
//   - 写业务/系统错误响应（基于 errx）。
//   - 在边界自故障时 fail-closed。
//   - 作为 handler 的最终 HTTP 收口。
//
// 边界：
//   - 不解析请求。
//   - 不做 DTO 校验。
//   - 不做业务判断。
//   - 不定义业务错误语义。
//
// 三条主路径：
//   - Success / Created / NoContent：成功响应。
//   - Problem：请求错误，输入必须来自 reqx.Problem。
//   - Error：业务/系统错误，输入来自 errx 标准语义或 feature mapper。
//
// fail-closed 约定：
//   - success payload 缺失、顶层 data 编码后为 null、或边界编码失败时，返回裸 500。
//   - Problem 收到非 reqx.Problem 或非法状态码时，回退到 internal 包络。
//   - Error 收到非法 feature Mapping 时，回退到 internal 包络并记录原因。
//
// 包间协作：
//   - reqx 负责构造请求错误，resp.Problem 负责最终写回。
//   - errx 负责业务错误映射，resp.Error 负责最终写回。
//   - handler 只做解请求、调 service、选对 resp 入口。
//   - 传入 mapper 时，推荐使用 errx.NewMapper，让 feature 规则优先于通用语义，
//     再由 errx 内建语义与 fallback 兜底。
//
// 推荐用法：
//
//	if err := reqx.DecodeJSON(w, r, &body); err != nil {
//	    resp.Problem(w, r, err)
//	    return
//	}
//
//	result, err := service.Create(r.Context(), input)
//	if err != nil {
//	    resp.Error(w, r, err, mapper)
//	    return
//	}
//
//	resp.Created(w, toResponse(result))
//
// 仅复用标准 errx 语义时：
//
//	item, err := h.service.Get(r.Context(), id)
//	if err != nil {
//	    resp.Error(w, r, err, nil)
//	    return
//	}
//	if item == nil {
//	    resp.Error(w, r, errors.New("get item response is nil"), nil)
//	    return
//	}
//
//	resp.Success(w, toItemDTO(item))
//
// 使用 feature mapper 时：
//
//	result, err := h.service.Create(r.Context(), input)
//	if err != nil {
//	    resp.Error(w, r, err, mapper)
//	    return
//	}
//
//	resp.Created(w, toResponse(result))
//
// 无返回体语义时：
//
//	if err := h.service.Delete(r.Context(), id); err != nil {
//	    resp.Error(w, r, err, mapper)
//	    return
//	}
//	resp.NoContent(w)
//
// 反例：
//   - 不要在 handler 手写 `{code,message,data}` 包络。
//   - 不要把业务错误交给 Problem，也不要把 reqx.Problem 交给 Error。
//   - 不要把 nil pointer / nil slice / nil map 直接传给 Success / Created。
package resp
