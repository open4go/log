# log 优化方案

| # | 优先级 | 问题 | 位置 | 建议 | 工作量 |
|---|---|---|---|---|---|
| 1 | 高 | 下游仍在用旧版 log（v0.0.12–v0.0.18）。这些版本的 `log.go` 每次 `Log()` 都调用 Docker 客户端获取容器元数据，要求挂载 `/var/run/docker.sock`（相当于宿主机 root 权限），还引入了 `github.com/docker/docker`（GO-2026-4887/4883，暂无修复版本） | 下游 go.mod：db、model、auth、rest、p7、x8rpc、data、util9s | 在 log 仓库发版说明里标注“v0.0.19 之前的版本已废弃”；下游统一升级到 v0.0.23+，并去掉部署里的 docker.sock 挂载 | M（涉及多仓库） |
| 2 | 高 | 仓库提交了 vendor/（1756 个文件），可提交记录里有 “rm vendor”。作为库，vendor 不会被下游使用，只会让仓库变大、diff 难读 | `vendor/` | `git rm -r vendor` 并加入 `.gitignore` | S |
| 3 | 中 | 与 `model` 循环依赖：log→model v0.0.4（`model/login`、`model/operation`），model→log v0.0.16 | `go.mod`、`model/*` | 把审计/登录模型移到独立模块（例如 `open4go/audit`）或 `middle` | M |
| 4 | 中 | `getBaseEntry` 每条日志都调用 `viper.GetString("server.name")`（有锁、有反射） | `log.go` getBaseEntry | 在 `Init` 时缓存 server 名，或提供 `SetServiceName` | S |
| 5 | 中 | 旧字符串 key（`"traceid"`、`"ip"`、`"MERCHANT_KEY"`）还在写入和读取，`go vet`/staticcheck 会告警，也可能和其他库冲突 | `context.go` | 给定迁移期限：middle 和 model 改用类型化 key 后删除旧 key | S |
| 6 | 低 | `RequestLogger` 超过延迟阈值时统一记 Warning 并 `return`，5xx 请求如果同时超时，会被降级成 Warning | `middle.go` RequestLogger | 先按状态码定级别，超时只追加字段 `slow=true` | S |
| 7 | 低 | `go.mod` 里 gin v1.8.1、mongo-driver v1.12.0 太旧 | `go.mod` | 升级到 gin v1.10+、mongo-driver v1.17.7+（拆出 model 后可以去掉 mongo 依赖） | S |
| 8 | 中 | govulncheck：`golang.org/x/net v0.43.0`（GO-2026-6617/6612/6611/6603） | `go.mod` | `go get golang.org/x/net@v0.60.0` | S |
| 9 | 低 | 没有 README | 仓库根目录 | 补上本文档 | S |
