package agent

import (
	"context"
	"fmt"
	"reflect"
	"strings"
	"sync"

	"github.com/Pentahill/actionflow/internal/channel"

	"github.com/zeromicro/go-zero/core/logx"
)

const (
	AgentHandlerNamePrefix = "AgentHandler-"
)

// ActionHandler Agent 处理器接口
// 负责处理特定类型的 Action
type ActionHandler interface {
	channel.ActionHandler
	SupportedAction() []reflect.Type
	Name() string
}

// AgentRegistry Agent 注册表
// 负责注册和管理多个 AgentHandler，并根据 Action 类型路由到对应的 Handler
type AgentRegistry struct {
	dispatcher *channel.Dispatcher

	handlers             map[string]ActionHandler
	actionTypeToHandlers map[reflect.Type][]ActionHandler
	mu                   sync.RWMutex
}

// NewAgentRegistry 创建 Agent 注册表
func NewAgentRegistry(dispatcher *channel.Dispatcher) *AgentRegistry {
	registry := &AgentRegistry{
		dispatcher:           dispatcher,
		handlers:             make(map[string]ActionHandler),
		actionTypeToHandlers: make(map[reflect.Type][]ActionHandler),
	}

	// 注册 Task 类型的回调函数
	// AgentRegistry 处理所有类型的 Action（用于路由）
	dispatcher.Register(registry)

	// 注册所有默认的 AgentHandler
	registry.RegisterHandler(
		&EventStreamHandler{name: AgentHandlerNamePrefix + "EventStreamHandler", dispatcher: dispatcher},
		nil,
	)

	return registry
}

// Handle 处理 Action（实现 channel.ActionHandler 接口）
func (r *AgentRegistry) Handle(ctx context.Context, action channel.Action) error {
	sessionID := action.GetSessionID()
	if sessionID == "" {
		sessionID = "<empty>"
	}
	logx.WithContext(ctx).Debugf("Handling action, session_id=%s", sessionID)

	// 获取 action 的类型（保持指针类型，因为注册时使用的是指针类型）
	actionType := reflect.TypeOf(action)

	r.mu.RLock()
	// 查找支持该 action 类型的 handlers
	handlers, hasSpecificHandlers := r.actionTypeToHandlers[actionType]
	allHandlers := make([]ActionHandler, 0, len(r.handlers))
	for _, handler := range r.handlers {
		allHandlers = append(allHandlers, handler)
	}
	r.mu.RUnlock()

	// 如果有特定 handlers 支持该 action 类型，只调用这些 handlers
	// 如果没有，则调用所有 handlers
	targetHandlers := handlers
	if !hasSpecificHandlers || len(handlers) == 0 {
		logx.WithContext(ctx).Debugf("No specific handlers for action type, broadcasting to all handlers, session_id=%s, action_type=%s, total_handlers=%d",
			sessionID, actionType.String(), len(allHandlers))
		targetHandlers = allHandlers
	} else {
		logx.WithContext(ctx).Debugf("Routing to specific handlers, session_id=%s, action_type=%s, handlers=%d",
			sessionID, actionType.String(), len(handlers))
	}

	// 调用所有目标 handlers
	var lastErr error
	for i, handler := range targetHandlers {
		if err := handler.Handle(ctx, action); err != nil {
			logx.WithContext(ctx).Errorf("Handler[%d] error, session_id=%s, handler_name=%s, error=%v",
				i, sessionID, handler.Name(), err)
			lastErr = err
		}
	}

	if lastErr != nil {
		return fmt.Errorf("some handlers failed to handle action: %w", lastErr)
	}

	return nil
}

// Name 返回 Handler 名称（实现 channel.ActionHandlerSupportType 接口）
func (r *AgentRegistry) Name() string {
	return "AgentRegistry"
}

// SupportedAction 返回支持的 Action 类型（实现 channel.ActionHandlerSupportType 接口）
// 返回 nil 表示支持所有类型的 Action（用于路由）
func (r *AgentRegistry) SupportedAction() []reflect.Type {
	return nil // nil 表示支持所有类型
}

