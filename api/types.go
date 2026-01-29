package actionflow

import (
	async "github.com/Pentahill/actionflow/internal"
	"github.com/Pentahill/actionflow/internal/protocol"
)

// 以下类型从 internal 重导出，供应用层使用。

// AsyncServer 异步服务器，用于创建 SSE 端点与会话管理。
type AsyncServer = async.AsyncServer

// ServerOptional 创建 AsyncServer 时的可选配置。
type ServerOptional = async.ServerOptional

// SSEHandler 实现 http.Handler，提供 SSE 端点；通常挂载到 http.HandleFunc("/", handler.ServeHTTP)。
type SSEHandler = async.SSEHandler

// UserRequest 用户请求，在 UserRequestHandler 中传入；可获取 Session 与 Payload。
type UserRequest = async.UserRequest

// AgentResponse Agent 结果，在 AgentResultCallback 中传入；可获取 Session 与 Payload。
type AgentResponse = async.AgentResponse

// RequestHandlerFunc 用户请求处理函数类型。
type RequestHandlerFunc = async.RequestHandlerFunc

// AgentOutputHandlerFunc Agent 输出处理函数类型。
type AgentOutputHandlerFunc = async.AgentOutputHandlerFunc

// GetSessionID 从 *http.Request 解析 session_id 的函数类型。
type GetSessionID = async.GetSessionID

// AgentResultCallback Agent 结果回调函数类型。
type AgentResultCallback = async.AgentResultCallback

// EventStream 事件流函数类型：传入 context，返回事件 channel 与 error。
// 用于 ServerOptional.EventStream 及类型转换。
type EventStream = protocol.EventStream
