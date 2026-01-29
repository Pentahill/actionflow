package async

import (
	"context"
	"net/http"
	"sync"

	"github.com/Pentahill/actionflow/internal/agent"
	"github.com/Pentahill/actionflow/internal/biz"
	"github.com/Pentahill/actionflow/internal/channel"
	"github.com/Pentahill/actionflow/internal/protocol"
	"github.com/Pentahill/actionflow/internal/transport"

	"github.com/zeromicro/go-zero/core/logx"
)

// Session 完整的会话对象
type Session struct {
	server        *AsyncServer
	SessionID     string
	Channel       channel.Channel
	Dispatcher    *channel.Dispatcher
	BizServer     *biz.BizServer
	AgentRegistry *agent.AgentRegistry

	mu     sync.RWMutex
	closed bool
}

// Close 关闭会话
func (s *Session) Close() error {
	// s.mu.Lock()
	// defer s.mu.Unlock()

	if s.closed {
		return nil
	}

	s.closed = true

	logx.Debugf("Closing session, session_id=%s", s.SessionID)

	// dispatcher 会等待所有任务完成
	if s.Dispatcher != nil {
		if err := s.Dispatcher.Close(); err != nil {
			logx.Errorf("Failed to close dispatcher, session_id=%s, error=%v", s.SessionID, err)
		}
	}

	if s.Channel != nil {
		if err := s.Channel.Close(); err != nil {
			logx.Errorf("Failed to close channel, session_id=%s, error=%v", s.SessionID, err)
		}
	}

	logx.Debugf("Session closed, session_id=%s", s.SessionID)
	return nil
}

// IsClosed 检查会话是否已关闭
func (s *Session) IsClosed() bool {
	// s.mu.RLock()
	// defer s.mu.RUnlock()
	return s.closed
}

func (s *Session) AddServerContext(sc protocol.ServerSession) {
	agentResultCallback := s.server.agentResultCallback
	// AgentResultCallbackHandler 实现了 ActionHandlerSupportType 接口，会自动获取支持的类型
	s.Dispatcher.Register(agent.NewAgentResultCallbackHandler(sc, agentResultCallback))
}

// SessionManagerOptions 会话管理器的可选配置
type SessionManagerOptions struct {
	// BufferSize 通道缓冲区大小
	// 如果 <= 0，使用默认值 100
	BufferSize int

	// ActionStorage Action 持久化存储
	ActionStorage channel.ActionStorageHandler

	// AllowReplayOnClosed 是否允许在 session 关闭时进行重放
	// 如果为 true，当 session 已关闭时，仍然可以创建重放资源来消费 ActionStorage 中的结果
	// 如果为 false，session 关闭后不再允许重放，会创建新的资源
	AllowReplayOnClosed bool
}

// 负责管理所有 session 的资源创建、关闭和重放
type SessionManager struct {
	server *AsyncServer

	// sessionID 到会话的映射
	sessions map[string]*Session
	mu       sync.RWMutex

	// 配置
	bufferSize int

	// Action 持久化和重放
	actionStorage channel.ActionStorageHandler

	// 是否允许在 session 关闭时进行重放
	allowReplayOnClosed bool
}

// NewSessionManager 创建会话管理器（使用默认配置）
// 使用默认的内存 ActionStorage 和自动创建的 ReplayManager
func NewSessionManager(server *AsyncServer, bufferSize int) *SessionManager {
	return NewSessionManagerWithOptions(server, &SessionManagerOptions{
		BufferSize: bufferSize,
	})
}

// NewSessionManagerWithOptions 创建会话管理器（支持定制化配置）
func NewSessionManagerWithOptions(server *AsyncServer, opt *SessionManagerOptions) *SessionManager {
	if opt == nil {
		opt = &SessionManagerOptions{}
	}

	// 设置默认缓冲区大小
	bufferSize := opt.BufferSize
	if bufferSize <= 0 {
		bufferSize = 100
	}

	// 设置 ActionStorage
	actionStorage := opt.ActionStorage
	if actionStorage == nil {
		actionStorage = channel.NewMemoryActionStorage()
	}

	return &SessionManager{
		server:              server,
		sessions:            make(map[string]*Session),
		bufferSize:          bufferSize,
		actionStorage:       actionStorage,
		allowReplayOnClosed: opt.AllowReplayOnClosed,
	}
}

