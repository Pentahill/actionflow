package channel

import (
	"context"
	"sync"
)

// contextKey 防止与其他包的 context key 冲突
type contextKey struct{ name string }

var (
	contextFieldsKey = &contextKey{"channel.fields"}
	passthroughKey   = &contextKey{"channel.passthrough"}
)

// CarrierPlugin 上下文载体插件接口。
// 实现此接口可在跨异步边界时注入/恢复框架上下文（如 OTel trace context、日志字段等）。
// 通过 RegisterCarrierPlugin 注册后，在投送侧与消费侧自动调用。
type CarrierPlugin interface {
	// Inject 投送侧：将 ctx 中持有的框架上下文写入 carrier
	Inject(ctx context.Context, carrier map[string]string)
	// Extract 消费侧：从 carrier 中恢复框架上下文，返回富化后的 ctx
	Extract(ctx context.Context, carrier map[string]string) context.Context
}

var (
	carrierPlugins   []CarrierPlugin
	carrierPluginsMu sync.RWMutex
)

// RegisterCarrierPlugin 注册一个上下文载体插件。
// 插件按注册顺序依次调用，允许注册多个插件（如同时集成 OTel 与 logx）。
// 建议在程序初始化阶段调用（非并发安全场景）。
func RegisterCarrierPlugin(p CarrierPlugin) {
	carrierPluginsMu.Lock()
	defer carrierPluginsMu.Unlock()
	carrierPlugins = append(carrierPlugins, p)
}

// ContextWithValue 在 ctx 中设置自定义键值对，用于跨异步边界传递（投送侧调用）。
// 多次调用会累积键值对，相同 key 后者覆盖前者。
func ContextWithValue(ctx context.Context, key, val string) context.Context {
	existing, _ := ctx.Value(contextFieldsKey).(map[string]string)
	next := make(map[string]string, len(existing)+1)
	for k, v := range existing {
		next[k] = v
	}
	next[key] = val
	return context.WithValue(ctx, contextFieldsKey, next)
}

// ValueFromContext 从 ctx 中读取指定 key 的自定义值（消费侧调用）。
func ValueFromContext(ctx context.Context, key string) (string, bool) {
	fields, _ := ctx.Value(contextFieldsKey).(map[string]string)
	v, ok := fields[key]
	return v, ok
}

// ContextWithPassthrough 在 ctx 中设置业务透传数据（投送侧调用）。
// data 须是可序列化的字节数组（建议使用 json.Marshal 编码）。
func ContextWithPassthrough(ctx context.Context, data []byte) context.Context {
	return context.WithValue(ctx, passthroughKey, data)
}

// PassthroughFromContext 从 ctx 中读取业务透传数据（消费侧调用）。
func PassthroughFromContext(ctx context.Context) ([]byte, bool) {
	data, ok := ctx.Value(passthroughKey).([]byte)
	return data, ok
}

// extractContextFields 提取 ctx 中所有通过 ContextWithValue 设置的自定义字段，
// 并依次调用已注册插件的 Inject 方法（如 OTel trace context 注入），
// 返回合并后的 carrier map（内部使用）。
func extractContextFields(ctx context.Context) map[string]string {
	fields, _ := ctx.Value(contextFieldsKey).(map[string]string)
	carrier := make(map[string]string, len(fields))
	for k, v := range fields {
		carrier[k] = v
	}

	carrierPluginsMu.RLock()
	plugins := make([]CarrierPlugin, len(carrierPlugins))
	copy(plugins, carrierPlugins)
	carrierPluginsMu.RUnlock()

	for _, p := range plugins {
		p.Inject(ctx, carrier)
	}
	return carrier
}

// restoreContextCarrier 将 carrier 中的字段恢复到 ctx 中（内部使用）：
//   - 将 carrier 写回 ctx 供 ValueFromContext 读取
//   - 依次调用已注册插件的 Extract 方法（如恢复 OTel trace context、注入 logx 字段）
func restoreContextCarrier(ctx context.Context, carrier map[string]string) context.Context {
	if len(carrier) == 0 {
		return ctx
	}

	// 将 carrier 字段写回 ctx，供 ValueFromContext 读取
	restored := make(map[string]string, len(carrier))
	for k, v := range carrier {
		restored[k] = v
	}
	ctx = context.WithValue(ctx, contextFieldsKey, restored)

	carrierPluginsMu.RLock()
	plugins := make([]CarrierPlugin, len(carrierPlugins))
	copy(plugins, carrierPlugins)
	carrierPluginsMu.RUnlock()

	for _, p := range plugins {
		ctx = p.Extract(ctx, carrier)
	}
	return ctx
}
