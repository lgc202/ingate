// Package appconfig 统一加载和校验各组件的 Kratos 配置
package appconfig

import (
	"errors"
	"fmt"

	"github.com/go-kratos/kratos/v3/config"
	"github.com/go-kratos/kratos/v3/config/file"
)

// Config 是可以在加载后自行校验的进程配置
type Config interface {
	Validate() error
}

// Load 从 YAML 文件读取、解析并校验完整进程配置
func Load(configFile string, target Config) (err error) {
	loaded := config.New(config.WithSource(file.NewSource(configFile)))
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
