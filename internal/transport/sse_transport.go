package transport

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"sync"

	"github.com/Pentahill/actionflow/internal/protocol"
)

// ServeHTTP 处理 POST 请求到传输端点
// 从 HTTP 请求中读取事件数据并推送到 incoming channel
func (t *SSETransport) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	t.mu.Lock()
	incoming := t.incoming
	closed := t.closed
	t.mu.Unlock()

	if incoming == nil {
		http.Error(w, "session not connected", http.StatusInternalServerError)
		return
	}

	if closed {
		http.Error(w, "session closed", http.StatusBadRequest)
		return
	}

	if req.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// 读取并解析消息
	data, err := io.ReadAll(req.Body)
	if err != nil {
		http.Error(w, "failed to read body", http.StatusBadRequest)
		return
	}
	defer req.Body.Close()

	// 解码
	msg, err := t.decoder(data)
	if err != nil {
		http.Error(w, "failed to parse body", http.StatusBadRequest)
		return
	}

	// 尝试将消息推送到队列
	select {
	case t.incoming <- msg:
		w.WriteHeader(http.StatusAccepted)
		_, _ = w.Write([]byte("accepted"))
	case <-t.done:
		http.Error(w, "session closed", http.StatusBadRequest)
	}
}

// SSETransport SSE 传输层实现
type SSETransport struct {
	Response http.ResponseWriter

	// incoming 是传入消息的队列
	// 它永远不会关闭，按照约定，incoming 非 nil 当且仅当 transport 已连接（服务器模式）
	incoming chan any

	decoder func([]byte) (any, error)
	encoder func(any) ([]byte, error)

	// 必须保护对 incoming 队列的推送
	mu     sync.Mutex    // 保护对 incoming 和 closed 的访问
	closed bool          // 当流关闭时设置
	done   chan struct{} // 当连接关闭时关闭

	sessionID string
}

func NewSSETransport(response http.ResponseWriter, decoder func([]byte) (any, error), encoder func(any) ([]byte, error), sessionID string) *SSETransport {
	id := sessionID
	if id == "" {
		id = protocol.RandText()
	}

	return &SSETransport{
		sessionID: id,
		Response:  response,
		done:      make(chan struct{}),
		decoder:   decoder,
		encoder:   encoder,
	}
}

// Connect 创建 SSE 连接
func (t *SSETransport) Connect(ctx context.Context) (Connection, error) {
	t.mu.Lock()
	defer t.mu.Unlock()

	if t.incoming != nil {
		return nil, fmt.Errorf("transport already connected")
	}

	t.incoming = make(chan any, 100) // 缓冲 channel，避免阻塞

	return NewSSEConnection(t), nil
}

// Close 关闭传输
func (t *SSETransport) Close() error {
	t.mu.Lock()
	defer t.mu.Unlock()

	if t.closed {
		return nil
	}

	t.closed = true
	if t.done != nil {
		close(t.done)
	}
	if t.incoming != nil {
		close(t.incoming)
		t.incoming = nil
	}

	return nil
}

func (t *SSETransport) Done() <-chan struct{} {
	return t.done
}

// GetSessionID 获取会话 ID（在 Connect 之前可以调用）
func (t *SSETransport) GetSessionID() string {
	t.mu.Lock()
	defer t.mu.Unlock()
	return t.sessionID
}

// SetSessionID 设置会话 ID（在 Connect 之前可以调用）
func (t *SSETransport) SetSessionID(sessionID string) {
	if sessionID == "" {
		return
	}
	t.mu.Lock()
	defer t.mu.Unlock()
	t.sessionID = sessionID
}

// 用于从 HTTP POST 请求中读取数据
type sseConnection struct {
	transport *SSETransport
	closed    bool
}

func NewSSEConnection(transport *SSETransport) *sseConnection {
	return &sseConnection{
		transport: transport,
		closed:    false,
	}
}

// Read 从连接读取下一条消息（从 incoming channel）
func (c *sseConnection) Read(ctx context.Context) (any, error) {
	var zero protocol.Decodable
	c.transport.mu.Lock()
	closed := c.transport.closed
	incoming := c.transport.incoming
	c.transport.mu.Unlock()

	if closed || incoming == nil {
		return zero, fmt.Errorf("connection closed")
	}

	select {
	case <-ctx.Done():
		return zero, ctx.Err()
	case <-c.transport.done:
		return zero, fmt.Errorf("transport closed")
	case msg, ok := <-incoming:
		if !ok {
			return zero, fmt.Errorf("incoming channel closed")
		}

		return msg, nil
	}
}

// Write SSE 连接是单向的（只读），不支持写入
func (c *sseConnection) Write(ctx context.Context, msg any) error {
	if ctx.Err() != nil {
		return ctx.Err()
	}

	data, err := c.transport.encoder(msg)
	if err != nil {
		return err
	}

	_, err = writeEvent(c.transport.Response, Event{Name: "message", Data: data})
	if err != nil {
		return err
	}

	return err
}

// Close 关闭连接
func (c *sseConnection) Close() error {
	if c.closed {
		return nil
	}
	c.closed = true
	return c.transport.Close()
}

// SessionID 返回会话 ID（使用 Endpoint 作为会话标识）
func (c *sseConnection) SessionID() string {
	c.transport.mu.Lock()
	defer c.transport.mu.Unlock()
	if c.transport.sessionID != "" {
		return c.transport.sessionID
	}
	return ""
}
