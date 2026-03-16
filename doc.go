// Package chix 是基于 chi 的推荐入口。
//
// 定位：
//   - 暴露 path/query/header 参数读取 facade。
//   - 作为外部使用者默认导入的入口，屏蔽内部 transport helper 布局。
//   - 与 reqx / errx / resp 共同组成最小 JSON API 边界内核。
//
// 推荐公开组合：
//   - chix：读取 path/query/header 参数。
//   - reqx：解 JSON body、校验 DTO、建模请求错误。
//   - errx：定义业务/系统错误语义与 mapper。
//   - resp：统一成功/错误响应写回。
//
// 最小调用链：
//
//	handler -> chix/reqx -> resp.Problem
//	service/repo -> errx -> resp.Error
package chix
