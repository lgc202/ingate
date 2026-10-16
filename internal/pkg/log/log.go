// Package log 配置普通服务的原生 slog 输出及轮转文件，不替换日志调用 API。
package log

import (
	"errors"
	"fmt"
	"io"
	"log/slog"
	"os"
	"slices"

	"gopkg.in/natefinch/lumberjack.v2"
)

// Format 表示日志编码格式；空值使用 JSON，未知值由 New 拒绝。
type Format string

const (
	// FormatJSON 使用结构化 JSON 编码。
	FormatJSON Format = "json"
	// FormatText 使用文本键值编码。
	FormatText Format = "text"
)

// Output 表示日志输出目标；命名类型不保证取值合法，未知值由 New 拒绝。
type Output string

const (
	// OutputStdout 将日志写入进程标准输出。
	OutputStdout Output = "stdout"
	// OutputStderr 将日志写入进程标准错误输出。
	OutputStderr Output = "stderr"
	// OutputFile 将日志写入配置的轮转文件。
	OutputFile Output = "file"
)

// Config 配置启动时固定的日志级别、格式、输出和调用位置。
// 零值使用 info、JSON 和 stdout；文件输出必须显式选择。
type Config struct {
	Level     string
	Format    Format
	Outputs   []Output
	AddSource bool
	File      FileConfig
}

// FileConfig 配置一个进程独占的轮转文件。
// MaxSizeMB 为零时使用 100 MiB；MaxBackups 和 MaxAgeDays 为零时不按该维度清理。
type FileConfig struct {
	Path       string
	MaxSizeMB  int
	MaxBackups int
	MaxAgeDays int
	Compress   bool
}

// New 创建原生 Logger，不设置进程默认 Logger，并返回文件输出的关闭函数。
// 文件在首次写入时打开；关闭函数不关闭标准流，也不等待后台压缩和清理完成。
func New(config Config) (*slog.Logger, func() error, error) {
	return newLogger(config, os.Stdout, os.Stderr)
}

func newLogger(config Config, stdout, stderr io.Writer) (*slog.Logger, func() error, error) {
	var level slog.Level
	if config.Level != "" {
		if err := level.UnmarshalText([]byte(config.Level)); err != nil {
			return nil, nil, fmt.Errorf("parse log level: %w", err)
		}
	}

	if config.Format != "" && config.Format != FormatJSON && config.Format != FormatText {
		return nil, nil, fmt.Errorf("unsupported log format %q", config.Format)
	}

	outputs := config.Outputs
	if len(outputs) == 0 {
		outputs = []Output{OutputStdout}
	}

	options := &slog.HandlerOptions{Level: level, AddSource: config.AddSource}
	handlers := make([]slog.Handler, 0, len(outputs))
	closeOutput := func() error { return nil }
	for i, output := range outputs {
		if slices.Contains(outputs[:i], output) {
			return nil, nil, fmt.Errorf("duplicate log output %q", output)
		}

		var writer io.Writer

		switch output {
		case OutputStdout:
			writer = stdout
		case OutputStderr:
			writer = stderr
		case OutputFile:
			if config.File.Path == "" {
				return nil, nil, errors.New("log file path is required")
			}

			if config.File.MaxSizeMB < 0 || config.File.MaxBackups < 0 || config.File.MaxAgeDays < 0 {
				return nil, nil, errors.New("log file rotation limits must not be negative")
			}

			// lumberjack 打开已有路径失败后可能轮转，避免把目录等非普通文件移走。
			info, err := os.Stat(config.File.Path)
			if err != nil && !errors.Is(err, os.ErrNotExist) {
				return nil, nil, fmt.Errorf("inspect log file path: %w", err)
			}

			if err == nil && !info.Mode().IsRegular() {
				return nil, nil, fmt.Errorf("log file path %q must name a regular file", config.File.Path)
			}

			// 这里只组装 Writer；后续配置失败时尚未打开文件或启动后台清理。
			file := &lumberjack.Logger{
				Filename:   config.File.Path,
				MaxSize:    config.File.MaxSizeMB,
				MaxBackups: config.File.MaxBackups,
				MaxAge:     config.File.MaxAgeDays,
				Compress:   config.File.Compress,
			}
			writer = file
			closeOutput = file.Close
		default:
			return nil, nil, fmt.Errorf("unsupported log output %q", output)
		}

		if config.Format == FormatText {
			handlers = append(handlers, slog.NewTextHandler(writer, options))
		} else {
			handlers = append(handlers, slog.NewJSONHandler(writer, options))
		}
	}

	return slog.New(slog.NewMultiHandler(handlers...)), closeOutput, nil
}
