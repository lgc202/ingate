package xds

import (
	"fmt"
	"log/slog"

	"github.com/envoyproxy/go-control-plane/pkg/log"
)

var _ log.Logger = (*slogLogger)(nil)

type slogLogger struct {
	logger *slog.Logger
}

// NewSlogLogger 将 go-control-plane xDS 日志接入项目的结构化日志入口。
func NewSlogLogger(logger *slog.Logger) log.Logger {
	return &slogLogger{logger: logger}
}

func (l *slogLogger) Debugf(format string, args ...any) {
	l.logger.Debug("go-control-plane", "message", fmt.Sprintf(format, args...))
}

func (l *slogLogger) Infof(format string, args ...any) {
	// go-control-plane 的 INFO 主要是 xDS 流建立和资源推送明细，默认输出会随 Envoy 实例数增长
	l.logger.Debug("go-control-plane", "message", fmt.Sprintf(format, args...))
}

func (l *slogLogger) Warnf(format string, args ...any) {
	l.logger.Warn("go-control-plane", "message", fmt.Sprintf(format, args...))
}

func (l *slogLogger) Errorf(format string, args ...any) {
	l.logger.Error("go-control-plane", "err", fmt.Errorf(format, args...))
}
