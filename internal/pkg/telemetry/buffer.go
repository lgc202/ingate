package telemetry

import (
	"context"
	"sync/atomic"

	sdktrace "go.opentelemetry.io/otel/sdk/trace"
)

// traceDropCounters 在 Span 结束路径和指标采集之间共享计数。
type traceDropCounters struct {
	queueFull atomic.Uint64
}

// capacityExporter 在每批导出结束后归还缓冲容量。
type capacityExporter struct {
	sdktrace.SpanExporter
	pending chan struct{}
}

// spanBuffer 为官方 BatchSpanProcessor 补充可观测的非阻塞容量边界。
type spanBuffer struct {
	sdktrace.SpanProcessor
	pending chan struct{}
	drops   *traceDropCounters
	stopped atomic.Bool
}

func (e *capacityExporter) ExportSpans(ctx context.Context, spans []sdktrace.ReadOnlySpan) error {
	err := e.SpanExporter.ExportSpans(ctx, spans)
	for range spans {
		<-e.pending
	}
	return err
}

func (b *spanBuffer) OnEnd(span sdktrace.ReadOnlySpan) {
	if b.stopped.Load() {
		return
	}
	select {
	case b.pending <- struct{}{}:
		b.SpanProcessor.OnEnd(span)
	default:
		// ALS 的请求记录链路不能等待可观测性后端释放容量。
		b.drops.queueFull.Add(1)
	}
}

func (b *spanBuffer) Shutdown(ctx context.Context) error {
	b.stopped.Store(true)
	return b.SpanProcessor.Shutdown(ctx)
}

func newSpanBuffer(
	exporter sdktrace.SpanExporter,
	config TraceConfig,
	drops *traceDropCounters,
) sdktrace.SpanProcessor {
	// 容量令牌覆盖排队和正在导出的 Span，Collector 持续故障时也不会突破内存上限。
	pending := make(chan struct{}, config.QueueSize)
	exporter = &capacityExporter{
		SpanExporter: exporter,
		pending:      pending,
	}
	batch := sdktrace.NewBatchSpanProcessor(
		exporter,
		sdktrace.WithMaxQueueSize(config.QueueSize),
		sdktrace.WithMaxExportBatchSize(config.BatchSize),
		sdktrace.WithBatchTimeout(config.BatchTimeout),
		sdktrace.WithExportTimeout(config.ExportTimeout),
	)
	return &spanBuffer{
		SpanProcessor: batch,
		pending:       pending,
		drops:         drops,
	}
}
