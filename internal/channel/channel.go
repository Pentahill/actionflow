package channel

import (
	"context"
	"errors"
	"reflect"
	"runtime"
	"strings"
	"sync"

	"github.com/zeromicro/go-zero/core/logx"
)

// ErrChannelClosed 通道已关闭错误
var ErrChannelClosed = errors.New("channel is closed")

// ErrDispatcherClosed 分发器已关闭错误
var ErrDispatcherClosed = errors.New("dispatcher is closed")

// Channel 通道接口
// 只负责 Action 的发送和接收，不包含分发和消费逻辑
type Channel interface {
	// Send 发送 Action 到通道
	Send(ctx context.Context, action Action) error
	// Receive 从通道接收 Action
	Receive(ctx context.Context) (Action, error)
	// Close 关闭通道
	Close() error
	// IsClosed 检查通道是否已关闭
	IsClosed() bool
}

// DefaultChannel 异步通道实现
// 使用 buffered channel，支持非阻塞发送/接收
// 只负责 Action 的发送和接收，不包含分发和消费逻辑
// 分发和消费逻辑由 Dispatcher 负责
type DefaultChannel struct {
	// actions: Action 通道（所有 Action 都发送到这里，也从这里接收）
	actions chan Action

	// 通道配置
	bufferSize int

	// 关闭标志
	mu     sync.RWMutex
	closed bool
}

// NewDefaultChannel 创建异步通道
func NewDefaultChannel(bufferSize int) *DefaultChannel {
	if bufferSize <= 0 {
		bufferSize = 100 // 默认缓冲区大小
	}

	return &DefaultChannel{
		actions:    make(chan Action, bufferSize),
		bufferSize: bufferSize,
		closed:     false,
	}
}

// Send 发送 Action 到通道
func (c *DefaultChannel) Send(ctx context.Context, action Action) error {
	c.mu.RLock()
	closed := c.closed
	c.mu.RUnlock()

	if closed {
		return ErrChannelClosed
	}

	// 发送到 actions channel
	select {
	case <-ctx.Done():
		return ctx.Err()
	case c.actions <- action:
		return nil
	}
}

// Receive 从通道接收 Action
func (c *DefaultChannel) Receive(ctx context.Context) (Action, error) {
	c.mu.RLock()
	closed := c.closed
	c.mu.RUnlock()

	if closed {
		return nil, ErrChannelClosed
	}

	// 从 actions channel 接收
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	case action, ok := <-c.actions:
		if !ok {
			return nil, ErrChannelClosed
		}

		return action, nil
	}
}

// Close 关闭通道
func (c *DefaultChannel) Close() error {
	// c.mu.Lock()
	// defer c.mu.Unlock()

	if c.closed {
		return nil
	}

	c.closed = true
	close(c.actions)

	return nil
}

// IsClosed 检查通道是否已关闭
func (c *DefaultChannel) IsClosed() bool {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.closed
}

// Action 处理接口
type ActionHandler interface {
	// Handle 处理 Action
	Handle(ctx context.Context, action Action) error
}

// ActionHandlerFunc 函数类型，实现 ActionHandler 接口
// 方便将函数直接作为 Handler 使用
type ActionHandlerFunc func(ctx context.Context, action Action) error

// Handle 实现 ActionHandler 接口
func (f ActionHandlerFunc) Handle(ctx context.Context, action Action) error {
	return f(ctx, action)
}

// namedHandler 带名称和支持类型的 Handler
type namedHandler struct {
	handler        ActionHandler
	name           string
	supportedTypes []reflect.Type // 支持的 Action 类型列表，如果为空或 nil，表示支持所有类型
}

// Dispatcher Action 分发器
type Dispatcher struct {
	channel Channel

	handlers []*namedHandler
	// 保护 handlers 的并发访问
	handlersMu sync.RWMutex

	// 关闭标志
	mu     sync.RWMutex
	closed bool

	// 消费 goroutine 的停止信号
	stopChan chan struct{}
	wg       sync.WaitGroup
}

