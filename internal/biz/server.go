package biz

import (
	"context"
	"reflect"
	"sync"
	"time"

	"github.com/Pentahill/actionflow/internal/channel"
	"github.com/Pentahill/actionflow/internal/protocol"
	"github.com/Pentahill/actionflow/internal/transport"

	"github.com/zeromicro/go-zero/core/logx"
)

const (
	InferTaskLockExpire = 120 * time.Second
)

// RequestHandlerFunc 请求处理函数类型
type RequestHandlerFunc func(ctx context.Context, req *UserRequest) (any, error)

// EventStreamGenerator 事件流生成器类型
type EventStreamGenerator func(ctx context.Context, req *UserRequest) protocol.EventStream

// contextKey 用于 context 值的键类型
type contextKey string

const (
	EventStreamKey contextKey = "event_stream"
)

// DefaultEventStreamGenerator 默认的事件流生成器实现
// 通过检查 context 中是否存在 EventStreamKey 来判断是否返回 eventstream
func DefaultEventStreamGenerator(ctx context.Context, req protocol.Request) protocol.EventStream {
	// 从 context 中获取 EventStream
	if eventStream, ok := ctx.Value(EventStreamKey).(protocol.EventStream); ok && eventStream != nil {
		logx.WithContext(ctx).Debugf("Using EventStream from context, session_id=%s", req.GetSession().ID())
		return eventStream
	}

	// context 中不存在 EventStream，返回 nil
	logx.WithContext(ctx).Debugf("No EventStream in context, returning nil, session_id=%s", req.GetSession().ID())
	return nil
}

// RequestOptional 请求处理器的可选配置
type RequestOptional struct {
	// HandlerFunc 用于处理用户请求的处理函数
	HandlerFunc RequestHandlerFunc
	// EventStreamGenerator 生成 eventstream 的生成器，用来存放用户配置的 eventstream
	EventStreamGenerator EventStreamGenerator
	// AgentOutputHandler Agent 输出处理函数
	// 用于处理 AgentResultAction 的输出，接收 ctx 和 agent 的 payload
	AgentOutputHandler AgentOutputHandlerFunc
}

// Server 服务器接口，用于避免循环导入
// 只包含 RequestHandler 需要的方法
type Server interface {
	EventStream() protocol.EventStream
}

// BizServer 请求处理器
// 负责接收客户端请求，管理会话，处理 Agent 返回的结果
type BizServer struct {
	server               Server
	dispatcher           *channel.Dispatcher
	handlerFunc          RequestHandlerFunc
	eventStreamGenerator EventStreamGenerator
	agentOutputHandler   *AgentOutputHandler
	sessionCloser        *SessionCloser
}

// NewRequestHandler 创建请求处理器
func NewBizServer(server Server, dispatcher *channel.Dispatcher, opt *RequestOptional) *BizServer {
	rh := &BizServer{
		server:        server,
		dispatcher:    dispatcher,
		sessionCloser: NewSessionCloser(),
	}

	// 应用可选配置
	if opt != nil {
		if opt.HandlerFunc != nil {
			rh.handlerFunc = opt.HandlerFunc
		}
		if opt.EventStreamGenerator != nil {
			rh.eventStreamGenerator = opt.EventStreamGenerator
		}
		// 创建并注册 AgentOutputHandler
		if opt.AgentOutputHandler != nil {
			rh.agentOutputHandler = NewAgentOutputHandler(dispatcher, opt.AgentOutputHandler)
			// AgentOutputHandler 实现了 ActionHandlerSupportType 接口，会自动获取支持的类型
			dispatcher.Register(rh.agentOutputHandler)
		}
	}

	// 注册 SessionCloser 处理关闭事件
	// SessionCloser 实现了 ActionHandlerSupportType 接口，会自动获取支持的类型
	dispatcher.Register(rh.sessionCloser)

	return rh
}

// serverSessionActionHandler 包装 ServerSession 以实现 channel.ActionHandler 接口
type serverSessionActionHandler struct {
	ss *ServerSession
}

