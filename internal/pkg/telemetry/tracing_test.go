package telemetry

import (
	"context"
	"errors"
	"testing"
	"time"

	"go.opentelemetry.io/otel"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
)

type blockingExporter struct {
	started chan struct{}
	release chan struct{}
	done    chan struct{}
}

func (e *blockingExporter) ExportSpans(ctx context.Context, _ []sdktrace.ReadOnlySpan) error {
	close(e.started)
	defer close(e.done)
	select {
	case <-e.release:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

func (*blockingExporter) Shutdown(context.Context) error { return nil }

// TestNewTracingDisabledUsesNoopProvider 验证未启用 OTLP 时不会记录或导出 Span。
func TestNewTracingDisabledUsesNoopProvider(t *testing.T) {
	previousProvider := otel.GetTracerProvider()
	previousPropagator := otel.GetTextMapPropagator()
	t.Cleanup(func() {
		otel.SetTracerProvider(previousProvider)
		otel.SetTextMapPropagator(previousPropagator)
	})

	tracing, err := NewTracing(t.Context(), TraceConfig{}, Identity{})
	if err != nil {
		t.Fatalf("NewTracing(disabled) error: %v", err)
	}
	_, span := tracing.Provider().Tracer(t.Name()).Start(t.Context(), "noop")
	if span.IsRecording() {
		t.Error("NewTracing(disabled) span is recording, want noop provider")
	}
	if err := tracing.Shutdown(t.Context()); err != nil {
		t.Errorf("Tracing.Shutdown(disabled) error: %v", err)
	}
}

// TestBatchProcessorDropsWithoutBlockingWhenFull 验证导出容量耗尽时主链路不会阻塞。
func TestBatchProcessorDropsWithoutBlockingWhenFull(t *testing.T) {
	exporter := &blockingExporter{
		started: make(chan struct{}),
		release: make(chan struct{}),
		done:    make(chan struct{}),
	}
	drops := &traceDropCounters{}
	processor := newSpanBuffer(exporter, testTraceConfig(), drops)
	provider := sdktrace.NewTracerProvider(
		sdktrace.WithSampler(sdktrace.AlwaysSample()),
		sdktrace.WithSpanProcessor(processor),
	)

	_, first := provider.Tracer(t.Name()).Start(t.Context(), "first")
	first.End()
	select {
	case <-exporter.started:
	case <-time.After(time.Second):
		t.Fatal("first span was not handed to the exporter within one second")
	}

	_, second := provider.Tracer(t.Name()).Start(t.Context(), "second")
	ended := make(chan struct{})
	go func() {
		second.End()
		close(ended)
	}()
	select {
	case <-ended:
	case <-time.After(100 * time.Millisecond):
		close(exporter.release)
		t.Fatal("second span blocked on a full export pipeline")
	}

	if got, want := drops.queueFull.Load(), uint64(1); got != want {
		t.Errorf("queue-full drops = %d, want %d", got, want)
	}
	close(exporter.release)
	shutdownTraceProvider(t, provider)
}

// TestBatchProcessorShutdownHonorsContext 验证关闭过程遵守调用方的超时边界。
func TestBatchProcessorShutdownHonorsContext(t *testing.T) {
	exporter := &blockingExporter{
		started: make(chan struct{}),
		release: make(chan struct{}),
		done:    make(chan struct{}),
	}
	processor := newSpanBuffer(exporter, testTraceConfig(), &traceDropCounters{})
	provider := sdktrace.NewTracerProvider(
		sdktrace.WithSampler(sdktrace.AlwaysSample()),
		sdktrace.WithSpanProcessor(processor),
	)

	_, span := provider.Tracer(t.Name()).Start(t.Context(), "blocked-export")
	span.End()
	select {
	case <-exporter.started:
	case <-time.After(time.Second):
		t.Fatal("span was not handed to the exporter within one second")
	}

	ctx, cancel := context.WithTimeout(t.Context(), 100*time.Millisecond)
	start := time.Now()
	err := provider.Shutdown(ctx)
	cancel()
	close(exporter.release)
	select {
	case <-exporter.done:
	case <-time.After(time.Second):
		t.Error("blocked exporter did not exit after release")
	}
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Errorf("TracerProvider.Shutdown() error = %v, want context deadline exceeded", err)
	}
	if elapsed := time.Since(start); elapsed >= time.Second {
		t.Errorf("TracerProvider.Shutdown() took %v, want less than one second", elapsed)
	}
}

func testTraceConfig() TraceConfig {
	return TraceConfig{
		QueueSize:     1,
		BatchSize:     1,
		BatchTimeout:  time.Hour,
		ExportTimeout: time.Minute,
	}
}

func shutdownTraceProvider(t *testing.T, provider *sdktrace.TracerProvider) {
	t.Helper()
	ctx, cancel := context.WithTimeout(t.Context(), time.Second)
	defer cancel()
	if err := provider.Shutdown(ctx); err != nil {
		t.Errorf("TracerProvider.Shutdown() error: %v", err)
	}
}
