# log — 结构化日志、Trace 与 Gin 访问日志

> 仓库：https://github.com/open4go/log ｜ 模块：`github.com/open4go/log` ｜ Go 1.25.8 ｜ 最近提交：2026-10-08（最新 tag v0.0.23）｜ 仓库提交了 vendor（1756 个文件）

## 项目简介
基于 logrus 的进程级 JSON 日志。`Log(ctx)` 自动附带 server、trace、ip、merchantId、operator 和构建元数据：`image`（`IMAGE_TAG`，否则 `SERVER_VERSION` / `APP_VERSION` / ldflags / `VERSION` 文件）、`git_commit`（`GIT_COMMIT`，否则 ldflags 或 `runtime/debug` 的 `vcs.revision`）、`git_branch`、`build_time`、`instance`（`HOSTNAME`）。`Init` 会重新读取这些环境变量，所以进程启动后（以及测试里 `Setenv` 之后）注入的值也会生效。hook 在每条日志上写短字段 `caller`（如 `handler.go:88`），并跳过 `open4go/log`、logrus、runtime 以及 `rest.MakeError` / `MakeResponse` 包装帧，让错误行指向业务代码。默认只有 panic/fatal 带堆栈（`log.stacktrace=error` 可改为 error 也带）；提供 Gin 的 `TraceMiddleware` 和 `RequestLogger`。另外包含登录日志、操作日志两个 Mongo 模型。

## 目录结构
```
log/
├── log.go       # Init / Logger / Log / Error / Errorf / WarnWithStack ...
├── context.go   # ctxKey、NewTraceID、WithTraceID、WithIP、Inject、TraceID、ClientIP
├── caller.go    # 调用栈采集（过滤 runtime/logrus/本包帧）
├── hook.go      # locationHook：file/func/line + stacktrace
├── middle.go    # RequestLogger、TraceMiddleware（Gin）
├── model/login      # 登录日志模型（嵌入 open4go/model）
├── model/operation  # 操作审计日志模型 auth_operation_log
└── *_test.go
```

## 对外 API
| API | 说明 |
|---|---|
| `Init(level string, out io.Writer)` | 设置级别：debug/test/info/warn/error，其他值按 info |
| `Log(ctx) *logrus.Entry` | 通用入口 |
| `Error/Errorf(ctx, err, ...)`、`ErrorWithStack`（别名）、`WarnWithStack/WarnfWithStack` | 带堆栈 |
| `NewTraceID()`、`WithTraceID`、`WithIP`、`Inject`、`TraceID(ctx)`、`ClientIP(ctx)` | 同时写入类型化 key 和旧的字符串 key（`"traceid"`、`"ip"`） |
| `TraceMiddleware()` | 复用入站的 `X-Request-ID`/`X-Trace-ID`，没有就生成；写进 ctx、gin（`RequestID`、`log`）和响应头 |
| `RequestLogger()` | 每个请求一行访问日志；跳过 /healthz 等探活路径和 `*/metrics`；超过 `server.maxLatency`（ms）记 Warning |
| `model/operation.Model` | 审计字段：Service/Module/Action/Before/After/Summary/TraceID/LatencyMs 等，集合名 `auth_operation_log` |

## 使用示例
```go
log.Init(viper.GetString("log.level"), nil)
r := gin.New()
r.Use(log.TraceMiddleware(), log.RequestLogger())
log.Log(c.Request.Context()).WithField("order", id).Info("created")
log.Error(ctx, err, "pay failed")
```
`middle/trace.go` 直接转发 `log.TraceMiddleware()`。

## 依赖
`sirupsen/logrus v1.9.3`、`gin v1.8.1`、`viper`、`open4go/model v0.0.4`、`r2day/db v0.3.5`、`r2day/base v1.6.7`、`mongo-driver v1.12.0`

## 构建与测试
`go build`、`go vet` 通过；`go test ./...` 通过（`context_test.go`、`log_test.go`）。

## 与其他仓库的关系
几乎所有 open4go 仓库都依赖它（代码中有 44 处 import），但版本分散：db v0.0.12、model/auth/rest/p7/x8rpc v0.0.16、data/util9s v0.0.18、req5rsp v0.0.19、middle v0.0.23。**v0.0.18 及更早的版本会在每条日志里调用 Docker API（需要挂载 `/var/run/docker.sock`）**，新版已经去掉这一逻辑。