// Handle 处理 Action（实现 channel.ActionHandler 接口）
func (h *serverSessionActionHandler) Handle(ctx context.Context, action channel.Action) error {
	userRequestAction, ok := action.(*channel.UserRequestAction)
	if !ok {
		return nil
	}

	return h.ss.HandleAction(ctx, userRequestAction)
}

// Name 返回 Handler 名称（实现 channel.ActionHandlerSupportType 接口）
func (h *serverSessionActionHandler) Name() string {
	return "serverSessionActionHandler"
}

// SupportedAction 返回支持的 Action 类型（实现 channel.ActionHandlerSupportType 接口）
func (h *serverSessionActionHandler) SupportedAction() []reflect.Type {
	return []reflect.Type{
		reflect.TypeOf((*channel.UserRequestAction)(nil)),
	}
}

func (rh *BizServer) Connect(ctx context.Context, t transport.Transport) (*ServerSession, error) {
	ss, err := transport.ConnectAndBind(ctx, t, rh)
	if err != nil {
		return nil, err
	}

	// serverSessionActionHandler 实现了 ActionHandlerSupportType 接口，会自动获取支持的类型
	rh.dispatcher.Register(&serverSessionActionHandler{ss: ss})

	return ss, nil
}

// Handle 处理 Action（实现 channel.ActionHandler 接口）
// 主要处理 AgentResultAction，将结果通知给对应的会话
func (rh *BizServer) Handle(ctx context.Context, action channel.Action) error {
	sessionID := action.GetSessionID()

	logx.WithContext(ctx).Debugf("BizServer handling action, session_id=%s", sessionID)
	return nil
}

// Bind 绑定连接，创建 ServerSession
func (rh *BizServer) Bind(conn transport.Connection) *ServerSession {
	ss := &ServerSession{
		bizServer: rh,
		conn:      conn,
		closed:    false,
	}

	// 注册会话到 SessionCloser
	rh.sessionCloser.RegisterSession(ss)

	return ss
}

type ServerSession struct {
	bizServer *BizServer
	conn      transport.Connection
	mu        sync.Mutex
	closed    bool
}

// NewServerSession 创建服务器会话
func NewServerSession(requestHandler *BizServer) *ServerSession {
	return &ServerSession{
		bizServer: requestHandler,
	}
}

func (ss *ServerSession) ID() string {
	return ss.conn.SessionID()
}

// Handle 处理用户请求（实现 transport.Handler 接口）
func (ss *ServerSession) Handle(ctx context.Context, req any) (any, error) {
	sessionID := ss.conn.SessionID()
	if sessionID == "" {
		sessionID = "<empty>"
	}

	logx.WithContext(ctx).Debugf("Received user request, session_id=%s, payload=%+v", sessionID, req)

	// 提交请求到channel
	action := channel.NewUserRequestAction(ss, req)
	if err := ss.bizServer.dispatcher.Send(ctx, action); err != nil {
		logx.WithContext(ctx).Errorf("Failed to send action, session_id=%s, error=%v", sessionID, err)
		return nil, err
	}

	return nil, nil
}

