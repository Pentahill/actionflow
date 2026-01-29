package agent

import (
	"github.com/Pentahill/actionflow/internal/protocol"
)

// AgentResponse Agent 响应结构
type AgentResponse struct {
	session protocol.ServerSession
	Payload interface{}
}

// NewAgentResponse 创建 AgentResponse
func NewAgentResponse(session protocol.ServerSession, payload interface{}) *AgentResponse {
	return &AgentResponse{
		session: session,
		Payload: payload,
	}
}

// GetSession 获取 ServerSession
func (r *AgentResponse) GetSession() protocol.ServerSession {
	return r.session
}

// GetPayload 获取 agent 响应数据
func (r *AgentResponse) GetPayload() interface{} {
	return r.Payload
}
