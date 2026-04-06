# chix `_example`

这个目录是一个独立 Go module，用来演示 `chi + chix` 的推荐组合方式。

示例刻意保持 access log 由服务自己配置：

- 显式挂 `chi/middleware.RequestID`
- 显式挂 `traceid.Middleware`
- 显式挂 `httplog.RequestLogger`
- 默认不挂 `middleware.RequestLogAttrs()`
- 如果你希望所有 access log 都带上 `traceId`、`request.id`，可以再额外挂这个辅助中间件

同时演示几类常见写法：

- `chix.BindAndValidate(...)` 处理 path + JSON body 输入边界
- `chix.WriteError(...)` 写统一错误响应；其底层 `resp.WriteError(...)` 保留 5xx 时的 request log 注解和独立 error log 行为
- 单个 `healthz` 探针，以及 shutdown 时降级为 `503`
- `http.Server` 的常见 timeout 配置
- 基于 `SIGINT` / `SIGTERM` 的优雅停机

## 运行

```bash
cd _example
go run .
```

服务默认监听 `:8080`。

## 请求示例

健康检查：

```bash
curl -i http://localhost:8080/healthz
```

创建账号：

```bash
curl -i \
  -X POST http://localhost:8080/orgs/org_123/accounts \
  -H 'Content-Type: application/json' \
  -d '{"name":"Acme"}'
```

触发校验错误：

```bash
curl -i \
  -X POST http://localhost:8080/orgs/org_123/accounts \
  -H 'Content-Type: application/json' \
  -d '{"name":"  "}'
```

查询账号：

```bash
curl -i http://localhost:8080/orgs/org_123/accounts/acct_000001
```

重复创建同名账号，触发业务冲突：

```bash
curl -i \
  -X POST http://localhost:8080/orgs/org_123/accounts \
  -H 'Content-Type: application/json' \
  -d '{"name":"Acme"}'
```
