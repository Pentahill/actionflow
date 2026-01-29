# Server Example

这是一个使用 ActionFlow 创建异步服务器的完整示例。

## 引用

包在 **api** 子目录下，导入路径为：

```go
import "github.com/Pentahill/actionflow/api"
```

## 功能说明

- **EventStream**: 创建事件流，定期向客户端发送数据
- **UserRequestHandler**: 处理客户端请求，可以持久化数据到数据库
- **AgentOutputHandler**: 处理 Agent 的输出
- **AgentResultCallback**: 处理 Agent 的结果回调，并发送到客户端

## 运行方式

```bash
cd examples/server
go run main.go
```

服务器将在 `:8080` 端口启动。

## 使用方式

### 1. 连接 SSE 端点

使用浏览器或 curl 连接到 SSE 端点：

```bash
curl -N "http://localhost:8080/?session_id=test-session-123"
```

### 2. 发送 POST 请求

向服务器发送 POST 请求：

```bash
curl -X POST "http://localhost:8080/?session_id=test-session-123" \
  -H "Content-Type: application/json" \
  -d '{"id": 1, "content": "test message"}'
```

## 配置说明

- **GetSessionID**: 从 URL 查询参数 `session_id` 中获取会话 ID
- **EventStream**: 每 5 秒发送一次事件，共发送 5 次
- **端口**: 默认 `:8080`，可在代码中修改

## 优雅关闭

服务器支持优雅关闭，按 `Ctrl+C` 时会：
1. 停止接受新请求
2. 等待现有请求完成（最多 5 秒）
3. 关闭服务器
