# actionflow（公开 API）

应用层应通过本包引用 ActionFlow，**不要**直接使用 `internal`。

## 导入

```go
import "github.com/Pentahill/actionflow/pkg/actionflow"
```

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

见项目根目录下 [examples/server/main.go](../../examples/server/main.go)。