// HandleAction 处理 UserRequestAction
func (ss *ServerSession) HandleAction(ctx context.Context, action *channel.UserRequestAction) error {
	logx.WithContext(ctx).Debugf("Handling user request action, session_id=%s", ss.ID())

	payload := action.Payload
	req := NewUserRequest(ss, payload)

	if ss.bizServer.handlerFunc != nil {
		_, handleRequestError := ss.bizServer.handlerFunc(ctx, req)
		if handleRequestError != nil {
			logx.WithContext(ctx).Errorf("Failed to handle user request, session_id=%s, error=%v", ss.ID(), handleRequestError)

			// 發生錯誤，關閉會話
			processingErrorAction := channel.NewProcessingErrorActionWithSessionID(ss.ID(), handleRequestError, "UserRequestHandler", payload)
			if err := ss.bizServer.dispatcher.Send(ctx, processingErrorAction); err != nil {
				logx.WithContext(ctx).Errorf("Failed to send processing error action, session_id=%s, error=%v", ss.ID(), err)
				return err
			}

			closeAction := channel.NewSessionCloseActionWithSessionID(ss.ID(), "user_request_handler_error")
			if err := ss.bizServer.dispatcher.Send(ctx, closeAction); err != nil {
				logx.WithContext(ctx).Errorf("Failed to send close action, session_id=%s, error=%v", ss.ID(), err)
				return err
			}

			return nil
		}
	}

	eventStreamGenerator := ss.bizServer.eventStreamGenerator
	if eventStreamGenerator != nil {
		eventStream := eventStreamGenerator(ctx, req)
		if eventStream != nil {
			callAction := channel.NewCallAgentAction(ss, eventStream)
			if err := ss.bizServer.dispatcher.Send(ctx, callAction); err != nil {
				logx.WithContext(ctx).Errorf("Failed to send call action, session_id=%s, error=%v", ss.ID(), err)
			}
		}
	}

	return nil
}

func (ss *ServerSession) Send(ctx context.Context, result any) (any, error) {
	ss.mu.Lock()
	closed := ss.closed
	ss.mu.Unlock()

	if closed {
		logx.WithContext(ctx).Debugf("Session is closed, ignoring notification, session_id=%s", ss.ID())
		return nil, nil
	}

	logx.WithContext(ctx).Debugf("Send result to user, session_id=%s, payload=%+v", ss.ID(), result)
	if err := ss.conn.Write(ctx, result); err != nil {
		logx.WithContext(ctx).Errorf("Failed to write to connection, session_id=%s, error=%v", ss.ID(), err)
		return nil, err
	}

	return nil, nil
}

// Close 关闭会话并释放资源
func (ss *ServerSession) Close(ctx context.Context) error {
	ss.mu.Lock()
	defer ss.mu.Unlock()

	if ss.closed {
		return nil
	}

	ss.closed = true

	// 关闭连接
	if ss.conn != nil {
		if err := ss.conn.Close(); err != nil {
			logx.WithContext(ctx).Errorf("Failed to close connection, session_id=%s, error=%v", ss.ID(), err)
			return err
		}
	}

	logx.WithContext(ctx).Debugf("Session closed, session_id=%s", ss.ID())
	return nil
}

// SessionCloser 会话关闭处理器
// 负责处理会话关闭事件，等待所有任务完成后关闭连接和释放资源
type SessionCloser struct {
	mu sync.Mutex

	// 会话ID到会话的映射
	sessions map[string]*ServerSession

	// 会话ID到待完成任务计数的映射
	pendingTasks map[string]int

	// 会话关闭等待组
	closeWaitGroups map[string]*sync.WaitGroup
}

// NewSessionCloser 创建会话关闭处理器
func NewSessionCloser() *SessionCloser {
	return &SessionCloser{
		sessions:        make(map[string]*ServerSession),
		pendingTasks:    make(map[string]int),
		closeWaitGroups: make(map[string]*sync.WaitGroup),
	}
}

// RegisterSession 注册会话
func (sc *SessionCloser) RegisterSession(session *ServerSession) {
	sc.mu.Lock()
	defer sc.mu.Unlock()

	sessionID := session.ID()
	sc.sessions[sessionID] = session
	sc.pendingTasks[sessionID] = 0
	sc.closeWaitGroups[sessionID] = &sync.WaitGroup{}

	logx.Debugf("Client session registered, session_id=%s", sessionID)
}

// IncrementTask 增加待完成任务计数
func (sc *SessionCloser) IncrementTask(sessionID string) {
	sc.mu.Lock()
	defer sc.mu.Unlock()

	if _, exists := sc.pendingTasks[sessionID]; !exists {
		sc.pendingTasks[sessionID] = 0
	}
	sc.pendingTasks[sessionID]++
	sc.closeWaitGroups[sessionID].Add(1)

	logx.Debugf("Task incremented, session_id=%s, pending_tasks=%d", sessionID, sc.pendingTasks[sessionID])
}

