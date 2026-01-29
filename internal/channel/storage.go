package channel

import (
	"context"
	"errors"
	"iter"
	"reflect"
	"sync"

	"github.com/zeromicro/go-zero/core/logx"
)

var (
	// ErrStreamNotOpen 流未打开错误
	ErrStreamNotOpen = errors.New("stream not open")
	// ErrDataDropped 数据被丢弃错误
	ErrDataDropped = errors.New("data dropped")
)

// ActionStorage 持久化存储接口
// 所有方法必须支持多 goroutine 并发调用
type ActionStorage interface {
	// Open 在创建新流时调用，用于确保底层数据结构已初始化
	// 使流准备好存储和重放 Action
	// sessionID: 会话ID
	// streamID: 流ID（可选，用于区分同一 session 下的多个流）
	// 如果 streamID 为空，使用默认流
	Open(_ context.Context, sessionID, streamID string) error

	// Append 追加一个 Action 到指定流
	// sessionID: 会话ID
	// streamID: 流ID（可选，如果为空使用默认流）
	// action: 要存储的 Action（所有类型都支持）
	// 返回: 错误信息，如果存储失败则返回错误
	// Action 会被自动序列化为字节数组存储
	Append(_ context.Context, sessionID, streamID string, action Action) error

	// After 返回一个迭代器，从指定索引之后开始迭代 Action
	// sessionID: 会话ID
	// streamID: 流ID（可选，如果为空使用默认流）
	// index: 起始索引（从0开始，不包含此索引）
	// 返回: 迭代器，每次迭代返回 (Action, error)
	// 一旦迭代器返回非 nil 错误，迭代将停止
	// 如果 index 之后的数据被丢弃，After 的迭代器必须立即返回错误，不能返回部分结果
	// 流必须已经被打开（参见 [ActionStorage.Open]）
	After(_ context.Context, sessionID, streamID string, index int) iter.Seq2[Action, error]

	// SessionClosed 通知存储该会话已完成，包括其所有流
	// sessionID: 会话ID
	// 存储不能依赖此方法进行清理，应该建立额外的机制（如超时）来回收存储空间
	SessionClosed(_ context.Context, sessionID string) error
}

// ActionStorageHandler 组合接口
// 同时包含 ActionStorage 和 ActionHandler 接口
type ActionStorageHandler interface {
	// ActionStorage 接口：持久化存储功能
	ActionStorage

	// ActionHandler 接口：可以直接作为 Handler 注册到 Dispatcher
	// Handle 处理 Action，自动持久化到存储
	// 在 Handle 方法中，使用默认流（空字符串）进行存储
	Handle(ctx context.Context, action Action) error
}

// streamData 流数据
type streamData struct {
	// 存储序列化后的 Action 字节数组
	actions [][]byte
	mu      sync.RWMutex
	opened  bool
}

// MemoryActionStorage 内存持久化存储实现
// 实现 ActionStorageHandler 接口，可以直接注册到 Dispatcher 进行自动持久化
// 使用 map 结构存储：sessionID -> streamID -> streamData
type MemoryActionStorage struct {
	// 存储结构：sessionID -> streamID -> streamData
	storage map[string]map[string]*streamData
	mu      sync.RWMutex
}

// NewMemoryActionStorage 创建内存持久化存储实例
func NewMemoryActionStorage() *MemoryActionStorage {
	return &MemoryActionStorage{
		storage: make(map[string]map[string]*streamData),
	}
}

// Handle 处理 Action（实现 ActionHandler 接口）
// 自动持久化所有类型的 Action 到存储
// 使用默认流（空字符串）进行存储
func (m *MemoryActionStorage) Handle(ctx context.Context, action Action) error {
	if action == nil {
		return nil
	}

	sessionID := action.GetSessionID()
	if sessionID == "" {
		sessionID = "<empty>"
	}

	// 使用默认流（空字符串）
	streamID := ""

	// 确保流已打开
	if err := m.Open(ctx, sessionID, streamID); err != nil {
		logx.WithContext(ctx).Errorf("Failed to open stream, session_id=%s, stream_id=%s, error=%v",
			sessionID, m.getStreamID(streamID), err)
		return err
	}

	// 持久化存储 Action
	if err := m.Append(ctx, sessionID, streamID, action); err != nil {
		logx.WithContext(ctx).Errorf("Failed to store action, session_id=%s, stream_id=%s, error=%v",
			sessionID, m.getStreamID(streamID), err)
		return err
	}

	logx.WithContext(ctx).Debugf("MemoryActionStorage: Action stored, session_id=%s, stream_id=%s, type=%T",
		sessionID, m.getStreamID(streamID), action)
	return nil
}

// Name 返回 Handler 名称（实现 channel.ActionHandlerSupportType 接口）
func (m *MemoryActionStorage) Name() string {
	return "MemoryActionStorage"
}

// SupportedAction 返回支持的 Action 类型（实现 channel.ActionHandlerSupportType 接口）
// 返回 nil 表示支持所有类型的 Action
func (m *MemoryActionStorage) SupportedAction() []reflect.Type {
	return nil // nil 表示支持所有类型
}