// RegisterHandler 注册 AgentHandler
// handler: 要注册的 handler
// supportedActionTypes: 该 handler 支持的 action 类型列表，如果为 nil 或空，则表示支持所有 action 类型
func (r *AgentRegistry) RegisterHandler(handler ActionHandler, supportedActionTypes []reflect.Type) {
	r.mu.Lock()
	defer r.mu.Unlock()

	// 注册 handler
	r.handlers[handler.Name()] = handler

	// 从 SupportedAction 方法中获取 actionType
	methodSupported := handler.SupportedAction()
	typesSet := make(map[reflect.Type]struct{})

	// 合并方法和参数的 actionType
	for _, t := range supportedActionTypes {
		if t != nil {
			typesSet[t] = struct{}{}
		}
	}
	for _, t := range methodSupported {
		if t != nil {
			typesSet[t] = struct{}{}
		}
	}

	if len(typesSet) > 0 {
		for actionType := range typesSet {
			r.actionTypeToHandlers[actionType] = append(r.actionTypeToHandlers[actionType], handler)
		}
		var typeNames []string
		for actionType := range typesSet {
			typeNames = append(typeNames, actionType.String())
		}
		logx.Debugf("Agent handler registered, name=%s, supported_types=%d, types=[%s]",
			handler.Name(), len(typesSet), strings.Join(typeNames, ", "))
	} else {
		logx.Debugf("Agent handler registered, name=%s, supports_all_actions=true", handler.Name())
	}
}

// EventStreamHandler 事件流处理器
// 处理 EventStreamCallAgentAction 类型的 Action
type EventStreamHandler struct {
	name       string
	dispatcher *channel.Dispatcher
}

func (h *EventStreamHandler) Name() string {
	return h.name
}

func (h *EventStreamHandler) SupportedAction() []reflect.Type {
	return []reflect.Type{reflect.TypeOf((*channel.EventStreamCallAgentAction)(nil))}
}

func (h *EventStreamHandler) Handle(ctx context.Context, action channel.Action) (err error) {
	eventStreamAction, ok := action.(*channel.EventStreamCallAgentAction)
	if !ok {
		return nil
	}

	sessionID := eventStreamAction.GetSessionID()
	eventStream := eventStreamAction.EventStream

	// 在 goroutine 中异步处理事件流
	go func() {
		defer func() {
			// 当事件流处理完成时，发送 Done 信号
			doneAction := channel.NewAgentResultActionDoneWithSessionID(sessionID, true, err)
			_ = h.dispatcher.Send(ctx, doneAction)
		}()

		// 使用通用的迭代方法处理事件流
		handler := func(ctx context.Context, eventData any) error {
			// 创建 AgentResultAction 并发送到 dispatcher
			resultAction := channel.NewAgentResultActionWithSessionID(sessionID, eventData)
			if err := h.dispatcher.Send(ctx, resultAction); err != nil {
				logx.WithContext(ctx).Errorf("Failed to send agent result action, session_id=%s, error=%v", sessionID, err)
				// 继续处理下一个事件，不因单个事件发送失败而中断
			}

			return nil
		}

		if err = IterateEventStream(ctx, eventStream, handler); err != nil {
			logx.WithContext(ctx).Errorf("Failed to iterate call handler events, session_id=%s, error=%v", sessionID, err)
		}
	}()

	return nil
}

// processTask 处理单个任务
// func (ac *AgentClient) processTask(ctx context.Context, task *AsyncTask) {
// 	logx.WithContext(ctx).Infof("[AGENT_CLIENT] Processing task - task_id=%s", task.TaskID)

// 	// 根据任务类型选择处理器
// 	var err error
// 	if task.CallHandler != nil {
// 		err = ac.handleCallHandler(ctx, task)
// 	} else {
// 		err = ac.handleHTTPRequest(ctx, task)
// 	}

// 	// 如果处理失败，发送错误结果
// 	if err != nil {
// 		result := NewTaskResultError(task.TaskID, err)
// 		if sendErr := ac.dispatcher.Send(ctx, result); sendErr != nil {
// 			logx.WithContext(ctx).Errorf("[AGENT_CLIENT] Failed to send error result - task_id=%s, error=%v", task.TaskID, sendErr)
// 		}
// 		return
// 	}

