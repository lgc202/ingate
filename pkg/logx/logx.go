// Package logx 提供基于标准库 slog 的项目日志入口
package logx

import (
	"fmt"
	"io"
	"log/slog"
	"strings"

	"gopkg.in/natefinch/lumberjack.v2"
)

// Format 表示日志输出格式
type Format string

const (
	// FormatText 表示适合本地阅读的文本日志
	FormatText Format = "text"
	// FormatJSON 表示适合生产采集的 JSON 日志
	FormatJSON            Format = "json"
	defaultFileMaxSizeMB  int    = 100
	defaultFileMaxBackups int    = 10
	defaultFileMaxAgeDays int    = 30
)

// Level 表示日志级别
type Level string

const (
	// LevelDebug 表示调试日志
	LevelDebug Level = "debug"
	// LevelInfo 表示普通运行日志
	LevelInfo Level = "info"
	// LevelWarn 表示可恢复异常日志
	LevelWarn Level = "warn"
	// LevelError 表示需要关注的错误日志
	LevelError Level = "error"
)

// Options 定义 logger 初始化参数
type Options struct {
	Output io.Writer
	Format Format
	Level  Level
	File   FileOptions
}

// FileOptions 定义文件日志和轮转参数
type FileOptions struct {
	Path       string
	MaxSizeMB  int
	MaxBackups int
	MaxAgeDays int
	Compress   bool
}

// FormatValue 让 Format 可直接作为命令行 flag 解析目标
type FormatValue Format

// LevelValue 让 Level 可直接作为命令行 flag 解析目标
type LevelValue Level

// Logger 包装 slog.Logger，并保留运行时可调整的级别
type Logger struct {
	*slog.Logger
	level  *slog.LevelVar
	closer io.Closer
}

// New 创建项目 logger
func New(options Options) (*Logger, error) {
	output, closer := buildOutput(options.Output, options.File)
	if output == nil {
		output = io.Discard
	}

	level, err := parseSlogLevel(options.Level)
	if err != nil {
		return nil, err
	}
	levelVar := new(slog.LevelVar)
	levelVar.Set(level)

	handlerOptions := &slog.HandlerOptions{Level: levelVar}
	var handler slog.Handler
	switch options.Format {
	case "", FormatText:
		handler = slog.NewTextHandler(output, handlerOptions)
	case FormatJSON:
		handler = slog.NewJSONHandler(output, handlerOptions)
	default:
		return nil, fmt.Errorf("unsupported log format %q", options.Format)
	}

	return &Logger{
		Logger: slog.New(handler),
		level:  levelVar,
		closer: closer,
	}, nil
}

// SetLevel 在运行时调整日志级别
func (l *Logger) SetLevel(level Level) error {
	slogLevel, err := parseSlogLevel(level)
	if err != nil {
		return err
	}
	l.level.Set(slogLevel)
	return nil
}

// Level 返回当前日志级别
func (l *Logger) Level() Level {
	switch l.level.Level() {
	case slog.LevelDebug:
		return LevelDebug
	case slog.LevelInfo:
		return LevelInfo
	case slog.LevelWarn:
		return LevelWarn
	case slog.LevelError:
		return LevelError
	default:
		return Level(l.level.Level().String())
	}
}

// Close 关闭 logger 持有的文件输出
func (l *Logger) Close() error {
	if l.closer == nil {
		return nil
	}
	return l.closer.Close()
}

func buildOutput(output io.Writer, file FileOptions) (io.Writer, io.Closer) {
	if file.Path == "" {
		return output, nil
	}
	if file.MaxSizeMB == 0 {
		file.MaxSizeMB = defaultFileMaxSizeMB
	}
	if file.MaxBackups == 0 {
		file.MaxBackups = defaultFileMaxBackups
	}
	if file.MaxAgeDays == 0 {
		file.MaxAgeDays = defaultFileMaxAgeDays
	}

	rotatedFile := &lumberjack.Logger{
		Filename:   file.Path,
		MaxSize:    file.MaxSizeMB,
		MaxBackups: file.MaxBackups,
		MaxAge:     file.MaxAgeDays,
		Compress:   file.Compress,
	}
	if output == nil {
		return rotatedFile, rotatedFile
	}
	return io.MultiWriter(output, rotatedFile), rotatedFile
}

func parseSlogLevel(level Level) (slog.Level, error) {
	switch Level(strings.ToLower(string(level))) {
	case "", LevelInfo:
		return slog.LevelInfo, nil
	case LevelDebug:
		return slog.LevelDebug, nil
	case LevelWarn:
		return slog.LevelWarn, nil
	case LevelError:
		return slog.LevelError, nil
	default:
		return slog.LevelInfo, fmt.Errorf("unsupported log level %q", level)
	}
}

// Set 解析日志输出格式
func (f *FormatValue) Set(value string) error {
	format := Format(strings.ToLower(value))
	switch format {
	case FormatText, FormatJSON:
		*f = FormatValue(format)
		return nil
	default:
		return fmt.Errorf("unsupported log format %q", value)
	}
}

// String 返回日志输出格式
func (f *FormatValue) String() string {
	if f == nil || *f == "" {
		return string(FormatText)
	}
	return string(*f)
}

// Type 返回命令行 flag 类型
func (f *FormatValue) Type() string {
	return "log-format"
}

// Set 解析日志级别
func (l *LevelValue) Set(value string) error {
	level := Level(strings.ToLower(value))
	if _, err := parseSlogLevel(level); err != nil {
		return err
	}
	*l = LevelValue(level)
	return nil
}

// String 返回日志级别
func (l *LevelValue) String() string {
	if l == nil || *l == "" {
		return string(LevelInfo)
	}
	return string(*l)
}

// Type 返回命令行 flag 类型
func (l *LevelValue) Type() string {
	return "log-level"
}
