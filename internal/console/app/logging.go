package app

import (
	"errors"
	"fmt"
	"io"

	"github.com/lgc202/go-kit/logx"
	"github.com/lgc202/go-kit/version"
)

// Logging 定义 Console 独有的动态日志和文件轮转配置
//
// 其他组件由 Kratos 管理日志，Console 仍使用 go-kit config watch，因此这套配置
// 留在 Console 内部，避免把单组件实现伪装成全局公共能力
type Logging struct {
	Format    logx.Format `json:"format" mapstructure:"format"`
	Level     logx.Level  `json:"level" mapstructure:"level"`
	AddSource bool        `json:"add_source" mapstructure:"add_source"`
	Stdout    bool        `json:"stdout" mapstructure:"stdout"`
	File      LogFile     `json:"file" mapstructure:"file"`
}

// LogFile 定义 Console 文件日志和轮转配置
type LogFile struct {
	Path       string `json:"path" mapstructure:"path"`
	MaxSizeMB  int    `json:"max_size_mb" mapstructure:"max_size_mb"`
	MaxBackups int    `json:"max_backups" mapstructure:"max_backups"`
	MaxAgeDays int    `json:"max_age_days" mapstructure:"max_age_days"`
	Compress   bool   `json:"compress" mapstructure:"compress"`
	LocalTime  bool   `json:"local_time" mapstructure:"local_time"`
}

// Validate 校验 Console 日志配置
func (c Logging) Validate() error {
	switch c.Format {
	case logx.FormatText, logx.FormatJSON:
	default:
		return fmt.Errorf("unsupported log format %q", c.Format)
	}
	switch c.Level {
	case logx.LevelDebug, logx.LevelInfo, logx.LevelWarn, logx.LevelError:
	default:
		return fmt.Errorf("unsupported log level %q", c.Level)
	}
	if !c.Stdout && c.File.Path == "" {
		return errors.New("logging stdout and file cannot both be disabled")
	}
	if c.File.MaxSizeMB < 0 || c.File.MaxBackups < 0 || c.File.MaxAgeDays < 0 {
		return errors.New("log file rotation values must not be negative")
	}
	return nil
}

// NewLogger 根据 Console 配置创建 logger
func (c Logging) NewLogger(stdout io.Writer) (*logx.Logger, error) {
	options := []logx.Option{
		logx.WithFormat(c.Format),
		logx.WithLevel(c.Level),
		logx.WithAddSource(c.AddSource),
		logx.WithService("ingate-console"),
		logx.WithVersion(version.Get().String()),
	}
	if c.Stdout {
		options = append(options, logx.WithOutput(stdout))
	}
	if c.File.Path != "" {
		options = append(options, logx.WithFile(logx.FileOptions{
			Path:       c.File.Path,
			MaxSizeMB:  c.File.MaxSizeMB,
			MaxBackups: c.File.MaxBackups,
			MaxAgeDays: c.File.MaxAgeDays,
			Compress:   c.File.Compress,
			LocalTime:  c.File.LocalTime,
		}))
	}
	return logx.New(options...)
}

// ApplyDynamic 立即应用日志级别，并提示其他配置需要重启生效
func (c Logging) ApplyDynamic(next Logging, logger *logx.Logger) {
	if logger.Level() != next.Level {
		if err := logger.SetLevel(next.Level); err != nil {
			logger.Error("update log level failed, keeping current level", "err", err)
		} else {
			logger.Info("log level updated", "level", next.Level)
		}
	}

	current := c
	current.Level = next.Level
	if current != next {
		logger.Warn("other logging settings changed and require a service restart")
	}
}
