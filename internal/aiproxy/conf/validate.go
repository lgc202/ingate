// Package conf 定义 ingate-ai-proxy 的进程配置
package conf

import (
	"errors"
	"fmt"
	"strings"

	"github.com/lgc202/ingate/pkg/redisx"
)

// Validate 校验 AI Proxy 的进程配置
func (c *Bootstrap) Validate() error {
	if c.GetServer() == nil || c.GetServer().GetHttp() == nil || c.GetServer().GetGrpc() == nil {
		return errors.New("server config is incomplete")
	}
	http := c.GetServer().GetHttp()
	if strings.TrimSpace(http.GetNetwork()) == "" || strings.TrimSpace(http.GetAddr()) == "" {
		return errors.New("server HTTP network and address must not be empty")
	}
	if http.GetTimeout() == nil || http.GetTimeout().AsDuration() <= 0 {
		return errors.New("server HTTP timeout must be greater than zero")
	}
	grpc := c.GetServer().GetGrpc()
	if strings.TrimSpace(grpc.GetNetwork()) == "" || strings.TrimSpace(grpc.GetAddr()) == "" {
		return errors.New("server gRPC network and address must not be empty")
	}
	if c.GetServer().GetShutdownTimeout() == nil || c.GetServer().GetShutdownTimeout().AsDuration() <= 0 {
		return errors.New("server shutdown timeout must be greater than zero")
	}
	if c.GetData() == nil || c.GetData().GetRedis() == nil {
		return errors.New("data Redis config is required")
	}
	redis := c.GetData().GetRedis()
	if err := (redisx.Config{
		Address:  redis.GetAddress(),
		Password: redis.GetPassword(),
		Database: int(redis.GetDatabase()),
	}).Validate(); err != nil {
		return fmt.Errorf("validate Redis config: %w", err)
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
