package async

import (
	"net/http"

	"github.com/Pentahill/actionflow/internal/transport"
)

// SSEHandlerOptions 可选配置，用于自定义 transport 的编解码器。
// Decoder 与 Encoder 为 nil 时使用 transport 默认的 JSON 编解码。
type SSEHandlerOptions struct {
	// Decoder 解码器：将请求体字节解析为消息对象；nil 表示使用 DefaultJSONDecoder。
	Decoder func([]byte) (any, error)
	// Encoder 编码器：将消息对象序列化为字节；nil 表示使用 DefaultJSONEncoder。
	Encoder func(any) ([]byte, error)
}

type SSEHandler struct {
	server         *AsyncServer
	sessionManager *SessionManager
	opts           *SSEHandlerOptions
}

// NewSSEHandlerWithOptions 使用可选配置创建 SSE HTTP Handler。
// opts 可为 nil；opts 中 Decoder/Encoder 为 nil 时使用默认 JSON 编解码。
func NewSSEHandler(server *AsyncServer, opts *SSEHandlerOptions) *SSEHandler {
	if opts == nil {
		opts = &SSEHandlerOptions{}
	}

	return &SSEHandler{
		server: server,
		sessionManager: NewSessionManagerWithOptions(server, &SessionManagerOptions{
			BufferSize:          100,
			AllowReplayOnClosed: true,
		}),
		opts: opts,
	}
}

func (s *SSEHandler) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")

	// 使用 server 中配置的 GetSessionID 方法获取 sessionID
	sessionID := s.server.GetSessionID(req)

	decoder := transport.DefaultJSONDecoder()
	encoder := transport.DefaultJSONEncoder()
	if s.opts.Decoder != nil {
		decoder = s.opts.Decoder
	}
	if s.opts.Encoder != nil {
		encoder = s.opts.Encoder
	}

	// 创建 transport，传入 sessionID（如果为空，transport 会自动生成）
	t := transport.NewSSETransport(w, decoder, encoder, sessionID)
	defer t.Close()

	sessionID = t.GetSessionID()
	w.Header().Set("X-Session-ID", t.GetSessionID())

	// 连接、处理请求并等待断开（由 SessionManager 统一处理）
	if err := s.sessionManager.ConnectAndServe(req.Context(), t, sessionID, w, req); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
}
