// Package appconfig 统一加载配置并装配各 Kratos 组件共享的进程能力。
package appconfig

import (
	"errors"
	"fmt"

	"github.com/go-kratos/kratos/v3/config"
	"github.com/go-kratos/kratos/v3/config/env"
	"github.com/go-kratos/kratos/v3/config/file"
)

// Config 是可以在加载后自行校验的进程配置。
type Config interface {
	Validate() error
}

// Load 从 YAML 文件读取、解析并校验完整进程配置。
func Load(configFile string, target Config) (err error) {
	// YAML 保存非敏感配置，INGATE_ 前缀环境变量只用于解析文件中的显式占位符。
	// 这样密码和会话密钥不需要进入仓库，也不会让全部普通配置变成环境变量。
	loaded := config.New(config.WithSource(
		file.NewSource(configFile),
		env.NewSource("INGATE"),
	))
	defer func() {
		if closeErr := loaded.Close(); closeErr != nil {
			err = errors.Join(err, fmt.Errorf("close configuration %q: %w", configFile, closeErr))
		}
	}()

	if err := loaded.Load(); err != nil {
		return fmt.Errorf("load configuration %q: %w", configFile, err)
	}
	if err := loaded.Scan(target); err != nil {
		return fmt.Errorf("scan configuration %q: %w", configFile, err)
	}
	if err := target.Validate(); err != nil {
		return fmt.Errorf("validate configuration %q: %w", configFile, err)
	}
	return nil
}
