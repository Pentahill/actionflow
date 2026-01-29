package agent

import (
	"context"

	"github.com/Pentahill/actionflow/internal/protocol"
)

// 用于处理从 EventStream 获取的每个事件
// 如果返回错误，迭代器会停止
type EventHandler func(ctx context.Context, eventData any) error

// IterateEventStream 使用 iter.Seq2 迭代 EventStream 事件流的通用方法
// 如果 EventStream 调用失败或处理过程中出错则返回错误
func IterateEventStream(ctx context.Context, eventStream protocol.EventStream, handler EventHandler) error {
	// 调用 EventStream 获取事件流
	eventChan, err := eventStream(ctx)
	if err != nil {
		return err
	}

	// 使用 iter.Seq2 风格的迭代器处理事件流
	// 创建一个符合 iter.Seq2[any, bool] 签名的迭代器函数
	seq := func(yield func(any, bool) bool) {
		for {
			select {
			case <-ctx.Done():
				return
			case eventData, ok := <-eventChan:
				if !ok {
					// channel 已关闭，发送最后一个 false 值表示结束
					yield(nil, false)
					return
				}

				// 发送事件数据，第二个参数 true 表示还有更多数据
				if !yield(eventData, true) {
					// yield 返回 false，表示调用者希望停止迭代
					return
				}
			}
		}
	}

	// 使用迭代器遍历事件
	for eventData, ok := range seq {
		// 如果 channel 已关闭（ok == false），退出循环
		if !ok {
			return nil
		}

		// 调用处理函数处理事件
		if err := handler(ctx, eventData); err != nil {
			return err
		}
	}

	return nil
}
