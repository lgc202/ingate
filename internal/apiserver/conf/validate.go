// Package conf 定义并校验 ingate-apiserver 进程配置
package conf

import (
	"errors"
	"net"
	"strconv"
	"strings"
)

// Validate 校验 API Server 进程启动所需的配置
func (c *Bootstrap) Validate() error {
	if c.GetServer() == nil || c.GetServer().GetHttp() == nil {
		return errors.New("server HTTP config is required")
	}
	host, portText, err := net.SplitHostPort(c.GetServer().GetHttp().GetAddr())
	if err != nil || net.ParseIP(host) == nil {
		return errors.New("server HTTP address must contain a valid IP and port")
	}
	port, err := strconv.Atoi(portText)
	if err != nil || port < 1 || port > 65535 {
		return errors.New("server HTTP port must be between 1 and 65535")
	}
	if strings.TrimSpace(c.GetServer().GetHttp().GetCertDirectory()) == "" {
		return errors.New("server certificate directory must not be empty")
	}
	if c.GetServer().GetShutdownTimeout() == nil || c.GetServer().GetShutdownTimeout().AsDuration() <= 0 {
		return errors.New("server shutdown timeout must be greater than zero")
	}

	if c.GetData() == nil || c.GetData().GetEtcd() == nil || len(c.GetData().GetEtcd().GetEndpoints()) == 0 {
		return errors.New("at least one etcd endpoint is required")
	}
	for _, endpoint := range c.GetData().GetEtcd().GetEndpoints() {
		if strings.TrimSpace(endpoint) == "" {
			return errors.New("etcd endpoint must not be empty")
		}
	}
	if !strings.HasPrefix(c.GetData().GetEtcd().GetPrefix(), "/") {
		return errors.New("etcd prefix must start with /")
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