// getStreamID 获取流ID，如果为空则返回默认值
func (m *MemoryActionStorage) getStreamID(streamID string) string {
	if streamID == "" {
		return "default"
	}
	return streamID
}

// getOrCreateStream 获取或创建流
func (m *MemoryActionStorage) getOrCreateStream(sessionID, streamID string) *streamData {
	m.mu.Lock()
	defer m.mu.Unlock()

	streamID = m.getStreamID(streamID)

	if m.storage[sessionID] == nil {
		m.storage[sessionID] = make(map[string]*streamData)
	}

	if m.storage[sessionID][streamID] == nil {
		m.storage[sessionID][streamID] = &streamData{
			actions: make([][]byte, 0),
			opened:  false,
		}
	}

	return m.storage[sessionID][streamID]
}

// Open 打开流，初始化数据结构
func (m *MemoryActionStorage) Open(ctx context.Context, sessionID, streamID string) error {
	stream := m.getOrCreateStream(sessionID, streamID)

	stream.mu.Lock()
	defer stream.mu.Unlock()

	stream.opened = true

	logx.WithContext(ctx).Debugf("MemoryActionStorage: Stream opened, session_id=%s, stream_id=%s", sessionID, m.getStreamID(streamID))
	return nil
}

// Append 追加 Action 到流
func (m *MemoryActionStorage) Append(ctx context.Context, sessionID, streamID string, action Action) error {
	if action == nil {
		return errors.New("action cannot be nil")
	}

	stream := m.getOrCreateStream(sessionID, streamID)

	stream.mu.Lock()
	defer stream.mu.Unlock()

	if !stream.opened {
		// 如果流未打开，自动打开
		stream.opened = true
		logx.WithContext(ctx).Debugf("Stream auto-opened, session_id=%s, stream_id=%s", sessionID, m.getStreamID(streamID))
	}

	// 序列化 Action
	data, err := action.Marshal()
	if err != nil {
		logx.WithContext(ctx).Errorf("Failed to marshal action, session_id=%s, stream_id=%s, error=%v",
			sessionID, m.getStreamID(streamID), err)
		return err
	}

	// 追加到流
	stream.actions = append(stream.actions, data)

	index := len(stream.actions) - 1
	logx.WithContext(ctx).Debugf("MemoryActionStorage: Action appended, session_id=%s, stream_id=%s, index=%d",
		sessionID, m.getStreamID(streamID), index)
	return nil
}

// After 返回从指定索引之后开始的迭代器
// 流不存在，返回空迭代器
func (m *MemoryActionStorage) After(ctx context.Context, sessionID, streamID string, index int) iter.Seq2[Action, error] {
	streamID = m.getStreamID(streamID)

	m.mu.RLock()
	sessionStreams, exists := m.storage[sessionID]
	if !exists {
		m.mu.RUnlock()
		return func(yield func(Action, error) bool) {
		}
	}

	stream, streamExists := sessionStreams[streamID]
	if !streamExists {
		m.mu.RUnlock()
		return func(yield func(Action, error) bool) {
		}
	}

	// 复制 actions 切片，避免在迭代时持有锁
	stream.mu.RLock()
	actionsCopy := make([][]byte, len(stream.actions))
	copy(actionsCopy, stream.actions)
	opened := stream.opened
	stream.mu.RUnlock()
	m.mu.RUnlock()

	if !opened {
		return func(yield func(Action, error) bool) {
			yield(nil, ErrStreamNotOpen)
		}
	}

	// 检查索引是否有效
	if index < 0 {
		index = 0
	}

	// 如果索引超出范围，返回空迭代器
	if index >= len(actionsCopy) {
		return func(yield func(Action, error) bool) {
			// 索引超出范围，返回空迭代器
		}
	}

	// 返回迭代器
	return func(yield func(Action, error) bool) {
		// 从 index+1 开始迭代（After 不包含 index）
		for i := index + 1; i < len(actionsCopy); i++ {
			// 反序列化 Action
			action, err := UnmarshalAction(actionsCopy[i])
			if err != nil {
				logx.WithContext(ctx).Errorf("Failed to unmarshal action, session_id=%s, stream_id=%s, index=%d, error=%v",
					sessionID, streamID, i, err)
				if !yield(nil, err) {
					return
				}
				continue
			}

			// 检查上下文是否已取消
			select {
			case <-ctx.Done():
				if !yield(nil, ctx.Err()) {
					return
				}
				return
			default:
			}

			if !yield(action, nil) {
				return
			}
		}
	}
}

// SessionClosed 通知会话已关闭
func (m *MemoryActionStorage) SessionClosed(ctx context.Context, sessionID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	delete(m.storage, sessionID)
	logx.WithContext(ctx).Debugf("Session closed, session_id=%s", sessionID)
	return nil
}

