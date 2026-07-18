package appconfig

import (
	"bytes"
	"strings"
	"testing"

	"github.com/lgc202/go-kit/logx"
)

func TestLoggingApplyDynamicChangesLevel(t *testing.T) {
	var output bytes.Buffer
	current := Logging{Format: logx.FormatJSON, Level: logx.LevelInfo, Stdout: true}
	logger, err := current.NewLogger("test-service", &output)
	if err != nil {
		t.Fatalf("Logging.NewLogger() error = %v", err)
	}
	t.Cleanup(func() {
		if err := logger.Close(); err != nil {
			t.Fatalf("Logger.Close() error = %v", err)
		}
	})

	logger.Debug("before level change")
	current.ApplyDynamic(Logging{Format: logx.FormatJSON, Level: logx.LevelDebug, Stdout: true}, logger)
	logger.Debug("after level change")

	if strings.Contains(output.String(), "before level change") {
		t.Fatalf("debug log was written before level change: %s", output.String())
	}
	if !strings.Contains(output.String(), "after level change") {
		t.Fatalf("debug log was not written after level change: %s", output.String())
	}
}

func TestLoggingApplyDynamicUsesLoggerCurrentLevel(t *testing.T) {
	var output bytes.Buffer
	current := Logging{Format: logx.FormatJSON, Level: logx.LevelInfo, Stdout: true}
	logger, err := current.NewLogger("test-service", &output)
	if err != nil {
		t.Fatalf("Logging.NewLogger() error = %v", err)
	}
	t.Cleanup(func() {
		if err := logger.Close(); err != nil {
			t.Fatalf("Logger.Close() error = %v", err)
		}
	})

	invalid := Logging{
		Format: logx.FormatJSON,
		Level:  logx.LevelDebug,
		Stdout: true,
		File:   LogFile{MaxSizeMB: -1},
	}
	if err := invalid.Validate(); err == nil {
		t.Fatal("Logging.Validate() error = nil, want invalid rotation value error")
	}
	if got := logger.Level(); got != logx.LevelInfo {
		t.Fatalf("Logger.Level() after invalid config = %q, want %q", got, logx.LevelInfo)
	}

	next := invalid
	next.File.MaxSizeMB = 0
	if err := next.Validate(); err != nil {
		t.Fatalf("Logging.Validate() error = %v", err)
	}

	// 无效配置会被加载器记为 old，但 logger 仍保持原级别
	invalid.ApplyDynamic(next, logger)
	logger.Debug("after corrected config")

	if got := logger.Level(); got != logx.LevelDebug {
		t.Fatalf("Logger.Level() = %q, want %q", got, logx.LevelDebug)
	}
	if !strings.Contains(output.String(), "after corrected config") {
		t.Fatalf("debug log was not written after corrected config: %s", output.String())
	}
}

func TestLoggingValidateRequiresOutput(t *testing.T) {
	config := Logging{Format: logx.FormatText, Level: logx.LevelInfo}
	if err := config.Validate(); err == nil {
		t.Fatal("Logging.Validate() error = nil, want disabled output error")
	}
}
