// Package errx 提供 HTTP 错误响应侧最小但统一的一组能力。
//
// 定位：
//   - 定义通用业务/系统错误语义与 fallback internal code。
//   - 解析内建标准语义与 transport 生命周期错误。
//   - 为 feature 构造本地 mapper（规则 + fallback）。
//   - 统一校验 Mapping 与 mapper 配置。
//   - 格式化错误链用于日志。
//   - 为 resp.Error 提供稳定、可验证的输入。
//
// 边界：
//   - 不解析请求。
//   - 不做 DTO 校验。
//   - 不写 HTTP 响应。
//
// 核心概念：
//   - 标准语义：ErrUnauthorized / ErrNotFound / ErrConflict 这类跨模块通用错误。
//   - Lookup(err)：只查询 errx 内建语义与 transport 生命周期错误。
//   - Mapper：feature 本地 error -> Mapping 函数，用于补充模块规则与 fallback。
//   - Mapping：最终 HTTP-facing 语义，包含 status/code/message。
//
// 包间协作：
//   - reqx 负责请求侧错误，不进入 errx。
//   - service/repo 负责产出标准 errx 语义或 feature 本地错误。
//   - resp.Error 先调 Lookup(err) 保留标准语义；未命中时再调 feature mapper。
//
// 什么时候直接用标准 errx：
//   - 当 feature 不需要保留额外业务错误语义时，service 可以直接返回
//     ErrUnauthorized / ErrForbidden / ErrNotFound / ErrConflict 等。
//   - 这类场景 handler 可以直接 resp.Error(w, r, err, nil)。
//
// 什么时候需要 NewMapper：
//   - feature 需要保留自定义哨兵错误供 service 分支判断。
//   - feature 需要聚合下游模块错误。
//   - feature 需要自己的 fallback internal code。
//
// 推荐用法：
//
//	var mapper = errx.NewMapper(codeInternal,
//	    errx.MapTo(ErrNotFound, errx.ErrNotFound),
//	)
//
//	func MapError(err error) errx.Mapping { return mapper.Map(err) }
//
// 直接复用标准语义：
//
//	func (s *Service) Get(ctx context.Context, id string) (*Item, error) {
//	    item, err := s.repo.Get(ctx, id)
//	    if err != nil {
//	        if errors.Is(err, sql.ErrNoRows) {
//	            return nil, errx.ErrNotFound
//	        }
//	        return nil, err
//	    }
//	    return item, nil
//	}
//
// feature 级映射：
//
//	var (
//	    ErrProductNotFound = errors.New("product not found")
//	    ErrTagExists       = errors.New("tag exists")
//	)
//
//	var mapper = errx.NewMapper(codeInternal,
//	    errx.MapTo(ErrProductNotFound, errx.ErrNotFound),
//	    errx.MapTo(ErrTagExists, errx.ErrConflict),
//	)
//
//	func MapError(err error) errx.Mapping { return mapper.Map(err) }
//
// transport 生命周期语义不应放进 MapTo：
//
//	if mapping, ok := errx.Lookup(err); ok {
//	    return mapping
//	}
//
// 反例：
//   - 不要把 context.Canceled / context.DeadlineExceeded 当作 MapTo target。
//   - 不要为 404/409 等常见状态再发明一套并行 code。
//   - 不要返回未校验的自定义 Mapping 给 resp.Error。
package errx
