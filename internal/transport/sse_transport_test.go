package transport

// import (
// 	"bytes"
// 	"context"
// 	"encoding/json"
// 	"io"
// 	"net/http"
// 	"net/http/httptest"
// 	"testing"

// 	"github.com/stretchr/testify/assert"
// 	"github.com/stretchr/testify/require"
// )

// // testMessage 测试用的消息类型，实现 Decodable 接口
// type testMessage struct {
// 	ID      int    `json:"id"`
// 	Content string `json:"content"`
// }

// func (t *testMessage) Decode(data []byte) error {
// 	return json.Unmarshal(data, t)
// }

// // errorReader 用于模拟读取错误
// type errorReader struct{}

// func (e *errorReader) Read(p []byte) (n int, err error) {
// 	return 0, io.ErrUnexpectedEOF
// }

// func TestSSETransport_ServeHTTP(t *testing.T) {
// 	t.Run("SessionNotConnected", func(t *testing.T) {
// 		req := httptest.NewRequest(http.MethodPost, "/test", bytes.NewBufferString(`{"id":1,"content":"test"}`))
// 		w := httptest.NewRecorder()

// 		transport := NewSSETransport(w, func(data []byte) (any, error) {
// 			var msg testMessage
// 			if err := json.Unmarshal(data, &msg); err != nil {
// 				return nil, err
// 			}
// 			return &msg, nil
// 		})
// 		transport.ServeHTTP(w, req)

// 		assert.Equal(t, http.StatusInternalServerError, w.Code)
// 		assert.Contains(t, w.Body.String(), "session not connected")
// 	})

// 	// 测试：连接已关闭
// 	t.Run("SessionClosed", func(t *testing.T) {
// 		transport := NewSSETransport(w, func(data []byte) (any, error) {
// 			var msg testMessage
// 			if err := json.Unmarshal(data, &msg); err != nil {
// 				return nil, err
// 			}
// 			return &msg, nil
// 		})
// 			incoming: make(chan any, 10),
// 			closed:   true,
// 			done:     make(chan struct{}),
// 		}

// 		req := httptest.NewRequest(http.MethodPost, "/test", bytes.NewBufferString(`{"id":1,"content":"test"}`))
// 		w := httptest.NewRecorder()

// 		transport.ServeHTTP(w, req)

// 		assert.Equal(t, http.StatusBadRequest, w.Code)
// 		assert.Contains(t, w.Body.String(), "session closed")
// 	})

// 	t.Run("MethodNotAllowed", func(t *testing.T) {
// 		req := httptest.NewRequest(http.MethodGet, "/test", nil)
// 		w := httptest.NewRecorder()

// 		transport := NewSSETransport[*testMessage](w)
// 		transport.Connect(context.Background())
// 		transport.ServeHTTP(w, req)

// 		assert.Equal(t, http.StatusMethodNotAllowed, w.Code)
// 		assert.Contains(t, w.Body.String(), "method not allowed")
// 	})

// 	// 测试：读取请求体失败
// 	t.Run("ReadBodyError", func(t *testing.T) {
// 		// 创建一个会失败的 body reader
// 		req := httptest.NewRequest(http.MethodPost, "/test", &errorReader{})
// 		w := httptest.NewRecorder()

// 		transport := NewSSETransport[*testMessage](w)
// 		transport.Connect(context.Background())
// 		transport.ServeHTTP(w, req)

// 		assert.Equal(t, http.StatusBadRequest, w.Code)
// 		assert.Contains(t, w.Body.String(), "failed to read body")
// 	})

// 	// 测试：解码失败
// 	t.Run("DecodeError", func(t *testing.T) {
// 		// 发送无效的 JSON
// 		req := httptest.NewRequest(http.MethodPost, "/test", bytes.NewBufferString(`invalid json`))
// 		w := httptest.NewRecorder()

// 		transport := NewSSETransport[*testMessage](w)
// 		transport.Connect(context.Background())
// 		transport.ServeHTTP(w, req)

// 		assert.Equal(t, http.StatusBadRequest, w.Code)
// 		assert.Contains(t, w.Body.String(), "failed to parse body")
// 	})

