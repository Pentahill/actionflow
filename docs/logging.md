# 日志说明

项目使用 [go-zero logx](https://go-zero.dev/docs/components/logx) 输出日志。非关键信息已设置为 **Debug** 级别，默认（Info 级别）下不会输出，避免日志冗杂。

## 日志级别

| 级别 | 说明 | 典型场景 |
|------|------|----------|
| **Debug** | 调试信息（handler 注册、分发、会话创建、存储读写等） | 开发/排查问题 |
| **Info** | 一般信息（当前默认仅保留少量关键信息） | 生产默认 |
| **Error** | 错误（连接失败、发送失败、超时等） | 始终输出 |

## 开启 Debug 日志

### 方式一：代码中设置（推荐用于示例/本地）

在 `main` 或初始化处调用：

```go
import "github.com/zeromicro/go-zero/core/logx"

func main() {
	// 开启 debug 级别，查看完整流水日志
	logx.SetLevel(logx.DebugLevel)
	// ...
}
```

### 方式二：环境变量

通过 `LOG_LEVEL` 控制（若你的应用或框架支持）：

```bash
LOG_LEVEL=debug ./your-app
```

### 方式三：配置文件

若使用 go-zero 的 `LogConf`，在配置中设置：

```yaml
Log:
  Level: debug  # debug | info | error | severe
```

## 当前 Debug 内容示例

- Channel：handler 注册、开始分发、无 handler 时的提示、dispatcher 关闭/拒绝
- Storage：流打开/关闭、Action 存储/追加、从存储读取/结束
- Biz：收到请求、处理 user request、发送结果、会话关闭、SessionCloser 处理
- Agent：处理 AgentResultAction / AgentResultCallbackAction、handler 注册、路由到具体 handler
- Session：创建/关闭会话、replay 资源创建、请求或 transport 结束时的关闭

错误类（如发送失败、连接失败、超时、handler 报错）仍为 **Error** 级别，不受 `SetLevel` 影响会照常输出。
