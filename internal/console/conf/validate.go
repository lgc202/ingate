// Package conf 定义并校验 ingate-console 进程配置
package conf

import (
	"errors"
	"fmt"
	"net/url"
	"strings"
)

// Validate 校验 Console 启动所需的配置
func (c *Bootstrap) Validate() error {
	if c.GetServer() == nil || c.GetServer().GetHttp() == nil {
		return errors.New("server HTTP config is required")
	}
	if strings.TrimSpace(c.GetServer().GetHttp().GetAddr()) == "" {
		return errors.New("server HTTP address must not be empty")
	}
	if c.GetServer().GetHttp().GetTimeout() == nil || c.GetServer().GetHttp().GetTimeout().AsDuration() <= 0 {
		return errors.New("server HTTP timeout must be greater than zero")
	}
	if strings.TrimSpace(c.GetServer().GetConsoleDir()) == "" {
		return errors.New("server console directory must not be empty")
	}
	if c.GetServer().GetShutdownTimeout() == nil || c.GetServer().GetShutdownTimeout().AsDuration() <= 0 {
		return errors.New("server shutdown timeout must be greater than zero")
	}
	if c.GetData() == nil || c.GetData().GetAdminApi() == nil {
		return errors.New("admin API config is required")
	}
	if err := validateAdminAPIURL(c.GetData().GetAdminApi().GetBaseUrl()); err != nil {
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

func validateAdminAPIURL(value string) error {
	target, err := url.Parse(strings.TrimSpace(value))
	if err != nil {
		return fmt.Errorf("parse admin API base URL: %w", err)
	}
	if target.Host == "" || target.Scheme != "http" && target.Scheme != "https" {
		return errors.New("admin API base URL must be an absolute HTTP URL")
	}
	return nil
}
