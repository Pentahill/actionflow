// Package integration_test 集成测试：通过 internal 包对外 API 测试多组件串联。
// 使用 go test ./test/integration/... 运行；加 -short 可跳过耗时集成测试。
package integration_test

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	async "github.com/Pentahill/actionflow/internal"
	"github.com/Pentahill/actionflow/internal/protocol"
)

type integrationTestMessage struct {
	ID      int    `json:"id"`
	Content string `json:"content"`
}

func (t *integrationTestMessage) Decode(data []byte) error {
	return json.Unmarshal(data, t)
}

func testCallHandler(ctx context.Context) (<-chan any, error) {
	ch := make(chan any, 1)
	go func() {
		defer close(ch)
		for i := 0; i < 10; i++ {
			result := integrationTestMessage{
				ID:      2,
				Content: "agent response " + fmt.Sprintf("%d", i+1),
			}
			ch <- &result
			time.Sleep(500 * time.Millisecond)
		}
	}()
	return ch, nil
}

// TestIntegration_AllComponents 集成测试：串联 AsyncServer、SSEHandler。
// 会阻塞等待连接断开，短测试时跳过。
func TestIntegration_AllComponents(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	server := async.NewAsyncServer(&async.ServerOptional{
		EventStream: protocol.EventStream(testCallHandler),
	})
	handler := async.NewSSEHandler(server)

	testPayload := integrationTestMessage{ID: 1, Content: "test message"}
	payloadBytes, _ := json.Marshal(testPayload)
	req := httptest.NewRequest(http.MethodPost, "/test", bytes.NewBuffer(payloadBytes)).WithContext(ctx)
	w := httptest.NewRecorder()

	done := make(chan struct{})
	go func() {
		handler.ServeHTTP(w, req)
		close(done)
	}()

	select {
	case <-done:
	case <-ctx.Done():
	}
}
