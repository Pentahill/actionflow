package transport

import (
	"context"
	"encoding/json"
	"net/http"
)

type Binder[R Handler] interface {
	Bind(Connection) R
}

type Handler interface {
	Handle(ctx context.Context, req any) (any, error)
}

// Transport 传输层接口，用于创建双向连接
type Transport interface {
	// Connect 返回逻辑连接
	// 每个 Transport 实例应该只用于一次 Connect 调用
	Connect(ctx context.Context) (Connection, error)

	Done() <-chan struct{}
}

// 扩展了 Transport 接口，要求实现 ServeHTTP 方法
type HTTPTransport interface {
	Transport
	// ServeHTTP 处理 HTTP 请求
	ServeHTTP(http.ResponseWriter, *http.Request)
}

// Connection 逻辑双向连接接口
type Connection interface {
	// Read 从连接读取下一条消息
	Read(ctx context.Context) (any, error)

	// Write 向连接写入新消息
	Write(ctx context.Context, msg any) error

	// Close 关闭连接。当 Read 或 Write 失败时会隐式调用
	Close() error

	// SessionID 返回会话 ID
	SessionID() string
}

func ConnectAndBind[H Handler](ctx context.Context, t Transport, b Binder[H]) (H, error) {
	var zero H
	conn, err := t.Connect(ctx)
	if err != nil {
		return zero, err
	}

	h := b.Bind(conn)

	// mock逻辑，消费并处理消息
	go func() {
		for {
			msg, err := conn.Read(ctx)
			if err != nil {
				return
			}

			_, err = h.Handle(ctx, msg)
			if err != nil {
				return
			}
		}
	}()

	return h, nil
}

// DefaultJSONDecoder 默认 JSON 解码器
// 将字节数据解码为 map[string]interface{}，适用于任意 JSON 数据
func DefaultJSONDecoder() func([]byte) (any, error) {
	return func(data []byte) (any, error) {
		var result map[string]interface{}
		if err := json.Unmarshal(data, &result); err != nil {
			return nil, err
		}
		return result, nil
	}
}

// DefaultJSONEncoder 默认 JSON 编码器
// 将任意类型编码为 JSON 字节数据
func DefaultJSONEncoder() func(any) ([]byte, error) {
	return func(data any) ([]byte, error) {
		return json.Marshal(data)
	}
}
