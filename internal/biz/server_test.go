package biz

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/Pentahill/actionflow/internal/channel"
	"github.com/Pentahill/actionflow/internal/protocol"
	"github.com/Pentahill/actionflow/internal/transport"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// testMessage 测试用的消息类型，实现 Decodable 接口
type testMessage struct {
	ID      int    `json:"id"`
	Content string `json:"content"`
}

func (t *testMessage) Decode(data []byte) error {
	return json.Unmarshal(data, t)
}

// mockServer 实现 Server 接口用于测试
type mockServer struct {
	eventStream protocol.EventStream
}

func (m *mockServer) EventStream() protocol.EventStream {
	return m.eventStream
}

func TestRequestHandler_Connect(t *testing.T) {
	t.Run("Success", func(t *testing.T) {
		// 创建测试依赖
		server := &mockServer{eventStream: nil}
		ch := channel.NewDefaultChannel(10)
		dispatcher := channel.NewDispatcher(ch)
		bizServer := NewBizServer(server, dispatcher, &RequestOptional{})

		req := httptest.NewRequest(http.MethodPost, "/test", bytes.NewBufferString(`{"id":1,"content":"test"}`))
		w := httptest.NewRecorder()
		trans := transport.NewSSETransport(w, func(data []byte) (any, error) {
			var msg testMessage
			if err := json.Unmarshal(data, &msg); err != nil {
				return nil, err
			}
			return &msg, nil
		}, func(data any) ([]byte, error) {
			return json.Marshal(data)
		}, "")

		ctx := context.Background()
		ss, err := bizServer.Connect(ctx, trans)

		trans.ServeHTTP(w, req)

		time.Sleep(100 * time.Millisecond)

		require.NoError(t, err)
		require.NotNil(t, ss)
		assert.IsType(t, &ServerSession{}, ss)
	})

}
