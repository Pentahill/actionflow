package actionflow

import (
	async "github.com/Pentahill/actionflow/internal"
)

// NewAsyncServer 创建异步服务器。opt 可为 nil，使用默认配置。
func NewAsyncServer(opt *ServerOptional) *AsyncServer {
	return async.NewAsyncServer(opt)
}

// NewSSEHandler 创建 SSE HTTP Handler，挂载到路由即可提供 SSE 端点。
func NewSSEHandler(server *AsyncServer) *SSEHandler {
	return async.NewSSEHandler(server)
}
