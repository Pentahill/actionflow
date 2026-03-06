package plugin

import (
	"context"

	"github.com/Pentahill/actionflow/internal/channel"
	"github.com/zeromicro/go-zero/core/logx"
)

// LogxPlugin 基于 go-zero logx 的上下文载体插件，实现 channel.CarrierPlugin 接口。
// 消费侧将 carrier 中的所有键值作为结构化字段注入 logx context，
// 使 logx.WithContext(ctx) 打出的日志自动携带 trace_id、request_id 等字段，
// 无需修改任何业务日志调用处。
//
// 使用示例（程序初始化时）：
//
//	channel.RegisterCarrierPlugin(plugin.LogxPlugin{})
type LogxPlugin struct{}

var _ channel.CarrierPlugin = LogxPlugin{}

// Inject logx 不需要向 carrier 写入任何内容。
func (LogxPlugin) Inject(_ context.Context, _ map[string]string) {}

// Extract 将 carrier 中的字段作为结构化字段注入 logx context。
func (LogxPlugin) Extract(ctx context.Context, carrier map[string]string) context.Context {
	if len(carrier) == 0 {
		return ctx
	}
	fields := make([]logx.LogField, 0, len(carrier))
	for k, v := range carrier {
		fields = append(fields, logx.Field(k, v))
	}
	return logx.ContextWithFields(ctx, fields...)
}
