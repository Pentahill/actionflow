package async

import (
	"context"
	"github.com/Pentahill/actionflow/internal/agent"
	"github.com/Pentahill/actionflow/internal/biz"
	"github.com/Pentahill/actionflow/internal/channel"
	"github.com/Pentahill/actionflow/internal/protocol"

	"github.com/zeromicro/go-zero/core/logx"
)

type UserRequest = biz.UserRequest
type AgentResponse = agent.AgentResponse

type AgentResultCallback = agent.AgentResultCallback

type ServerRequest struct {
	session protocol.Session
	payload any
}

func (r *ServerRequest) isRequest() {}

func (r *ServerRequest) GetSession() protocol.Session {
	return r.session
}

func (r *ServerRequest) GetPayload() any {
	return r.payload
}

func handleReceive(ctx context.Context, s *biz.ServerSession, req any) error {
	sessionID := s.ID()
	if sessionID == "" {
		sessionID = "<empty>"
	}

	logx.WithContext(ctx).Debugf("[SESSION] → Received request | session_id=%s | payload=%+v", sessionID, req)

	// 注意：ServerSession 现在在 biz 包中，逻辑已移到 ServerSession.Handle 方法
	// 此函数可能已不再使用，保留用于向后兼容
	action := channel.NewUserRequestAction(s, req)
	// 需要通过 requestHandler 发送，但这里无法直接访问
	// 建议使用 ServerSession.Handle 方法代替
	_ = action
	return nil
}