// 	// 测试：成功处理消息
// 	t.Run("Success", func(t *testing.T) {
// 		msg := testMessage{ID: 1, Content: "test content"}
// 		msgJSON, _ := json.Marshal(msg)

// 		req := httptest.NewRequest(http.MethodPost, "/test", bytes.NewBuffer(msgJSON))
// 		w := httptest.NewRecorder()

// 		transport := NewSSETransport[*testMessage](w)
// 		conn, err := transport.Connect(context.Background())
// 		require.NoError(t, err)
// 		defer conn.Close()

// 		// 发送 HTTP POST 请求
// 		transport.ServeHTTP(w, req)

// 		// 验证响应
// 		assert.Equal(t, http.StatusAccepted, w.Code)
// 		assert.Equal(t, "accepted", w.Body.String())

// 		received, err := conn.Read(context.Background())
// 		require.NoError(t, err)
// 		require.NotNil(t, received)
// 		assert.Equal(t, 1, received.ID)
// 	})

// 	// 测试：done channel 已关闭（连接正在关闭）
// 	t.Run("DoneChannelClosed", func(t *testing.T) {
// 		w := httptest.NewRecorder()
// 		transport := NewSSETransport[*testMessage](w)
// 		conn, err := transport.Connect(context.Background())
// 		require.NoError(t, err)

// 		// 关闭 done channel
// 		transport.mu.Lock()
// 		close(transport.done)
// 		transport.mu.Unlock()

// 		msg := testMessage{ID: 1, Content: "test"}
// 		msgJSON, _ := json.Marshal(msg)

// 		req := httptest.NewRequest(http.MethodPost, "/test", bytes.NewBuffer(msgJSON))
// 		w2 := httptest.NewRecorder()

// 		transport.ServeHTTP(w2, req)

// 		// 应该返回 session closed 错误
// 		assert.Equal(t, http.StatusBadRequest, w2.Code)
// 		assert.Contains(t, w2.Body.String(), "session closed")

// 		// 验证 conn.Read 也会返回错误
// 		_, err = conn.Read(context.Background())
// 		assert.Error(t, err)
// 		assert.Contains(t, err.Error(), "transport closed")
// 	})

// 	// 测试：并发请求处理
// 	t.Run("ConcurrentRequests", func(t *testing.T) {
// 		w := httptest.NewRecorder()
// 		transport := NewSSETransport[*testMessage](w)
// 		conn, err := transport.Connect(context.Background())
// 		require.NoError(t, err)
// 		defer conn.Close()

// 		// 发送多个并发请求
// 		numRequests := 10
// 		responses := make(chan *httptest.ResponseRecorder, numRequests)

// 		for i := 0; i < numRequests; i++ {
// 			go func(id int) {
// 				msg := testMessage{ID: id, Content: "concurrent test"}
// 				msgJSON, _ := json.Marshal(msg)

// 				req := httptest.NewRequest(http.MethodPost, "/test", bytes.NewBuffer(msgJSON))
// 				w := httptest.NewRecorder()

// 				transport.ServeHTTP(w, req)
// 				responses <- w
// 			}(i)
// 		}

// 		// 收集所有响应
// 		successCount := 0
// 		for i := 0; i < numRequests; i++ {
// 			w := <-responses
// 			if w.Code == http.StatusAccepted {
// 				successCount++
// 			}
// 		}

// 		// 验证所有请求都成功
// 		assert.Equal(t, numRequests, successCount)

// 		// 使用 conn.Read 验证所有消息都被接收
// 		receivedCount := 0
// 		ctx := context.Background()
// 		for i := 0; i < numRequests; i++ {
// 			msg, err := conn.Read(ctx)
// 			if err != nil {
// 				t.Logf("Read error at message %d: %v", i, err)
// 				break
// 			}
// 			require.NotNil(t, msg)
// 			assert.Equal(t, "concurrent test", msg.Content)
// 			receivedCount++
// 		}
// 		assert.Equal(t, numRequests, receivedCount)
// 	})
// }