// GetOrCreate 获取或创建会话
func (sm *SessionManager) GetOrCreate(sessionID string) *Session {
	// sm.mu.Lock()
	// defer sm.mu.Unlock()

	// 检查是否已存在
	if existing, exists := sm.sessions[sessionID]; exists {
		// 会话未关闭
		if !existing.IsClosed() {
			logx.Debugf("Session exists, creating new dispatcher for replay, session_id=%s", sessionID)
			return sm.createReplaySession(sessionID, existing)
		}

		// 会话已关闭
		if sm.allowReplayOnClosed {
			logx.Debugf("Session closed but replay allowed, creating replay resources, session_id=%s", sessionID)
			return sm.createReplaySession(sessionID, existing)
		}

		// 不允许重放，删除旧记录并创建新会话
		delete(sm.sessions, sessionID)
		logx.Debugf("Existing session closed, replay not allowed, creating new session, session_id=%s", sessionID)
	}

	// 创建新会话
	session := sm.createSession(sessionID)
	sm.sessions[sessionID] = session

	logx.Debugf("Created new session, session_id=%s", sessionID)
	return session
}

// Get 获取会话（不创建）
// 如果不存在或已关闭，返回 nil
func (sm *SessionManager) Get(sessionID string) *Session {
	sm.mu.RLock()
	defer sm.mu.RUnlock()

	session, exists := sm.sessions[sessionID]
	if !exists {
		return nil
	}

	if session.IsClosed() {
		return nil
	}

	return session
}

// createResources 创建会话
func (sm *SessionManager) createSession(sessionID string) *Session {
	// 创建 channel
	chann := channel.NewDefaultChannel(sm.bufferSize)
	dispatcher := channel.NewDispatcher(chann)

	// 【ActionStorage 介入点】注册 ActionStorageHandler 实现自动持久化
	if sm.actionStorage != nil {
		// ActionStorageHandler 处理所有类型的 Action（用于持久化）
		dispatcher.Register(sm.actionStorage)
	}

	// 创建 biz server
	bizServer := biz.NewBizServer(sm.server, dispatcher, &biz.RequestOptional{
		HandlerFunc: sm.server.userRequestHandler,
		EventStreamGenerator: func(ctx context.Context, req *biz.UserRequest) protocol.EventStream {
			logx.WithContext(ctx).Debugf("Prepare agent event stream for user request, session_id=%s", req.GetSession().ID())
			return sm.server.eventStream
		},
		AgentOutputHandler: sm.server.agentOutputHandler,
	})

	agentRegistry := agent.NewAgentRegistry(dispatcher)

	return &Session{
		SessionID:     sessionID,
		server:        sm.server,
		Channel:       chann,
		Dispatcher:    dispatcher,
		BizServer:     bizServer,
		AgentRegistry: agentRegistry,
		closed:        false,
	}
}

// createReplayResources 为已存在的 session 创建重放资源
// 创建一个新的 dispatcher，从 ActionStorage 中读取 Action 并消费 AgentResultAction
func (sm *SessionManager) createReplaySession(sessionID string, s *Session) *Session {
	logx.Debugf("Creating replay session, session_id=%s", sessionID)

	// 1. 创建 StorageChannel，从 ActionStorage 中读取 Action
	// 从索引 -1 开始（即从第一个开始），使用默认流（空字符串）
	chann := channel.NewStorageChannel(sm.actionStorage, sessionID, "", -1)

	// 2. 创建新的 Dispatcher，关联 StorageChannel
	dpt := channel.NewDispatcher(chann)

	bizServer := biz.NewBizServer(sm.server, dpt, &biz.RequestOptional{})

	// 4. 返回新的 Session（复用原有的 BizServer 和 AgentRegistry）
	// 但使用新的 Dispatcher 和 Channel
	return &Session{
		SessionID:  sessionID,
		server:     sm.server,
		Channel:    chann,
		Dispatcher: dpt,
		BizServer:  bizServer,
		closed:     false,
	}
}