// NewDispatcher 创建 Action 分发器
// channel 是底层 Channel 实例，用于发送和接收 Action
func NewDispatcher(channel Channel) *Dispatcher {
	d := &Dispatcher{
		channel:  channel,
		handlers: make([]*namedHandler, 0),
		closed:   false,
		stopChan: make(chan struct{}),
	}

	// 启动消费 goroutine，直接从 Channel 接收 Action 并调用回调函数
	d.wg.Add(1)
	go d.consume()

	return d
}

// consume 消费 Action
func (d *Dispatcher) consume() {
	defer d.wg.Done()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// 当 stopChan 关闭时，取消 context
	go func() {
		<-d.stopChan
		cancel()
	}()

	for {
		// 检查是否已关闭
		d.mu.RLock()
		closed := d.closed
		d.mu.RUnlock()

		if closed {
			logx.WithContext(ctx).Debugf("Dispatcher is closed, stopping consume")
			return
		}

		// 从 Channel 接收 Action（阻塞调用，支持 context 取消）
		action, err := d.channel.Receive(ctx)
		if err != nil {
			if err == ErrChannelClosed || err == context.Canceled {
				return
			}

			// 其他错误，记录并继续
			logx.WithContext(ctx).Errorf("Failed to receive action from channel, error=%v", err)
			continue
		}

		// 获取 action 的类型（使用指针类型，因为注册时使用的是指针类型）
		actionType := reflect.TypeOf(action)

		// 获取该 ActionType 的所有回调函数
		d.handlersMu.RLock()
		allHandlers := make([]*namedHandler, len(d.handlers))
		copy(allHandlers, d.handlers)
		d.handlersMu.RUnlock()

		sessionID := action.GetSessionID()
		if sessionID == "" {
			sessionID = "<empty>"
		}

		// 根据 action 类型过滤 handlers
		var handlers []*namedHandler
		for _, nh := range allHandlers {
			// 如果 supportedTypes 为空或 nil，表示支持所有类型
			if len(nh.supportedTypes) == 0 {
				handlers = append(handlers, nh)
				continue
			}

			// 检查是否支持该 action 类型
			for _, supportedType := range nh.supportedTypes {
				if supportedType != nil {
					// 直接比较类型，或者检查 actionType 是否可以赋值给 supportedType
					if actionType == supportedType || actionType.AssignableTo(supportedType) {
						handlers = append(handlers, nh)
						break
					}
				}
			}
		}

		// 如果没有匹配的 handler，记录警告
		if len(handlers) == 0 {
			logx.WithContext(ctx).Debugf("No handler registered for action type, action_type=%s, session_id=%s", actionType.String(), sessionID)
			continue
		}

		// 逐个调用匹配的 handlers
		logx.WithContext(ctx).Debugf("Begin to dispatch action, action_type=%s, session_id=%s, handlers=%d", actionType.String(), sessionID, len(handlers))
		for _, nh := range handlers {
			if err := nh.handler.Handle(ctx, action); err != nil {
				logx.WithContext(ctx).Errorf("Handler error, handler_name=%s, session_id=%s, error=%v", nh.name, sessionID, err)
			}
		}
	}
}

// getHandlerName 自动生成 handler 名称
// 优先使用类型名称，如果是函数类型则尝试获取函数名
func getHandlerName(handler ActionHandler) string {
	if handler == nil {
		return "unknown"
	}

	// 使用 reflect 获取类型名称
	t := reflect.TypeOf(handler)
	if t == nil {
		return "unknown"
	}

	// 如果是函数类型（ActionHandlerFunc）
	if t.Kind() == reflect.Func {
		// 尝试获取函数名
		v := reflect.ValueOf(handler)
		if v.Kind() == reflect.Func && !v.IsNil() {
			pc := v.Pointer()
			if pc != 0 {
				fn := runtime.FuncForPC(pc)
				if fn != nil {
					funcName := fn.Name()
					if funcName != "" {
						// 提取函数名（去掉包路径）
						parts := strings.Split(funcName, ".")
						if len(parts) > 0 {
							name := parts[len(parts)-1]
							// 去掉可能的类型后缀
							if idx := strings.Index(name, "-"); idx > 0 {
								name = name[:idx]
							}
							return name
						}
						return funcName
					}
				}
			}
		}
		return "anonymous_func"
	}

	// 对于结构体类型，使用类型名称
	typeName := t.String()
	// 去掉包路径，只保留类型名
	parts := strings.Split(typeName, ".")
	if len(parts) > 0 {
		return parts[len(parts)-1]
	}
	return typeName
}

