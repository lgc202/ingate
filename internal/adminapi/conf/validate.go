// Package conf 定义并校验 ingate-admin-api 进程配置
package conf

import (
	"errors"
	"strings"
)

// Validate 校验 Admin API 的进程配置
func (c *Bootstrap) Validate() error {
	if c.GetServer() == nil || c.GetServer().GetHttp() == nil {
		return errors.New("server http config is required")
	}
	http := c.GetServer().GetHttp()
	if strings.TrimSpace(http.GetAddr()) == "" {
		return errors.New("server http address must not be empty")
	}
	if http.GetTimeout() == nil || http.GetTimeout().AsDuration() <= 0 {
		return errors.New("server http timeout must be greater than zero")
	}
	if c.GetServer().GetShutdownTimeout() == nil || c.GetServer().GetShutdownTimeout().AsDuration() <= 0 {
		return errors.New("server shutdown timeout must be greater than zero")
	}
	if err := validateData(c.GetData()); err != nil {
		return err
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

func validateData(data *Data) error {
	if data == nil || data.GetApiserver() == nil {
		return errors.New("apiserver config is required")
	}
	analytics := data.GetAnalytics()
	if analytics == nil {
		return errors.New("analytics config is required")
	}
	if strings.TrimSpace(analytics.GetAddr()) == "" {
		return errors.New("analytics address must not be empty")
	}
	if analytics.GetTimeout() == nil || analytics.GetTimeout().AsDuration() <= 0 {
		return errors.New("analytics timeout must be greater than zero")
	}
	tls := analytics.GetTls()
	if tls.GetEnabled() && (tls.GetCertFile() == "") != (tls.GetKeyFile() == "") {
		return errors.New("analytics TLS certificate and key must be configured together")
	}
	return nil
}
