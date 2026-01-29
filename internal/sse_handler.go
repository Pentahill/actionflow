package async

import (
	"github.com/Pentahill/actionflow/internal/transport"
	"net/http"
)

type SSEHandler struct {
	server         *AsyncServer
	sessionManager *SessionManager
}

func NewSSEHandler(server *AsyncServer) *SSEHandler {
	return &SSEHandler{
		server: server,
		sessionManager: NewSessionManagerWithOptions(server, &SessionManagerOptions{
			BufferSize:          100,
			AllowReplayOnClosed: true,
		}),
	}
}

func (s *SSEHandler) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")

	// 使用 server 中配置的 GetSessionID 方法获取 sessionID
	sessionID := s.server.GetSessionID(req)

	// 创建 transport，传入 sessionID（如果为空，transport 会自动生成）
	t := transport.NewSSETransport(w, transport.DefaultJSONDecoder(), transport.DefaultJSONEncoder(), sessionID)
	defer t.Close()

	sessionID = t.GetSessionID()
	w.Header().Set("X-Session-ID", t.GetSessionID())

	// 连接、处理请求并等待断开（由 SessionManager 统一处理）
	if err := s.sessionManager.ConnectAndServe(req.Context(), t, sessionID, w, req); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
}
