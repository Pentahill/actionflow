// Package plugin 提供 channel.CarrierPlugin 的框架集成实现。
// 各插件按需 import、按需注册，不引入对 channel 基础包的额外外部依赖。
//
// 使用示例（程序初始化时）：
//
//	channel.RegisterCarrierPlugin(plugin.OtelPlugin{})
//	channel.RegisterCarrierPlugin(plugin.LogxPlugin{})
package plugin

import (
	"context"

	"github.com/Pentahill/actionflow/internal/channel"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/propagation"
)

// OtelPlugin 基于 OpenTelemetry 的上下文载体插件，实现 channel.CarrierPlugin 接口。
// 投送侧通过全局 TextMapPropagator 将 trace context 序列化到 carrier；
// 消费侧从 carrier 中反序列化，使 handler 可获取原始 span context 并创建子 span。
//
// 若未配置全局 propagator（otel.SetTextMapPropagator），两个方法均为 no-op，不影响正确性。
// 典型初始化：
//
//	otel.SetTextMapPropagator(propagation.NewCompositeTextMapPropagator(
//	    propagation.TraceContext{},
//	    propagation.Baggage{},
//	))
//	channel.RegisterCarrierPlugin(plugin.OtelPlugin{})
type OtelPlugin struct{}

var _ channel.CarrierPlugin = OtelPlugin{}

// Inject 将当前 span 的 trace context 写入 carrier。
func (OtelPlugin) Inject(ctx context.Context, carrier map[string]string) {
	otel.GetTextMapPropagator().Inject(ctx, propagation.MapCarrier(carrier))
}

// Extract 从 carrier 中恢复 trace context 到 ctx。
func (OtelPlugin) Extract(ctx context.Context, carrier map[string]string) context.Context {
	return otel.GetTextMapPropagator().Extract(ctx, propagation.MapCarrier(carrier))
}
