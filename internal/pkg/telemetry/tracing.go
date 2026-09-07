package telemetry

import (
	"context"
	"crypto/tls"
	"fmt"
	"time"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/propagation"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	oteltrace "go.opentelemetry.io/otel/trace"
	"go.opentelemetry.io/otel/trace/noop"
)

// TraceConfig 定义 OTLP Trace 导出及其进程内缓冲边界。
type TraceConfig struct {
	// Enabled 控制是否创建 OTLP Exporter。
	Enabled bool
	// Endpoint 是 OTLP gRPC Collector 的 host:port 地址。
	Endpoint string
	// Insecure 控制 OTLP gRPC 是否使用明文连接。
	Insecure bool
	// TLS 是安全连接使用的 TLS 配置；为 nil 时使用系统根证书。
	TLS *tls.Config
	// SampleRatio 是无父 Span 的采样比例，取值范围为 [0, 1]。
	SampleRatio float64
	// QueueSize 是等待导出的 Span 数量上限。
	QueueSize int
	// BatchSize 是单次导出的最大 Span 数量。
	BatchSize int
	// BatchTimeout 是未满批次的最长等待时间。
	BatchTimeout time.Duration
	// ExportTimeout 是单次 OTLP 导出的最长时间。
	ExportTimeout time.Duration
}

// TraceDrops 是因本地容量或远端导出故障丢弃的 Span 计数。
type TraceDrops struct {
	// QueueFull 是本地导出队列已满时拒绝的 Span 数量。
	QueueFull uint64
}

// Tracing 提供进程使用的 TracerProvider、上下文传播和丢弃统计。
type Tracing struct {
	provider   oteltrace.TracerProvider
	propagator propagation.TextMapPropagator
	shutdown   func(context.Context) error
	drops      *traceDropCounters
}

// NewTracing 创建非阻塞的进程 Trace 出口。
// 未启用 OTLP 时不会建立网络连接，并返回可直接使用的空实现。
func NewTracing(ctx context.Context, config TraceConfig, identity Identity) (*Tracing, error) {
	propagator := propagation.NewCompositeTextMapPropagator(
		propagation.TraceContext{},
		propagation.Baggage{},
	)
	if !config.Enabled {
		provider := noop.NewTracerProvider()
		otel.SetTracerProvider(provider)
		otel.SetTextMapPropagator(propagator)
		return &Tracing{
			provider:   provider,
			propagator: propagator,
			shutdown:   func(context.Context) error { return nil },
			drops:      &traceDropCounters{},
		}, nil
	}

	exporter, err := newOTLPExporter(ctx, config)
	if err != nil {
		return nil, fmt.Errorf("create OTLP trace exporter: %w", err)
	}
	drops := &traceDropCounters{}
	buffer := newSpanBuffer(exporter, config, drops)
	provider := sdktrace.NewTracerProvider(
		sdktrace.WithResource(identity.Resource()),
		sdktrace.WithSampler(sdktrace.ParentBased(sdktrace.TraceIDRatioBased(config.SampleRatio))),
		sdktrace.WithSpanProcessor(buffer),
	)
	otel.SetTracerProvider(provider)
	otel.SetTextMapPropagator(propagator)
	return &Tracing{
		provider:   provider,
		propagator: propagator,
		shutdown:   provider.Shutdown,
		drops:      drops,
	}, nil
}

// Provider 返回当前进程的 TracerProvider。
func (t *Tracing) Provider() oteltrace.TracerProvider {
	return t.provider
}

// Propagator 返回进程统一使用的 W3C Trace Context 和 Baggage 传播器。
func (t *Tracing) Propagator() propagation.TextMapPropagator {
	return t.propagator
}

// Drops 返回当前累计的 Span 丢弃计数。
func (t *Tracing) Drops() TraceDrops {
	return TraceDrops{
		QueueFull: t.drops.queueFull.Load(),
	}
}

// Shutdown 在 ctx 期限内刷新并关闭 Trace 出口。
func (t *Tracing) Shutdown(ctx context.Context) error {
	return t.shutdown(ctx)
}
