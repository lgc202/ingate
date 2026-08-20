package app

import (
	"errors"
	"fmt"
	"net/url"
	"strings"

	"github.com/lgc202/ingate/internal/pkg/appconfig"
)

const defaultConfigPath = "configs/ingate-console.yaml"

// Config 定义 ingate-console 的进程配置
type Config struct {
	Server   ServerConfig      `mapstructure:"server"`
	AdminAPI AdminAPIConfig    `mapstructure:"admin_api"`
	Logging  appconfig.Logging `mapstructure:"logging"`
}

// ServerConfig 定义控制台监听与静态资源配置
type ServerConfig struct {
	ListenAddress string `mapstructure:"listen_address"`
	ConsoleDir    string `mapstructure:"console_dir"`
}

// AdminAPIConfig 定义管理 API 的连接地址
type AdminAPIConfig struct {
	BaseURL string `mapstructure:"base_url"`
}

// Validate 校验进程配置
func (c Config) Validate() error {
	if strings.TrimSpace(c.Server.ListenAddress) == "" {
		return errors.New("server listen address must not be empty")
	}
	if err := validateAdminAPIURL(c.AdminAPI.BaseURL); err != nil {
		return err
	}
	return c.Logging.Validate()
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
