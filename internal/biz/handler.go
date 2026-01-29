package biz

import (
	"context"
	"github.com/Pentahill/actionflow/internal/channel"
	"reflect"

	"github.com/zeromicro/go-zero/core/logx"
)

// AgentOutputHandlerFunc Agent 输出处理函数类型
// 接收 context 和 AgentResultAction 的 payload
type AgentOutputHandlerFunc func(ctx context.Context, payload interface{}) error

// AgentOutputHandler 处理 AgentResultAction 的 Handler
// 调用用户注册的处理函数来处理 Agent 的输出
type AgentOutputHandler struct {
	name          string
	dispatcher    *channel.Dispatcher
	handlerFunc   AgentOutputHandlerFunc
	sessionCloser *SessionCloser
}

// NewAgentOutputHandler 创建 AgentOutputHandler
// dispatcher: Action 分发器
// handlerFunc: 用户注册的处理函数，用于处理 Agent 的输出
func NewAgentOutputHandler(dispatcher *channel.Dispatcher, handlerFunc AgentOutputHandlerFunc) *AgentOutputHandler {
	return &AgentOutputHandler{
		name:          "AgentOutputHandler",
		dispatcher:    dispatcher,
		handlerFunc:   handlerFunc,
		sessionCloser: NewSessionCloser(),
	}
}

// Handle 处理 Action（实现 channel.ActionHandler 接口）
// 只处理 AgentResultAction，调用用户注册的处理函数
func (h *AgentOutputHandler) Handle(ctx context.Context, action channel.Action) error {
	// 只处理 AgentResultAction
	agentResultAction, ok := action.(*channel.AgentResultAction)
	if !ok {
		// 不是 AgentResultAction，忽略
		return nil
	}

	sessionID := agentResultAction.GetSessionID()
	if sessionID == "" {
		sessionID = "<empty>"
	}

	logx.WithContext(ctx).Debugf("%s Processing AgentResultAction, session_id=%s, done=%v, success=%v", h.name,
		sessionID, agentResultAction.Done, agentResultAction.Success)

	// 如果任务完成，发送关闭 Action
	if agentResultAction.Done {
		closeAction := channel.NewSessionCloseActionWithSessionID(sessionID, "task_completed")
		if err := h.dispatcher.Send(ctx, closeAction); err != nil {
			logx.WithContext(ctx).Errorf("Failed to send close action, session_id=%s, error=%v", sessionID, err)
			return err
		}
		logx.WithContext(ctx).Debugf("Sent close action, session_id=%s", sessionID)
		return nil
	}

	// 如果有用户注册的处理函数，调用它
	if h.handlerFunc != nil {
		payload := agentResultAction.Payload
		if err := h.handlerFunc(ctx, payload); err != nil {
			logx.WithContext(ctx).Errorf("Handler function error, session_id=%s, error=%v", sessionID, err)

			// 发送处理错误 Action
			h.dispatcher.Send(ctx, channel.NewProcessingErrorActionWithSessionID(sessionID, err, "AgentOutputHandler", payload))
			return err
		}

		// 处理完成后，发送 AgentResultCallbackAction 来触发 AgentResultCallbackHandler
		// 保证处理和回复的顺序
		callbackAction := channel.NewAgentResultCallbackAction(agentResultAction)
		if err := h.dispatcher.Send(ctx, callbackAction); err != nil {
			logx.WithContext(ctx).Errorf("Failed to send callback action, session_id=%s, error=%v", sessionID, err)
			return err
		}
	}

	return nil
}

// Name 返回 Handler 名称（实现 channel.ActionHandlerSupportType 接口）
func (h *AgentOutputHandler) Name() string {
	return h.name
}

// SupportedAction 返回支持的 Action 类型（实现 channel.ActionHandlerSupportType 接口）
func (h *AgentOutputHandler) SupportedAction() []reflect.Type {
	return []reflect.Type{
		reflect.TypeOf((*channel.AgentResultAction)(nil)),
	}
}
