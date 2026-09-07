package telemetry

import (
	"context"
	"encoding/json"
	"log/slog"
	"os"
	"testing"

	oteltrace "go.opentelemetry.io/otel/trace"
)

type testLoggerConfig struct{}

func (testLoggerConfig) GetFormat() string  { return "json" }
func (testLoggerConfig) GetLevel() string   { return "info" }
func (testLoggerConfig) GetAddSource() bool { return false }

// TestNewLoggerAddsIdentityAndTraceContext 验证日志固定身份字段和调用链字段。
func TestNewLoggerAddsIdentityAndTraceContext(t *testing.T) {
	output, err := os.CreateTemp(t.TempDir(), "telemetry-log-*.json")
	if err != nil {
		t.Fatalf("os.CreateTemp() error: %v", err)
	}
	t.Cleanup(func() { _ = output.Close() })

	stderr := os.Stderr
	var logger *slog.Logger
	func() {
		os.Stderr = output
		defer func() { os.Stderr = stderr }()
		logger = NewLogger(testLoggerConfig{}, Identity{
			Namespace:   "ingate",
			Name:        "ingate-als",
			InstanceID:  "instance-1",
			Version:     "v1.2.3",
			Environment: "test",
			Hostname:    "host-1",
		})
	}()

	traceID, err := oteltrace.TraceIDFromHex("0102030405060708090a0b0c0d0e0f10")
	if err != nil {
		t.Fatalf("TraceIDFromHex() error: %v", err)
	}
	spanID, err := oteltrace.SpanIDFromHex("0102030405060708")
	if err != nil {
		t.Fatalf("SpanIDFromHex() error: %v", err)
	}
	ctx := oteltrace.ContextWithSpanContext(context.Background(), oteltrace.NewSpanContext(
		oteltrace.SpanContextConfig{TraceID: traceID, SpanID: spanID},
	))
	logger.InfoContext(ctx, "test message")

	if _, err := output.Seek(0, 0); err != nil {
		t.Fatalf("output.Seek(0, 0) error: %v", err)
	}
	var record map[string]any
	if err := json.NewDecoder(output).Decode(&record); err != nil {
		t.Fatalf("Decode(log record) error: %v", err)
	}
	want := map[string]string{
		"service.namespace":           "ingate",
		"service.name":                "ingate-als",
		"service.instance.id":         "instance-1",
		"service.version":             "v1.2.3",
		"deployment.environment.name": "test",
		"host.name":                   "host-1",
		"trace_id":                    traceID.String(),
		"span_id":                     spanID.String(),
	}
	for key, wantValue := range want {
		if got := record[key]; got != wantValue {
			t.Errorf("NewLogger() record[%q] = %v, want %q", key, got, wantValue)
		}
	}
}
