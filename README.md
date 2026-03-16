# chix

`chix` 是一个基于 `chi` 的、强约束的 JSON API 边界内核。

仓库与模块路径：

- `github.com/kanata996/chix`

安装：

```bash
go get github.com/kanata996/chix
```

当前公开的核心包：

- `chix`：推荐入口，提供 `chi` 下的 path/query facade
- `reqx`：JSON body 解码、DTO 校验、请求错误建模
- `errx`：业务/系统错误语义与 mapper
- `resp`：统一成功/请求错误/业务错误响应写回

内部实现：

- `internal/paramx`：`chi` 下的 path/query 参数读取与基础解析，由根包 `chix` 对外转发

设计原则：

- handler 只做解请求、调 service、写响应
- path/query 和 body 分流，不做大而全 binding
- 请求错误与业务错误分流，不混用
- 边界自故障 fail-closed

最小调用链：

`handler -> chix/reqx -> resp.Problem`

`service/repo -> errx -> resp.Error`

最小示例见 [`examples/basic/main.go`](./examples/basic/main.go)。

## 本地质量检查

```bash
make ci
```

## 社区与治理

- 贡献指南：[`CONTRIBUTING.md`](./CONTRIBUTING.md)
- 行为准则：[`CODE_OF_CONDUCT.md`](./CODE_OF_CONDUCT.md)
- 安全策略：[`SECURITY.md`](./SECURITY.md)
- 许可证：[`LICENSE`](./LICENSE)
