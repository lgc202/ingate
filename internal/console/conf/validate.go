// Package conf 定义并校验 ingate-console 进程配置。
package conf

import (
	"errors"
	"fmt"
	"net/url"
	"strings"

	"github.com/lgc202/ingate/internal/pkg/adminidentity"
	"github.com/lgc202/ingate/internal/pkg/appconfig"
)

// Validate 校验 Console 启动所需的配置。
func (c *Bootstrap) Validate() error {
	if c.GetServer() == nil || c.GetServer().GetHttp() == nil {
		return errors.New("server HTTP config is required")
	}
	if strings.TrimSpace(c.GetServer().GetHttp().GetAddr()) == "" {
		return errors.New("server HTTP address must not be empty")
	}
	if strings.TrimSpace(c.GetServer().GetConsoleDir()) == "" {
		return errors.New("server console directory must not be empty")
	}
	if c.GetServer().GetShutdownTimeout() == nil || c.GetServer().GetShutdownTimeout().AsDuration() <= 0 {
		return errors.New("server shutdown timeout must be greater than zero")
	}
	if err := validateAuthentication(c.GetServer().GetAuthentication()); err != nil {
		return err
	}
	if c.GetData() == nil || c.GetData().GetAdminApi() == nil || c.GetData().GetAssistant() == nil {
		return errors.New("admin API and assistant config are required")
	}
	if err := validateServiceURL("admin API", c.GetData().GetAdminApi().GetBaseUrl()); err != nil {
		return err
	}
	if err := validateServiceURL("assistant", c.GetData().GetAssistant().GetBaseUrl()); err != nil {
		return err
	}
	logging := c.GetLogging()
	if logging == nil {
		return errors.New("logging config is required")
	}
	return appconfig.ValidateLogging(logging)
}

func validateAuthentication(config *Server_Authentication) error {
	if config == nil {
		return errors.New("console authentication config is required")
	}
	if config.GetSessionTtl() == nil || config.GetSessionTtl().AsDuration() <= 0 {
		return errors.New("console authentication session TTL must be greater than zero")
	}
	if !adminidentity.IsValid(config.GetUsername()) {
		return errors.New("console authentication username is invalid")
	}
	if !config.GetEnabled() {
		return nil
	}
	if config.GetPassword() == "" {
		return errors.New("console authentication password is required")
	}
	if len(config.GetSessionSecret()) < 32 {
		return errors.New("console authentication session secret must contain at least 32 bytes")
	}
	return nil
}

func validateServiceURL(service, value string) error {
	target, err := url.Parse(strings.TrimSpace(value))
	if err != nil {
		return fmt.Errorf("parse %s base URL: %w", service, err)
	}
	if target.Host == "" || target.Scheme != "http" && target.Scheme != "https" {
		return fmt.Errorf("%s base URL must be an absolute HTTP URL", service)
	}
	if target.User != nil {
		return fmt.Errorf("%s base URL must not contain user information", service)
	}
	if target.RawQuery != "" || target.Fragment != "" {
		return fmt.Errorf("%s base URL must not contain a query or fragment", service)
	}
	return nil
}
