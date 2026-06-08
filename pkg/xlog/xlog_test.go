package xlog_test

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/lgc202/ingate/pkg/xlog"
)

func TestNewWritesJSONAtConfiguredLevel(t *testing.T) {
	var out bytes.Buffer
	logger, err := xlog.New(xlog.Options{
		Output: &out,
		Format: xlog.FormatJSON,
		Level:  xlog.LevelInfo,
	})
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	logger.Debug("debug hidden", "component", "test")
	logger.Info("service started", "component", "test")

	got := out.String()
	if strings.Contains(got, "debug hidden") {
		t.Fatalf("debug log was written at info level: %s", got)
	}
	if !strings.Contains(got, `"msg":"service started"`) {
		t.Fatalf("info log missing JSON message: %s", got)
	}
	if !strings.Contains(got, `"component":"test"`) {
		t.Fatalf("info log missing JSON attribute: %s", got)
	}
}

func TestLoggerSetLevelEnablesDebugAtRuntime(t *testing.T) {
	var out bytes.Buffer
	logger, err := xlog.New(xlog.Options{
		Output: &out,
		Format: xlog.FormatText,
		Level:  xlog.LevelInfo,
	})
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	logger.Debug("debug before")
	if out.Len() != 0 {
		t.Fatalf("debug log was written before level change: %s", out.String())
	}

	if err := logger.SetLevel(xlog.LevelDebug); err != nil {
		t.Fatalf("SetLevel() error = %v", err)
	}
	logger.Debug("debug after", "gateway", "public")

	got := out.String()
	if logger.Level() != xlog.LevelDebug {
		t.Fatalf("Level() = %q, want %q", logger.Level(), xlog.LevelDebug)
	}
	if !strings.Contains(got, "debug after") {
		t.Fatalf("debug log missing after level change: %s", got)
	}
	if !strings.Contains(got, "gateway=public") {
		t.Fatalf("debug log missing text attribute: %s", got)
	}
}

func TestNewRejectsUnsupportedLevel(t *testing.T) {
	_, err := xlog.New(xlog.Options{
		Output: &bytes.Buffer{},
		Level:  xlog.Level("trace"),
	})
	if err == nil {
		t.Fatal("New() error = nil")
	}
	if !strings.Contains(err.Error(), `unsupported log level "trace"`) {
		t.Fatalf("New() error = %v", err)
	}
}

func TestNewWritesToRotatedFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "ingate.log")
	logger, err := xlog.New(xlog.Options{
		Format: xlog.FormatText,
		Level:  xlog.LevelInfo,
		File: xlog.FileOptions{
			Path:       path,
			MaxSizeMB:  1,
			MaxBackups: 2,
			MaxAgeDays: 7,
		},
	})
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	defer logger.Close()

	logger.Info("file sink works", "component", "xlog")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}
	got := string(data)
	if !strings.Contains(got, "file sink works") {
		t.Fatalf("file log missing message: %s", got)
	}
	if !strings.Contains(got, "component=xlog") {
		t.Fatalf("file log missing attribute: %s", got)
	}
}

func TestLevelValueParsesFlagValue(t *testing.T) {
	level := xlog.LevelInfo
	value := (*xlog.LevelValue)(&level)

	if err := value.Set("debug"); err != nil {
		t.Fatalf("Set() error = %v", err)
	}
	if level != xlog.LevelDebug {
		t.Fatalf("level = %q, want %q", level, xlog.LevelDebug)
	}
	if value.String() != string(xlog.LevelDebug) {
		t.Fatalf("String() = %q, want %q", value.String(), xlog.LevelDebug)
	}
}

func TestFormatValueParsesFlagValue(t *testing.T) {
	format := xlog.FormatText
	value := (*xlog.FormatValue)(&format)

	if err := value.Set("json"); err != nil {
		t.Fatalf("Set() error = %v", err)
	}
	if format != xlog.FormatJSON {
		t.Fatalf("format = %q, want %q", format, xlog.FormatJSON)
	}
	if value.String() != string(xlog.FormatJSON) {
		t.Fatalf("String() = %q, want %q", value.String(), xlog.FormatJSON)
	}
}