// DecrementTask 减少待完成任务计数
func (sc *SessionCloser) DecrementTask(sessionID string) {
	sc.mu.Lock()
	defer sc.mu.Unlock()

	if count, exists := sc.pendingTasks[sessionID]; exists && count > 0 {
		sc.pendingTasks[sessionID]--
		sc.closeWaitGroups[sessionID].Done()
		logx.Debugf("Task decremented, session_id=%s, pending_tasks=%d", sessionID, sc.pendingTasks[sessionID])
	}
}

// Handle 处理 Action（实现 channel.ActionHandler 接口）
func (sc *SessionCloser) Handle(ctx context.Context, action channel.Action) error {
	sessionID := action.GetSessionID()
	if sessionID == "" {
		sessionID = "<empty>"
	}

	logx.WithContext(ctx).Debugf("SessionCloser: Handling action, session_id=%s", sessionID)

	// 处理 SessionCloseAction
	if closeAction, ok := action.(*channel.SessionCloseAction); ok {
		return sc.handleCloseAction(ctx, closeAction)
	}

	// 处理 CallAgentAction（增加任务计数）
	if _, ok := action.(*channel.EventStreamCallAgentAction); ok {
		sc.IncrementTask(sessionID)
		return nil
	}

	// 处理 AgentResultAction（减少任务计数）
	if resultAction, ok := action.(*channel.AgentResultAction); ok {
		if resultAction.Done {
			sc.DecrementTask(sessionID)
		}
		return nil
	}

	return nil
}

// Name 返回 Handler 名称（实现 channel.ActionHandlerSupportType 接口）
func (sc *SessionCloser) Name() string {
	return "SessionCloser"
}

// SupportedAction 返回支持的 Action 类型（实现 channel.ActionHandlerSupportType 接口）
func (sc *SessionCloser) SupportedAction() []reflect.Type {
	return []reflect.Type{
		reflect.TypeOf((*channel.SessionCloseAction)(nil)),
		reflect.TypeOf((*channel.EventStreamCallAgentAction)(nil)),
		reflect.TypeOf((*channel.AgentResultAction)(nil)),
	}
}

// handleCloseAction 处理关闭 Action
func (sc *SessionCloser) handleCloseAction(ctx context.Context, closeAction *channel.SessionCloseAction) error {
	sessionID := closeAction.GetSessionID()
	if sessionID == "" {
		sessionID = "<empty>"
	}

	logx.WithContext(ctx).Debugf("Processing close action, session_id=%s, reason=%s", sessionID, closeAction.Reason)

	sc.mu.Lock()
	session, exists := sc.sessions[sessionID]
	wg, wgExists := sc.closeWaitGroups[sessionID]
	sc.mu.Unlock()

	if !exists {
		logx.WithContext(ctx).Errorf("Session not found, session_id=%s", sessionID)
		return nil
	}

	// 等待所有任务完成（带超时）
	done := make(chan struct{})
	go func() {
		if wgExists {
			wg.Wait()
		}
		close(done)
	}()

	select {
	case <-done:
		logx.WithContext(ctx).Debugf("All tasks completed, session_id=%s", sessionID)
	case <-time.After(30 * time.Second):
		logx.WithContext(ctx).Errorf("Timeout waiting for tasks, session_id=%s", sessionID)
	case <-ctx.Done():
		logx.WithContext(ctx).Errorf("Context cancelled, session_id=%s", sessionID)
		return ctx.Err()
	}

	// 关闭连接和释放资源
	if session != nil {
		if err := session.Close(ctx); err != nil {
			logx.WithContext(ctx).Errorf("Failed to close session, session_id=%s, error=%v", sessionID, err)
		} else {
			logx.WithContext(ctx).Debugf("Session closed, session_id=%s", sessionID)
		}
	}

	// 清理资源
	sc.mu.Lock()
	delete(sc.sessions, sessionID)
	delete(sc.pendingTasks, sessionID)
	delete(sc.closeWaitGroups, sessionID)
	sc.mu.Unlock()

	return nil
}
