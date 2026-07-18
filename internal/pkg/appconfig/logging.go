// Package appconfig 提供各服务进程共享的基础配置模型
package appconfig

import (
	"errors"
	"fmt"
	"io"

	"github.com/lgc202/go-kit/logx"
	"github.com/lgc202/go-kit/version"
)

// Logging 定义服务日志配置
type Logging struct {
	Format    logx.Format `mapstructure:"format"`
	Level     logx.Level  `mapstructure:"level"`
	AddSource bool        `mapstructure:"add_source"`
	Stdout    bool        `mapstructure:"stdout"`
	File      LogFile     `mapstructure:"file"`
}

// LogFile 定义文件日志和轮转配置
type LogFile struct {
	Path       string `mapstructure:"path"`
	MaxSizeMB  int    `mapstructure:"max_size_mb"`
	MaxBackups int    `mapstructure:"max_backups"`
	MaxAgeDays int    `mapstructure:"max_age_days"`
	Compress   bool   `mapstructure:"compress"`
	LocalTime  bool   `mapstructure:"local_time"`
}

// Validate 校验日志配置
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

// NewLogger 根据公共配置创建服务 logger
func (c Logging) NewLogger(service string, stdout io.Writer) (*logx.Logger, error) {
	options := []logx.Option{
		logx.WithFormat(c.Format),
		logx.WithLevel(c.Level),
		logx.WithAddSource(c.AddSource),
		logx.WithService(service),
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

// ApplyDynamic 应用无需重启即可生效的日志配置
func (c Logging) ApplyDynamic(next Logging, logger *logx.Logger) {
	if logger.Level() != next.Level {
		if err := logger.SetLevel(next.Level); err != nil {
			logger.Error("更新日志级别失败，继续使用原级别", "err", err)
		} else {
			logger.Info("日志级别已更新", "level", next.Level)
		}
	}

	c.Level = next.Level
	if c != next {
		logger.Warn("其他日志配置已变化，将在服务重启后生效")
	}
}
