package channel

import (
	"context"
	"encoding/json"
	"reflect"

	"github.com/Pentahill/actionflow/internal/protocol"
)

type ActionHandlerSupportType interface {
	ActionHandler
	SupportedAction() []reflect.Type
	Name() string
}

type ActionHandle func(ctx context.Context, action Action) error

// Action 通道动作接口
type Action interface {
	// GetSessionID 获取会话 ID
	GetSessionID() string
	// Marshal 序列化 Action 为字节数组
	// 返回序列化后的字节数组和错误信息
	Marshal() ([]byte, error)
}

// ActionUnmarshaler Action 反序列化器接口
// 用于从字节数组反序列化为具体的 Action 类型
type ActionUnmarshaler interface {
	// UnmarshalAction 从字节数组反序列化为 Action
	// data: 序列化后的字节数组
	// 返回: Action 实例和错误信息
	UnmarshalAction(data []byte) (Action, error)
}

// ActionType 表示 Action 的类型
type ActionType string

const (
	ActionTypeUserRequest         ActionType = "user_request"
	ActionTypeAgentResult         ActionType = "agent_result"
	ActionTypeEventStreamCall     ActionType = "event_stream_call"
	ActionTypeSessionClose        ActionType = "session_close"
	ActionTypeProcessingError     ActionType = "processing_error"
	ActionTypeAgentResultCallback ActionType = "agent_result_callback"
)

// ActionMetadata Action 元数据，用于序列化
type ActionMetadata struct {
	Type      ActionType      `json:"type"`
	SessionID string          `json:"session_id"`
	Data      json.RawMessage `json:"data"`
}

// sessionAction 基础 Action 结构体
// 包含通用的 sessionID 字段和 GetSessionID 方法
type sessionAction struct {
	sessionID string
}

// GetSessionID 获取会话 ID
func (a *sessionAction) GetSessionID() string {
	if a.sessionID == "" {
		return ""
	}
	return a.sessionID
}

type UserRequestAction struct {
	sessionAction
	Payload interface{}
}

func NewUserRequestAction(s protocol.Session, payload interface{}) *UserRequestAction {
	sessionID := ""
	if s != nil {
		sessionID = s.ID()
	}
	return &UserRequestAction{
		sessionAction: sessionAction{sessionID: sessionID},
		Payload:       payload,
	}
}

// Marshal 序列化 UserRequestAction
func (a *UserRequestAction) Marshal() ([]byte, error) {
	payloadData, err := json.Marshal(a.Payload)
	if err != nil {
		return nil, err
	}

	metadata := ActionMetadata{
		Type:      ActionTypeUserRequest,
		SessionID: a.GetSessionID(),
		Data:      payloadData,
	}

	return json.Marshal(metadata)
}

// AgentResultActionData AgentResultAction 的序列化数据结构
type AgentResultActionData struct {
	Payload interface{} `json:"payload"`
	Success bool        `json:"success"`
	Error   string      `json:"error,omitempty"` // error 序列化为字符串
	Done    bool        `json:"done"`
}

type AgentResultAction struct {
	sessionAction
	Payload interface{}
	Success bool
	Error   error
	Done    bool
}

func NewAgentResultAction(session protocol.Session, payload interface{}) *AgentResultAction {
	sessionID := ""
	if session != nil {
		sessionID = session.ID()
	}
	return NewAgentResultActionWithSessionID(sessionID, payload)
}

// NewAgentResultActionWithSessionID 使用 sessionID 创建 AgentResultAction
func NewAgentResultActionWithSessionID(sessionID string, payload interface{}) *AgentResultAction {
	return &AgentResultAction{
		sessionAction: sessionAction{sessionID: sessionID},
		Payload:       payload,
		Success:       true,
		Error:         nil,
		Done:          false,
	}
}

func NewAgentResultActionDone(session protocol.Session, success bool, err error) *AgentResultAction {
	sessionID := ""
	if session != nil {
		sessionID = session.ID()
	}
	return NewAgentResultActionDoneWithSessionID(sessionID, success, err)
}

// NewAgentResultActionDoneWithSessionID 使用 sessionID 创建完成的 AgentResultAction
func NewAgentResultActionDoneWithSessionID(sessionID string, success bool, err error) *AgentResultAction {
	return &AgentResultAction{
		sessionAction: sessionAction{sessionID: sessionID},
		Done:          true,
		Success:       success,
		Error:         err,
	}
}

// Marshal 序列化 AgentResultAction
func (a *AgentResultAction) Marshal() ([]byte, error) {
	actionData := AgentResultActionData{
		Payload: a.Payload,
		Success: a.Success,
		Done:    a.Done,
	}
	if a.Error != nil {
		actionData.Error = a.Error.Error()
	}

	dataBytes, err := json.Marshal(actionData)
	if err != nil {
		return nil, err
	}

	metadata := ActionMetadata{
		Type:      ActionTypeAgentResult,
		SessionID: a.GetSessionID(),
		Data:      dataBytes,
	}

	return json.Marshal(metadata)
}

