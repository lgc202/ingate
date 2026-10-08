package log

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"strings"
	"testing"
	"time"
)

type failingWriter struct{}

func TestNewDoesNotSetDefaultLogger(t *testing.T) {
	before := slog.Default()
	logger, _, err := New(Config{Level: "debug"})
	if err != nil {
		t.Fatalf("New(debug) error = %v, want nil", err)
	}
	if logger == nil || !logger.Enabled(t.Context(), slog.LevelDebug) {
		t.Error("New(debug) logger does not enable debug, want native configured Logger")
	}
	if slog.Default() != before {
		t.Error("New(debug) replaced process default Logger, want unchanged")
	}
}

func TestDefaultJSONOutputFiltersDebug(t *testing.T) {
	var stdout, stderr bytes.Buffer
	logger, _, err := newLogger(Config{}, &stdout, &stderr)
	if err != nil {
		t.Fatalf("newLogger(defaults) error = %v, want nil", err)
	}
	logger.Debug("filtered")
	logger.With("service", "test-service").Info("visible", "count", 3)
	var record map[string]any
	if err := json.Unmarshal(stdout.Bytes(), &record); err != nil {
		t.Fatalf("default stdout JSON = %q, error = %v, want one JSON record", stdout.String(), err)
	}
	if record["level"] != "INFO" || record["msg"] != "visible" || record["service"] != "test-service" || record["count"] != float64(3) {
		t.Errorf("default record = %v, want INFO, visible, test-service and count 3", record)
	}
	if _, found := record["source"]; found {
		t.Error("default record includes source, want omitted")
	}
	if stderr.Len() != 0 {
		t.Errorf("default stderr = %q, want empty", stderr.String())
	}
}

func TestConfiguredLevels(t *testing.T) {
	tests := []struct {
		level string
		want  slog.Level
	}{
		{level: "debug", want: slog.LevelDebug},
		{level: "info", want: slog.LevelInfo},
		{level: "warn", want: slog.LevelWarn},
		{level: "error", want: slog.LevelError},
	}

	for _, tt := range tests {
		t.Run(tt.level, func(t *testing.T) {
			var stdout, stderr bytes.Buffer
			logger, _, err := newLogger(Config{Level: tt.level}, &stdout, &stderr)
			if err != nil {
				t.Fatalf("newLogger(level=%q) error = %v, want nil", tt.level, err)
			}
			logger.Log(t.Context(), tt.want-1, "filtered")
			logger.Log(t.Context(), tt.want, "visible")
			var record map[string]any
			if err := json.Unmarshal(stdout.Bytes(), &record); err != nil {
				t.Fatalf("level=%q output = %q, error = %v, want one record", tt.level, stdout.String(), err)
			}
			if record["level"] != tt.want.String() || record["msg"] != "visible" {
				t.Errorf("level=%q record = %v, want visible at %s", tt.level, record, tt.want)
			}
		})
	}
}

func TestTextStderrAndSource(t *testing.T) {
	var stdout, stderr bytes.Buffer
	logger, _, err := newLogger(Config{Format: FormatText, Outputs: []Output{OutputStderr}, AddSource: true}, &stdout, &stderr)
	if err != nil {
		t.Fatalf("newLogger(text stderr, source) error = %v, want nil", err)
	}
	logger.Info("visible", "count", 3)
	if stdout.Len() != 0 || !strings.Contains(stderr.String(), "level=INFO") || !strings.Contains(stderr.String(), "count=3") || !strings.Contains(stderr.String(), "log_test.go:") {
		t.Errorf("text stderr/source stdout/stderr = %q/%q, want empty stdout and INFO, count, source in stderr", stdout.String(), stderr.String())
	}
}

func TestJSONSourceAndMultipleOutputs(t *testing.T) {
	var stdout, stderr bytes.Buffer
	logger, _, err := newLogger(Config{Outputs: []Output{OutputStdout, OutputStderr}, AddSource: true}, &stdout, &stderr)
	if err != nil {
		t.Fatalf("newLogger(multiple outputs, source) error = %v, want nil", err)
	}
	logger.Info("visible")
	if stdout.String() != stderr.String() {
		t.Errorf("multiple outputs stdout/stderr = %q/%q, want identical records", stdout.String(), stderr.String())
	}
	var record struct {
		Source slog.Source `json:"source"`
	}
	if err := json.Unmarshal(stdout.Bytes(), &record); err != nil {
		t.Fatalf("multiple output JSON = %q, error = %v, want one record", stdout.String(), err)
	}
	if !strings.HasSuffix(record.Source.File, "log_test.go") || record.Source.Line <= 0 || !strings.HasSuffix(record.Source.Function, ".TestJSONSourceAndMultipleOutputs") {
		t.Errorf("record source = %+v, want this test function, file and positive line", record.Source)
	}
}

func TestOutputFailureDoesNotStopOtherOutput(t *testing.T) {
	var stderr bytes.Buffer
	logger, _, err := newLogger(Config{Outputs: []Output{OutputStdout, OutputStderr}}, failingWriter{}, &stderr)
	if err != nil {
		t.Fatalf("newLogger(multiple outputs) error = %v, want nil", err)
	}
	record := slog.NewRecord(time.Now(), slog.LevelInfo, "visible", 0)
	if err := logger.Handler().Handle(t.Context(), record); !errors.Is(err, io.ErrClosedPipe) {
		t.Errorf("Handle(failed stdout) error = %v, want io.ErrClosedPipe", err)
	}
	if !strings.Contains(stderr.String(), `"msg":"visible"`) {
		t.Errorf("Handle(failed stdout) stderr = %q, want visible record", stderr.String())
	}
}

func TestNewRejectsInvalidConfig(t *testing.T) {
	tests := []struct {
		name   string
		config Config
	}{
		{name: "level", config: Config{Level: "invalid"}},
		{name: "format", config: Config{Format: "invalid"}},
		{name: "output", config: Config{Outputs: []Output{"invalid"}}},
		{name: "duplicate output", config: Config{Outputs: []Output{OutputStdout, OutputStdout}}},
		{name: "missing file path", config: Config{Outputs: []Output{OutputFile}}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			logger, closeLog, err := New(tt.config)
			if logger != nil || closeLog != nil || err == nil {
				t.Errorf("New(%s) = %v, close present=%t, error=%v, want nil/nil/error", tt.name, logger, closeLog != nil, err)
			}
		})
	}
}

func (failingWriter) Write([]byte) (int, error) {
	return 0, io.ErrClosedPipe
}
