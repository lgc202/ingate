package appconfig

import (
	"log/slog"
	"os"
	"strings"

	kratoslog "github.com/go-kratos/kratos/v3/log"

	"github.com/lgc202/ingate/internal/pkg/version"
)

// LoggerConfig 是各组件生成配置向公共日志装配暴露的最小能力
//
// 接口定义在消费配置的 appconfig 包中，避免公共包依赖任一组件的 conf package
type LoggerConfig interface {
	GetFormat() string
	GetLevel() string
	GetAddSource() bool
}

// NewLogger 创建带有统一服务标识字段的 Kratos slog logger
func NewLogger(config LoggerConfig, serviceName, serviceID string) *slog.Logger {
	format := kratoslog.FormatText
	if strings.EqualFold(config.GetFormat(), "json") {
		format = kratoslog.FormatJSON
	}
	handler := kratoslog.NewHandler(
		kratoslog.WithWriter(os.Stderr),
		kratoslog.WithFormat(format),
		kratoslog.WithLevel(kratoslog.ParseLevel(config.GetLevel())),
		kratoslog.WithAddSource(config.GetAddSource()),
	)
	return kratoslog.NewLogger(handler).With(
		"service.id", serviceID,
		"service.name", serviceName,
		"service.version", version.String(),
	)
}