type EventStreamCallAgentAction struct {
	sessionAction
	EventStream protocol.EventStream
}

// NewCallAgentAction 创建 CallAgentAction
func NewCallAgentAction(session protocol.Session, eventStream protocol.EventStream) *EventStreamCallAgentAction {
	sessionID := ""
	if session != nil {
		sessionID = session.ID()
	}
	return &EventStreamCallAgentAction{
		sessionAction: sessionAction{sessionID: sessionID},
		EventStream:   eventStream,
	}
}

// Marshal 序列化 EventStreamCallAgentAction
// 注意：EventStream 无法直接序列化，这里只序列化元数据
func (a *EventStreamCallAgentAction) Marshal() ([]byte, error) {
	// EventStream 是函数类型，无法序列化，这里序列化为空数据
	// 实际使用时，EventStream 应该在运行时生成，不需要持久化
	dataBytes := []byte("{}")

	metadata := ActionMetadata{
		Type:      ActionTypeEventStreamCall,
		SessionID: a.GetSessionID(),
		Data:      dataBytes,
	}

	return json.Marshal(metadata)
}

// SessionCloseAction 会话关闭 Action
// 用于通知系统关闭会话并释放资源
type SessionCloseAction struct {
	sessionAction
	Reason string // 关闭原因
}

// NewSessionCloseAction 创建会话关闭 Action
func NewSessionCloseAction(session protocol.Session, reason string) *SessionCloseAction {
	sessionID := ""
	if session != nil {
		sessionID = session.ID()
	}
	return NewSessionCloseActionWithSessionID(sessionID, reason)
}

// NewSessionCloseActionWithSessionID 使用 sessionID 创建会话关闭 Action
func NewSessionCloseActionWithSessionID(sessionID string, reason string) *SessionCloseAction {
	return &SessionCloseAction{
		sessionAction: sessionAction{sessionID: sessionID},
		Reason:        reason,
	}
}

// Marshal 序列化 SessionCloseAction
func (a *SessionCloseAction) Marshal() ([]byte, error) {
	reasonData := map[string]interface{}{
		"reason": a.Reason,
	}
	dataBytes, err := json.Marshal(reasonData)
	if err != nil {
		return nil, err
	}

	metadata := ActionMetadata{
		Type:      ActionTypeSessionClose,
		SessionID: a.GetSessionID(),
		Data:      dataBytes,
	}

	return json.Marshal(metadata)
}

// ProcessingErrorActionData ProcessingErrorAction 的序列化数据结构
type ProcessingErrorActionData struct {
	Error              string      `json:"error"`                          // 错误信息
	OriginalActionType string      `json:"original_action_type,omitempty"` // 原始 Action 类型（可选）
	OriginalActionData interface{} `json:"original_action_data,omitempty"` // 原始 Action 数据（可选）
}

// ProcessingErrorAction 处理错误 Action
// 用于标识在 action 处理过程中发生的错误
type ProcessingErrorAction struct {
	sessionAction
	Error              error       // 错误信息
	OriginalActionType string      // 原始 Action 类型（可选，用于追踪是哪个 Action 处理时出错）
	OriginalActionData interface{} // 原始 Action 数据（可选）
}

// NewProcessingErrorAction 创建处理错误 Action
// session: 会话
// err: 错误信息
// originalActionType: 原始 Action 类型（可选）
// originalActionData: 原始 Action 数据（可选）
func NewProcessingErrorAction(session protocol.Session, err error, originalActionType string, originalActionData interface{}) *ProcessingErrorAction {
	sessionID := ""
	if session != nil {
		sessionID = session.ID()
	}
	return NewProcessingErrorActionWithSessionID(sessionID, err, originalActionType, originalActionData)
}

// NewProcessingErrorActionWithSessionID 使用 sessionID 创建处理错误 Action
func NewProcessingErrorActionWithSessionID(sessionID string, err error, originalActionType string, originalActionData interface{}) *ProcessingErrorAction {
	return &ProcessingErrorAction{
		sessionAction:      sessionAction{sessionID: sessionID},
		Error:              err,
		OriginalActionType: originalActionType,
		OriginalActionData: originalActionData,
	}
}

// Marshal 序列化 ProcessingErrorAction
func (a *ProcessingErrorAction) Marshal() ([]byte, error) {
	actionData := ProcessingErrorActionData{
		Error: "",
	}
	if a.Error != nil {
		actionData.Error = a.Error.Error()
	}
	if a.OriginalActionType != "" {
		actionData.OriginalActionType = a.OriginalActionType
	}
	if a.OriginalActionData != nil {
		actionData.OriginalActionData = a.OriginalActionData
	}

	dataBytes, err := json.Marshal(actionData)
	if err != nil {
		return nil, err
	}

	metadata := ActionMetadata{
		Type:      ActionTypeProcessingError,
		SessionID: a.GetSessionID(),
		Data:      dataBytes,
	}

	return json.Marshal(metadata)
}

