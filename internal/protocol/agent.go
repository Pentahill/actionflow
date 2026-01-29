package protocol

import "context"

// EventStream 事件流提供者类型
// 用于从上下文获取事件流，通常用于调用 agent 并获取其返回的事件序列
type EventStream func(ctx context.Context) (<-chan any, error)
