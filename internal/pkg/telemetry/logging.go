package telemetry

import (
	"errors"
	"log/slog"
	"os"
	"strings"

	kratosotel "github.com/go-kratos/kratos/contrib/otel/v3/tracing"
	kratoslog "github.com/go-kratos/kratos/v3/log"
)

// LoggerConfig 是各组件日志配置向公共装配入口暴露的最小能力。
type LoggerConfig interface {
	GetFormat() string
	GetLevel() string
	GetAddSource() bool
}

// ValidateLogging 校验所有进程共享的日志格式和级别。
func ValidateLogging(config LoggerConfig) error {
	switch strings.ToLower(config.GetFormat()) {
	case "json", "text":
	default:
		return errors.New("logging format must be json or text")
	}
	switch strings.ToLower(config.GetLevel()) {
	case "debug", "info", "warn", "error":
	default:
		return errors.New("logging level must be debug, info, warn or error")
	}
	return nil
}

// NewLogger 创建带有统一进程身份和 Trace 上下文的 Kratos slog logger。
func NewLogger(config LoggerConfig, identity Identity) *slog.Logger {
	format := kratoslog.FormatText
	if strings.EqualFold(config.GetFormat(), "json") {
		format = kratoslog.FormatJSON
	}
	logger := slog.New(kratoslog.NewHandler(
		kratoslog.WithWriter(os.Stderr),
		kratoslog.WithFormat(format),
		kratoslog.WithLevel(kratoslog.ParseLevel(config.GetLevel())),
		kratoslog.WithAddSource(config.GetAddSource()),
		kratoslog.WithExtractor(kratosotel.TraceAttrs),
	))
	return logger.With(
		"service.namespace", identity.Namespace,
		"service.name", identity.Name,
		"service.instance.id", identity.InstanceID,
		"service.version", identity.Version,
		"deployment.environment.name", identity.Environment,
		"host.name", identity.Hostname,
	)
}