// 	// 发送完成结果
// 	result := NewTaskResultSuccess(task.TaskID, nil)
// 	if sendErr := ac.dispatcher.Send(ctx, result); sendErr != nil {
// 		logx.WithContext(ctx).Errorf("[AGENT_CLIENT] Failed to send success result - task_id=%s, error=%v", task.TaskID, sendErr)
// 	}
// }

// // handleHTTPRequest 处理 HTTP 请求
// func (ac *AgentClient) handleHTTPRequest(ctx context.Context, task *AsyncTask) error {
// 	headers := make(map[string]string, 0)
// 	if task.InferRequestToken != "" {
// 		headers["Authorization"] = task.InferRequestToken
// 	}

// 	// 使用 HTTP Transport 连接
// 	transport := transport.NewHTTPTransportForEventData(task.InferChatRequestURL, "POST", task.InferChatRequestBody, headers, task.InferRequestToken)
// 	conn, err := transport.Connect(ctx)
// 	if err != nil {
// 		logx.WithContext(ctx).Errorf("[AGENT_CLIENT] Failed to connect HTTP transport - task_id=%s, error=%v", task.TaskID, err)
// 		return err
// 	}
// 	defer conn.Close()

// 	for {
// 		// 每次判断是否任务还在运行，如果不在则退出
// 		exist, _ := ac.server.bizServer.Exists(task.Invest.ID)
// 		if !exist {
// 			logx.WithContext(ctx).Infof("[AGENT_CLIENT] Task is not exist, invest id: %v", task.Invest.ID)
// 			return nil
// 		}

// 		eventData, err := conn.Read(ctx)
// 		if err != nil {
// 			if err == io.EOF {
// 				return nil
// 			}
// 			logx.WithContext(ctx).Errorf("[AGENT_CLIENT] Failed to read from HTTP connection - task_id=%s, error=%v", task.TaskID, err)
// 			return err
// 		}

// 		if task.Config.InferServerConfig.Debug {
// 			logx.WithContext(ctx).Infof("[AGENT_CLIENT] Receive event data - task_id=%s, data=%v", task.TaskID, eventData)
// 		}

// 		// 将事件数据封装成 TaskResult 并发送到 Channel
// 		result := NewTaskResultEvent(task.TaskID, &eventData)
// 		if err := ac.dispatcher.Send(ctx, result); err != nil {
// 			logx.WithContext(ctx).Errorf("[AGENT_CLIENT] Failed to send event result - task_id=%s, error=%v", task.TaskID, err)
// 			return err
// 		}
// 	}
// }

// handleCallHandler 处理 CallHandler
// func (ac *AgentClient) handleCallHandler(ctx context.Context, task *AsyncTask) error {
// 	// 直接调用 CallHandler 获取事件流
// 	eventChan, err := task.CallHandler(ctx)
// 	if err != nil {
// 		logx.WithContext(ctx).Errorf("[AGENT_CLIENT] Failed to call handler - task_id=%s, error=%v", task.TaskID, err)
// 		return err
// 	}

// 	for {
// 		// // 每次判断是否任务还在运行，如果不在则退出
// 		// exist, _ := ac.server.bizServer.Exists(task.Invest.ID)
// 		// if !exist {
// 		// 	logx.WithContext(ctx).Infof("[AGENT_CLIENT] Task is not exist, invest id: %v", task.Invest.ID)
// 		// 	return nil
// 		// }

// 		select {
// 		case <-ctx.Done():
// 			return ctx.Err()
// 		case eventData, ok := <-eventChan:
// 			if !ok {
// 				// channel 关闭，任务完成
// 				return nil
// 			}

// 			if task.Config.InferServerConfig.Debug {
// 				logx.WithContext(ctx).Infof("[AGENT_CLIENT] Receive event data - task_id=%s, data=%v", task.TaskID, eventData)
// 			}

// 			// 将事件数据封装成 TaskResult 并发送到 Channel
// 			result := NewTaskResultEvent(task.TaskID, &eventData)
// 			if err := ac.dispatcher.Send(ctx, result); err != nil {
// 				logx.WithContext(ctx).Errorf("[AGENT_CLIENT] Failed to send event result - task_id=%s, error=%v", task.TaskID, err)
// 				return err
// 			}
// 		}
// 	}
// }