// AgentResultCallbackAction 用于触发 AgentResultCallbackHandler 的 Action
// 包含和 AgentResultAction 相同的内容，用于保证处理和回复的顺序
type AgentResultCallbackAction struct {
	sessionAction
	Payload interface{}
	Success bool
	Error   error
	Done    bool
}

// NewAgentResultCallbackAction 创建 AgentResultCallbackAction
// 从 AgentResultAction 创建，包含相同的内容
func NewAgentResultCallbackAction(action *AgentResultAction) *AgentResultCallbackAction {
	return &AgentResultCallbackAction{
		sessionAction: sessionAction{sessionID: action.GetSessionID()},
		Payload:       action.Payload,
		Success:       action.Success,
		Error:         action.Error,
		Done:          action.Done,
	}
}

// Marshal 序列化 AgentResultCallbackAction
func (a *AgentResultCallbackAction) Marshal() ([]byte, error) {
	actionData := AgentResultActionData{
		Payload: a.Payload,
		Success: a.Success,
		Done:    a.Done,
	}
	if a.Error != nil {
		actionData.Error = a.Error.Error()
	}

	dataBytes, err := json.Marshal(actionData)
	if err != nil {
		return nil, err
	}

	metadata := ActionMetadata{
		Type:      ActionTypeAgentResultCallback,
		SessionID: a.GetSessionID(),
		Data:      dataBytes,
	}

	return json.Marshal(metadata)
}

// UnmarshalAction 从字节数组反序列化为 Action
// 这是全局的反序列化函数，根据 ActionType 选择对应的类型
func UnmarshalAction(data []byte) (Action, error) {
	var metadata ActionMetadata
	if err := json.Unmarshal(data, &metadata); err != nil {
		return nil, err
	}

	sessionID := metadata.SessionID

	switch metadata.Type {
	case ActionTypeUserRequest:
		var payload interface{}
		if err := json.Unmarshal(metadata.Data, &payload); err != nil {
			return nil, err
		}
		return &UserRequestAction{
			sessionAction: sessionAction{sessionID: sessionID},
			Payload:       payload,
		}, nil

	case ActionTypeAgentResult:
		var actionData AgentResultActionData
		if err := json.Unmarshal(metadata.Data, &actionData); err != nil {
			return nil, err
		}
		action := &AgentResultAction{
			sessionAction: sessionAction{sessionID: sessionID},
			Payload:       actionData.Payload,
			Success:       actionData.Success,
			Done:          actionData.Done,
		}
		if actionData.Error != "" {
			action.Error = &actionError{msg: actionData.Error}
		}
		return action, nil

	case ActionTypeEventStreamCall:
		// EventStream 无法反序列化，返回一个占位 Action
		// 实际使用时，EventStream 应该在运行时生成
		return &EventStreamCallAgentAction{
			sessionAction: sessionAction{sessionID: sessionID},
			EventStream:   nil, // EventStream 无法从持久化中恢复
		}, nil

	case ActionTypeSessionClose:
		var reasonData map[string]string
		if err := json.Unmarshal(metadata.Data, &reasonData); err != nil {
			return nil, err
		}
		return &SessionCloseAction{
			sessionAction: sessionAction{sessionID: sessionID},
			Reason:        reasonData["reason"],
		}, nil

	case ActionTypeProcessingError:
		var actionData ProcessingErrorActionData
		if err := json.Unmarshal(metadata.Data, &actionData); err != nil {
			return nil, err
		}
		action := &ProcessingErrorAction{
			sessionAction: sessionAction{sessionID: sessionID},
			Error:         &actionError{msg: actionData.Error},
		}
		if actionData.OriginalActionType != "" {
			action.OriginalActionType = actionData.OriginalActionType
		}
		if actionData.OriginalActionData != nil {
			action.OriginalActionData = actionData.OriginalActionData
		}
		return action, nil

	case ActionTypeAgentResultCallback:
		var actionData AgentResultActionData
		if err := json.Unmarshal(metadata.Data, &actionData); err != nil {
			return nil, err
		}
		action := &AgentResultCallbackAction{
			sessionAction: sessionAction{sessionID: sessionID},
			Payload:       actionData.Payload,
			Success:       actionData.Success,
			Done:          actionData.Done,
		}
		if actionData.Error != "" {
			action.Error = &actionError{msg: actionData.Error}
		}
		return action, nil

	default:
		return nil, json.Unmarshal(data, &struct{}{}) // 返回格式错误
	}
}

// actionError 简单的 error 实现，用于反序列化
type actionError struct {
	msg string
}

func (e *actionError) Error() string {
	return e.msg
}