// Register 注册 Action 处理接口
// 优先支持 ActionHandlerSupportType 接口，可以直接传入实现了该接口的 handler
// 如果 handler 实现了 ActionHandlerSupportType，会自动从接口获取名称和支持的类型
// 否则需要手动传入 supportedTypes 和 name
// supportedTypes 和 name 为可选参数，如果 handler 实现了 ActionHandlerSupportType，这些参数会被忽略
func (d *Dispatcher) Register(handler ActionHandler, supportedTypesAndName ...interface{}) {
	if handler == nil {
		logx.Errorf("Cannot register nil handler")
		return
	}

	d.handlersMu.Lock()
	defer d.handlersMu.Unlock()

	// 检查是否已关闭
	d.mu.RLock()
	closed := d.closed
	d.mu.RUnlock()

	if closed {
		logx.Errorf("Cannot register handler, dispatcher is closed")
		return
	}

	var handlerName string
	var finalSupportedTypes []reflect.Type

	// 如果 handler 实现了 ActionHandlerSupportType 接口，优先使用其提供的信息
	if supportTypeHandler, ok := handler.(ActionHandlerSupportType); ok {
		// 使用接口提供的名称
		handlerName = supportTypeHandler.Name()
		// 使用接口提供的支持类型（如果接口返回 nil，表示支持所有类型）
		finalSupportedTypes = supportTypeHandler.SupportedAction()
	} else {
		// 如果没有实现 ActionHandlerSupportType，从可变参数中解析
		var supportedTypes []reflect.Type
		var name string

		// 解析可变参数
		for _, arg := range supportedTypesAndName {
			switch v := arg.(type) {
			case []reflect.Type:
				supportedTypes = v
			case string:
				name = v
			}
		}

		// 确定 handler 名称
		if name != "" {
			handlerName = name
		} else {
			handlerName = getHandlerName(handler)
		}
		finalSupportedTypes = supportedTypes
	}

	// 添加到对应类型的回调列表
	nh := &namedHandler{
		handler:        handler,
		name:           handlerName,
		supportedTypes: finalSupportedTypes,
	}
	d.handlers = append(d.handlers, nh)

	// 记录注册信息
	if len(finalSupportedTypes) > 0 {
		var typeNames []string
		for _, t := range finalSupportedTypes {
			if t != nil {
				typeNames = append(typeNames, t.String())
			}
		}
		logx.Debugf("Handler registered, handler_name=%s, supported_types=%d, types=[%s]", handlerName, len(finalSupportedTypes), strings.Join(typeNames, ", "))
	} else {
		logx.Debugf("Handler registered, handler_name=%s, supports_all_actions=true", handlerName)
	}
}

// GetActionType 获取 Action 类型的 reflect.Type
// 用于在注册 handler 时指定支持的 action 类型
func GetActionType[T Action]() reflect.Type {
	var zero T
	return reflect.TypeOf(zero)
}

// Close 关闭分发器
func (d *Dispatcher) Close() error {
	// d.mu.Lock()
	// defer d.mu.Unlock()

	if d.closed {
		return nil
	}

	d.closed = true
	// 发送停止信号给分发和消费 goroutine
	close(d.stopChan)
	// 等待所有 goroutine 完成
	d.wg.Wait()

	return nil
}

// IsClosed 检查分发器是否已关闭
func (d *Dispatcher) IsClosed() bool {
	d.mu.RLock()
	defer d.mu.RUnlock()
	return d.closed
}

// Send 发送 Action 到 Channel
// 这是对 Channel.Send 的封装，方便使用
// 如果 Dispatcher 已关闭，则拒绝发送新的 Action
func (d *Dispatcher) Send(ctx context.Context, action Action) error {
	// 检查是否已关闭
	d.mu.RLock()
	closed := d.closed
	d.mu.RUnlock()

	if closed {
		sessionID := action.GetSessionID()
		if sessionID == "" {
			sessionID = "<empty>"
		}
		logx.WithContext(ctx).Debugf("[DISPATCHER] Rejecting action: dispatcher is closed | session_id=%s", sessionID)
		return ErrDispatcherClosed
	}

	return d.channel.Send(ctx, action)
}
