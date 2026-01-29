package agent

import (
	"context"
	"github.com/Pentahill/actionflow/internal/channel"
	"github.com/Pentahill/actionflow/internal/protocol"
	"reflect"

	"github.com/zeromicro/go-zero/core/logx"
)

// AgentResultCallback Agent 结果回调函数类型
// 用于处理 AgentResponse，组织向客户端发送的响应内容
type AgentResultCallback func(ctx context.Context, response *AgentResponse) error

// AgentResultCallbackHandler 专门处理 AgentResultAction 的 Handler
// 用于重放场景，消费所有从 ActionStorage 中读取的 AgentResultAction
type AgentResultCallbackHandler struct {
	name          string
	serverSession protocol.ServerSession
	onResult      AgentResultCallback
}

// NewAgentResultHandler 创建 AgentResultHandler
func NewAgentResultCallbackHandler(serverSession protocol.ServerSession, onResult AgentResultCallback) *AgentResultCallbackHandler {
	return &AgentResultCallbackHandler{
		name:          "AgentResultCallbackHandler",
		serverSession: serverSession,
		onResult:      onResult,
	}
}

// Handle 处理 Action（实现 ActionHandler 接口）
// 处理 AgentResultCallbackAction，其他类型的 Action 会被忽略
func (h *AgentResultCallbackHandler) Handle(ctx context.Context, action channel.Action) error {
	// 只处理 AgentResultCallbackAction
	callbackAction, ok := action.(*channel.AgentResultCallbackAction)
	if !ok {
		// 不是 AgentResultCallbackAction，忽略
		return nil
	}

	sessionID := callbackAction.GetSessionID()
	if sessionID == "" {
		sessionID = "<empty>"
	}

	logx.WithContext(ctx).Debugf("%s Processing AgentResultCallbackAction, session_id=%s, done=%v, success=%v", h.name,
		sessionID, callbackAction.Done, callbackAction.Success)

	// 如果有回调函数，创建 AgentResponse 并调用它
	if h.onResult != nil {
		// 使用 handler 中保存的 serverSession（在重放场景中，Action 中不再保存 Session 对象）
		serverSession := h.serverSession

		// 创建 AgentResponse
		response := NewAgentResponse(
			serverSession,
			callbackAction.Payload,
		)

		// 调用回调函数
		if err := h.onResult(ctx, response); err != nil {
			logx.WithContext(ctx).Errorf("Callback error, session_id=%s, error=%v",
				sessionID, err)
			return err
		}
	}

	logx.WithContext(ctx).Debugf("AgentResultCallbackAction processed, session_id=%s",
		sessionID)

	return nil
}

// Name 返回 Handler 名称（实现 channel.ActionHandlerSupportType 接口）
func (h *AgentResultCallbackHandler) Name() string {
	return h.name
}

// SupportedAction 返回支持的 Action 类型（实现 channel.ActionHandlerSupportType 接口）
func (h *AgentResultCallbackHandler) SupportedAction() []reflect.Type {
	return []reflect.Type{
		reflect.TypeOf((*channel.AgentResultCallbackAction)(nil)),
	}
}