// StorageChannel 从 ActionStorage 中读取 Action 的 Channel
type StorageChannel struct {
	storage   ActionStorage
	sessionID string
	streamID  string
	// 当前读取的索引位置
	currentIndex int
	// Action 通道
	actions chan Action
	// 关闭标志
	mu     sync.RWMutex
	closed bool
	// 停止信号
	stopChan chan struct{}
	// 等待 goroutine 完成
	wg sync.WaitGroup
}

// NewStorageChannel 创建从 ActionStorage 读取的 Channel
// sessionID: 会话ID
// streamID: 流ID（可选，如果为空使用默认流）
// startIndex: 起始索引（从0开始，-1表示从第一个开始）
func NewStorageChannel(storage ActionStorage, sessionID, streamID string, startIndex int) *StorageChannel {
	sc := &StorageChannel{
		storage:      storage,
		sessionID:    sessionID,
		streamID:     streamID,
		currentIndex: startIndex,
		actions:      make(chan Action, 100), // 缓冲区大小
		closed:       false,
		stopChan:     make(chan struct{}),
	}

	// 启动后台 goroutine 从 storage 读取 Action
	sc.wg.Add(1)
	go sc.readFromStorage()

	return sc
}

// readFromStorage 从 ActionStorage 中读取 Action 并发送到 channel
func (sc *StorageChannel) readFromStorage() {
	defer sc.wg.Done()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// 当 stopChan 关闭时，取消 context
	go func() {
		<-sc.stopChan
		cancel()
	}()

	// 使用 After 方法获取迭代器
	iter := sc.storage.After(ctx, sc.sessionID, sc.streamID, sc.currentIndex)

	// 从迭代器中读取 Action 并发送到 channel
	for action, err := range iter {
		// 检查是否已关闭
		sc.mu.RLock()
		closed := sc.closed
		sc.mu.RUnlock()

		if closed {
			return
		}

		if err != nil {
			logx.WithContext(ctx).Errorf("Iterator error, session_id=%s, stream_id=%s, error=%v",
				sc.sessionID, sc.streamID, err)
			// 关闭 channel 并返回
			sc.mu.Lock()
			if !sc.closed {
				sc.closed = true
				close(sc.actions)
			}
			sc.mu.Unlock()
			return
		}

		if action == nil {
			continue
		}

		// 检查是否是终止 Action (SessionCloseAction)
		if _, isCloseAction := action.(*SessionCloseAction); isCloseAction {
			logx.WithContext(ctx).Debugf("Reached termination action (SessionCloseAction), session_id=%s, stream_id=%s, index=%d",
				sc.sessionID, sc.streamID, sc.currentIndex+1)
			// 仍然发送这个 Action，然后停止读取
			select {
			case <-ctx.Done():
				return
			case <-sc.stopChan:
				return
			case sc.actions <- action:
				sc.currentIndex++
				logx.WithContext(ctx).Debugf("Termination action sent, session_id=%s, stream_id=%s, index=%d",
					sc.sessionID, sc.streamID, sc.currentIndex)
			}
			// 发送完终止 Action 后，关闭 channel 并返回
			sc.mu.Lock()
			if !sc.closed {
				sc.closed = true
				close(sc.actions)
			}
			sc.mu.Unlock()
			return
		}

		// 发送 Action 到 channel
		select {
		case <-ctx.Done():
			return
		case <-sc.stopChan:
			return
		case sc.actions <- action:
			sc.currentIndex++
			logx.WithContext(ctx).Debugf("Action read from storage, session_id=%s, stream_id=%s, index=%d, type=%T",
				sc.sessionID, sc.streamID, sc.currentIndex, action)
		}
	}

	// 迭代完成，关闭 channel
	sc.mu.Lock()
	if !sc.closed {
		sc.closed = true
		close(sc.actions)
	}
	sc.mu.Unlock()

	logx.WithContext(ctx).Debugf("Finished reading from storage, session_id=%s, stream_id=%s",
		sc.sessionID, sc.streamID)
}

// Send 发送 Action 到通道（StorageChannel 不支持发送，只支持从 storage 读取）
func (sc *StorageChannel) Send(ctx context.Context, action Action) error {
	return ErrChannelClosed // StorageChannel 是只读的
}

// Receive 从通道接收 Action
func (sc *StorageChannel) Receive(ctx context.Context) (Action, error) {
	sc.mu.RLock()
	closed := sc.closed
	sc.mu.RUnlock()

	if closed {
		return nil, ErrChannelClosed
	}

	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	case <-sc.stopChan:
		return nil, ErrChannelClosed
	case action, ok := <-sc.actions:
		if !ok {
			return nil, ErrChannelClosed
		}
		return action, nil
	}
}

// Close 关闭通道
func (sc *StorageChannel) Close() error {
	sc.mu.Lock()
	defer sc.mu.Unlock()

	if sc.closed {
		return nil
	}

	sc.closed = true
	close(sc.stopChan)
	close(sc.actions)

	// 等待读取 goroutine 完成
	sc.wg.Wait()

	return nil
}

// IsClosed 检查通道是否已关闭
func (sc *StorageChannel) IsClosed() bool {
	sc.mu.RLock()
	defer sc.mu.RUnlock()
	return sc.closed
}
