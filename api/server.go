package actionflow

import (
	async "github.com/Pentahill/actionflow/internal"
)

// NewAsyncServer 创建异步服务器。opt 可为 nil，使用默认配置。
func NewAsyncServer(opt *ServerOptional) *AsyncServer {
	return async.NewAsyncServer(opt)
}

// NewSSEHandlerWithOptions 使用可选配置创建 SSE HTTP Handler，可配置 transport 的 Decoder 与 Encoder。
// opts 可为 nil；opts 中 Decoder/Encoder 为 nil 时使用默认 JSON 编解码。
func NewSSEHandler(server *AsyncServer, opts *SSEHandlerOptions) *SSEHandler {
	return async.NewSSEHandler(server, opts)
}
