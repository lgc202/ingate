// Package conf 定义并校验 ingate-ai-extproc 进程配置
package conf

import (
	"errors"
	"strings"
)

// Validate 校验 AI ExtProc 进程启动所需的配置
func (c *Bootstrap) Validate() error {
	if c.GetServer() == nil || c.GetServer().GetGrpc() == nil || c.GetServer().GetHttp() == nil {
		return errors.New("server gRPC and HTTP config are required")
	}
	if strings.TrimSpace(c.GetServer().GetGrpc().GetAddr()) == "" {
		return errors.New("server gRPC address must not be empty")
	}
	if strings.TrimSpace(c.GetServer().GetHttp().GetAddr()) == "" {
		return errors.New("server HTTP address must not be empty")
	}
	if c.GetServer().GetHttp().GetTimeout() == nil || c.GetServer().GetHttp().GetTimeout().AsDuration() <= 0 {
		return errors.New("server HTTP timeout must be greater than zero")
	}
	if c.GetServer().GetShutdownTimeout() == nil || c.GetServer().GetShutdownTimeout().AsDuration() <= 0 {
		return errors.New("server shutdown timeout must be greater than zero")
	}
	if c.GetLogging() == nil {
		return errors.New("logging config is required")
	}
	switch strings.ToLower(c.GetLogging().GetFormat()) {
	case "json", "text":
	default:
		return errors.New("logging format must be json or text")
	}
	switch strings.ToLower(c.GetLogging().GetLevel()) {
	case "debug", "info", "warn", "error":
	default:
		return errors.New("logging level must be debug, info, warn or error")
	}
	return nil
}
