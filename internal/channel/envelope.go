package channel

import (
	"context"
	"encoding/json"
	"fmt"
)

// envelopeData 信封内部序列化数据结构（仅用于 Marshal/Unmarshal）
type envelopeData struct {
	Inner          ActionMetadata    `json:"inner"`
	ContextCarrier map[string]string `json:"context_carrier,omitempty"`
	Passthrough    []byte            `json:"passthrough,omitempty"`
}

// ActionEnvelope 信封结构，实现 Action 接口。
// 在 Dispatcher.Send 时自动将任意 Action 包入信封，
// 携带上下文 carrier 与业务透传数据，对 Channel/ActionStorage 完全透明。
// 消费侧 Dispatcher 收到后拆封，Handler 层无需感知信封。
type ActionEnvelope struct {
	// InnerAction 原始业务 Action
	InnerAction Action
	// ContextCarrier 上下文键值对（OTel trace context + 自定义字段）
	ContextCarrier map[string]string
	// Passthrough 业务透传数据（调用方负责序列化，建议 json.Marshal）
	Passthrough []byte
}

// GetSessionID 委托给 InnerAction。
func (e *ActionEnvelope) GetSessionID() string {
	return e.InnerAction.GetSessionID()
}

// Marshal 将信封序列化为标准 ActionMetadata 格式（type = "envelope"）。
// InnerAction 的完整序列化数据嵌套在 data.inner 中，与上下文 carrier 一并落库。
func (e *ActionEnvelope) Marshal() ([]byte, error) {
	innerBytes, err := e.InnerAction.Marshal()
	if err != nil {
		return nil, fmt.Errorf("marshal inner action: %w", err)
	}

	var innerMeta ActionMetadata
	if err := json.Unmarshal(innerBytes, &innerMeta); err != nil {
		return nil, fmt.Errorf("parse inner action metadata: %w", err)
	}

	dataBytes, err := json.Marshal(envelopeData{
		Inner:          innerMeta,
		ContextCarrier: e.ContextCarrier,
		Passthrough:    e.Passthrough,
	})
	if err != nil {
		return nil, fmt.Errorf("marshal envelope data: %w", err)
	}

	return json.Marshal(ActionMetadata{
		Type:      ActionTypeEnvelope,
		SessionID: e.GetSessionID(),
		Data:      dataBytes,
	})
}

// WrapAction 从 ctx 提取上下文元数据（自定义字段 + OTel trace context + 透传数据），
// 并将 action 包入 ActionEnvelope，在 Dispatcher.Send 内部调用。
func WrapAction(ctx context.Context, action Action) *ActionEnvelope {
	carrier := extractContextFields(ctx)
	passthrough, _ := PassthroughFromContext(ctx)
	return &ActionEnvelope{
		InnerAction:    action,
		ContextCarrier: carrier,
		Passthrough:    passthrough,
	}
}

// unmarshalEnvelope 反序列化信封，递归还原 InnerAction（在 UnmarshalAction 中调用）。
func unmarshalEnvelope(data json.RawMessage) (*ActionEnvelope, error) {
	var ed envelopeData
	if err := json.Unmarshal(data, &ed); err != nil {
		return nil, fmt.Errorf("unmarshal envelope data: %w", err)
	}

	innerBytes, err := json.Marshal(ed.Inner)
	if err != nil {
		return nil, fmt.Errorf("re-marshal inner metadata: %w", err)
	}

	innerAction, err := UnmarshalAction(innerBytes)
	if err != nil {
		return nil, fmt.Errorf("unmarshal inner action: %w", err)
	}

	return &ActionEnvelope{
		InnerAction:    innerAction,
		ContextCarrier: ed.ContextCarrier,
		Passthrough:    ed.Passthrough,
	}, nil
}