// Close 关闭指定 session
func (sm *SessionManager) Close(sessionID string) error {
	// sm.mu.Lock()
	// defer sm.mu.Unlock()

	session, exists := sm.sessions[sessionID]
	if !exists {
		logx.Debugf("Session not found, session_id=%s", sessionID)
		return nil
	}

	// 关闭会话
	if err := session.Close(); err != nil {
		return err
	}

	// 从映射中删除
	delete(sm.sessions, sessionID)

	return nil
}

// ConnectAndServe 连接 transport 并处理请求，然后等待断开并关闭 session 资源
func (sm *SessionManager) ConnectAndServe(ctx context.Context, t transport.HTTPTransport,
	sessionID string, w http.ResponseWriter, req *http.Request) error {
	// 获取或创建会话资源
	session := sm.GetOrCreate(sessionID)

	// 使用资源中的 BizServer 连接
	ss, err := session.BizServer.Connect(ctx, t)
	if err != nil {
		logx.WithContext(ctx).Errorf("Failed to connect, session_id=%s, error=%v", sessionID, err)
		return err
	}
	defer ss.Close(ctx)

	// 添加会话上下文
	session.AddServerContext(ss)

	t.ServeHTTP(w, req)

	// 等待连接断开
	select {
	case <-ctx.Done():
		// 请求上下文取消，关闭会话资源
		logx.WithContext(ctx).Debugf("Request context done, closing session, session_id=%s", sessionID)
		if err := sm.Close(sessionID); err != nil {
			logx.WithContext(ctx).Errorf("Failed to close session, session_id=%s, error=%v", sessionID, err)
		}
		return nil
	case <-t.Done():
		// Transport 关闭，关闭会话资源
		logx.WithContext(ctx).Debugf("Transport done, closing session, session_id=%s", sessionID)
		if err := sm.Close(sessionID); err != nil {
			logx.WithContext(ctx).Errorf("Failed to close session, session_id=%s, error=%v", sessionID, err)
		}
		return nil
	}

}

// CloseAll 关闭所有 session
func (sm *SessionManager) CloseAll() {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	logx.Debugf("Closing all sessions, count=%d", len(sm.sessions))

	for sessionID, session := range sm.sessions {
		if err := session.Close(); err != nil {
			logx.Errorf("Failed to close session, session_id=%s, error=%v", sessionID, err)
		}
	}

	// 清空映射
	sm.sessions = make(map[string]*Session)

	logx.Debugf("All sessions closed")
}

// Replay 重放功能（旧版本，保留用于向后兼容）
// 如果相同的 session_id 存在，可以获取其会话进行重放
// 注意：重放时不会创建新会话，而是返回现有会话
func (sm *SessionManager) Replay(sessionID string) (*Session, bool) {
	sm.mu.RLock()
	defer sm.mu.RUnlock()

	session, exists := sm.sessions[sessionID]
	if !exists {
		return nil, false
	}

	if session.IsClosed() {
		return nil, false
	}

	logx.Debugf("Replaying session, session_id=%s", sessionID)
	return session, true
}

// GetActionStorage 获取 ActionStorageHandler 实例（用于外部访问）
// 返回 ActionStorageHandler，它包含了 ActionStorage 接口
func (sm *SessionManager) GetActionStorage() channel.ActionStorageHandler {
	return sm.actionStorage
}

// ListSessions 列出所有活跃的 session ID
func (sm *SessionManager) ListSessions() []string {
	sm.mu.RLock()
	defer sm.mu.RUnlock()

	sessions := make([]string, 0, len(sm.sessions))
	for sessionID, resources := range sm.sessions {
		if !resources.IsClosed() {
			sessions = append(sessions, sessionID)
		}
	}

	return sessions
}
