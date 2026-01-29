package channel

import (
	"context"
	"encoding/json"
	"sync"
	"testing"
	"time"
)

// mockAction 用于测试的 Action 实现
type mockAction struct {
	Index     int
	sessionID string
}

func (m *mockAction) GetSessionID() string {
	return m.sessionID
}

func (m *mockAction) Marshal() ([]byte, error) {
	return json.Marshal(map[string]int{"index": m.Index})
}

// mockHandler 用于测试的 ActionHandler 实现
type mockHandler struct {
	callCount int
	mu        sync.Mutex
	onHandle  func(ctx context.Context, action Action) error
}

func newMockHandler(onHandle func(ctx context.Context, action Action) error) *mockHandler {
	return &mockHandler{
		onHandle: onHandle,
	}
}

func (m *mockHandler) Handle(ctx context.Context, action Action) error {
	m.mu.Lock()
	m.callCount++
	m.mu.Unlock()

	if m.onHandle != nil {
		return m.onHandle(ctx, action)
	}
	return nil
}

func (m *mockHandler) GetCallCount() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.callCount
}

// TestDispatcher 测试 Dispatcher 的所有功能
func TestDispatcher(t *testing.T) {
	t.Run("Send", func(t *testing.T) {
		ch := NewDefaultChannel(10)
		dispatcher := NewDispatcher(ch)
		defer dispatcher.Close()

		handler := newMockHandler(func(ctx context.Context, action Action) error {
			t.Logf("Handler called - action=%v", action)
			return nil
		})

		dispatcher.Register(handler)

		for i := range 10 {
			// 通过 channel 发送 action
			action := &mockAction{
				Index:     i,
				sessionID: "test-session-send",
			}

			if err := ch.Send(context.Background(), action); err != nil {
				t.Fatalf("Failed to send action: %v", err)
			}
		}

		// 等待 handler 被调用
		time.Sleep(100 * time.Millisecond)

		if handler.GetCallCount() == 0 {
			t.Error("Handler was not called after sending action")
		}
	})
}
