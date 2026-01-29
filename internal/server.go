package async

import (
	"github.com/Pentahill/actionflow/internal/biz"
	"github.com/Pentahill/actionflow/internal/protocol"
	"net/http"
)

// RequestHandlerFunc 请求处理函数类型别名
type RequestHandlerFunc = biz.RequestHandlerFunc

// AgentOutputHandlerFunc Agent 输出处理函数类型别名
type AgentOutputHandlerFunc = biz.AgentOutputHandlerFunc

// GetSessionIDFunc 从 HTTP 请求中获取 sessionID 的函数类型
// 如果返回空字符串，则使用 transport 自动生成的 sessionID
type GetSessionID func(*http.Request) string

// ServerOptional 异步服务器的可选配置
type ServerOptional struct {
	// 用于处理用户请求的处理函数
	UserRequestHandler RequestHandlerFunc
	// EventStream 事件流，用于生成事件数据
	EventStream protocol.EventStream
	// AgentOutputHandler Agent 输出处理函数
	// 用于处理 AgentResultAction 的输出，接收 ctx 和 agent 的 payload
	AgentOutputHandler AgentOutputHandlerFunc
	// GetSessionID 从 HTTP 请求中获取 sessionID 的函数
	// 如果为 nil，则使用默认方法（返回空字符串，让 transport 自动生成）
	GetSessionID GetSessionID
	// AgentResultCallback Agent 结果回调函数
	// 用于组织向客户端发送的响应内容，在重放场景的 AgentResultHandler 中调用
	// 接收 ctx 和 AgentResultAction，用于自定义响应内容的组织方式
	AgentResultCallback AgentResultCallback
}

// AsyncServer 异步服务器主结构
type AsyncServer struct {
	userRequestHandler  RequestHandlerFunc
	eventStream         protocol.EventStream
	agentOutputHandler  AgentOutputHandlerFunc
	getSessionID        GetSessionID
	agentResultCallback AgentResultCallback
}

// EventStream 返回事件流（实现 biz.Server 接口）
func (s *AsyncServer) EventStream() protocol.EventStream {
	return s.eventStream
}

// NewAsyncServer 创建新的异步服务器
func NewAsyncServer(opt *ServerOptional) *AsyncServer {
	server := &AsyncServer{}

	// 应用可选配置
	if opt != nil {
		if opt.UserRequestHandler != nil {
			server.userRequestHandler = opt.UserRequestHandler
		}
		if opt.EventStream != nil {
			server.eventStream = opt.EventStream
		}
		if opt.AgentOutputHandler != nil {
			server.agentOutputHandler = opt.AgentOutputHandler
		}
		if opt.GetSessionID != nil {
			server.getSessionID = opt.GetSessionID
		} else {
			server.getSessionID = defaultGetSessionID
		}
		if opt.AgentResultCallback != nil {
			server.agentResultCallback = opt.AgentResultCallback
		}
	}

	return server
}

// defaultGetSessionID 默认的 sessionID 获取方法
// 返回空字符串，让 transport 自动生成 sessionID
func defaultGetSessionID(*http.Request) string {
	return ""
}

// GetSessionID 获取 sessionID（供 SSEHandler 等使用）
// 如果用户自定义了 GetSessionID 方法，则使用用户的方法
// 否则使用默认方法（返回空字符串）
func (s *AsyncServer) GetSessionID(req *http.Request) string {
	if s.getSessionID != nil {
		return s.getSessionID(req)
	}
	return defaultGetSessionID(req)
}
