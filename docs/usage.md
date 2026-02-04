# 使用说明

应用层通过 **api** 子目录引用 ActionFlow，勿直接使用 `internal`。

## 子目录名称：api

公开 API 放在根目录下的 `api` 子目录，导入路径为 `github.com/Pentahill/actionflow/api`，好记且统一。

## 导入方式

```go
import "github.com/Pentahill/actionflow/api"
```

使用时包名为 `actionflow`，例如：`actionflow.NewAsyncServer`、`actionflow.ServerOptional`。

## 导出类型与函数

| 名称 | 说明 |
|------|------|
| `AsyncServer` | 异步服务器 |
| `ServerOptional` | 创建服务器时的可选配置 |
| `NewAsyncServer(opt *ServerOptional) *AsyncServer` | 创建异步服务器 |
| `NewSSEHandler(server *AsyncServer) *SSEHandler` | 创建 SSE HTTP Handler |
| `SSEHandler` | 实现 `http.Handler`，可挂载到路由 |
| `UserRequest` | 用户请求（含 `GetSession()`, `GetPayload()`） |
| `AgentResponse` | Agent 结果（含 `GetSession()`, `GetPayload()`） |
| `EventStream` | 事件流函数类型 `func(ctx context.Context) (<-chan any, error)` |
| `RequestHandlerFunc` | 用户请求处理函数类型 |
| `AgentOutputHandlerFunc` | Agent 输出处理函数类型 |
| `GetSessionID` | 从 `*http.Request` 解析 session_id 的函数类型 |
| `AgentResultCallback` | Agent 结果回调函数类型 |

## 示例

见 [examples/server/main.go](../examples/server/main.go)。
