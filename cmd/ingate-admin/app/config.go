package app

import (
	kitconfig "github.com/lgc202/go-kit/config"
	"github.com/lgc202/go-kit/logx"

	"github.com/lgc202/ingate/internal/admin"
)

const configEnvPrefix = "INGATE_ADMIN"

func loadConfig(path string) (*kitconfig.Config[admin.Config], admin.Config, error) {
	loaded, err := kitconfig.Load[admin.Config](path, kitconfig.WithEnv[admin.Config](configEnvPrefix))
	if err != nil {
		return nil, admin.Config{}, err
	}

	config := loaded.Get()
	if err := config.Validate(); err != nil {
		return nil, admin.Config{}, err
	}
	return loaded, config, nil
}

// watchConfig 只热更新日志级别，其他进程配置变化在重启后生效
func watchConfig(loaded *kitconfig.Config[admin.Config], logger *logx.Logger) {
	loaded.OnChange(func(old, next admin.Config) {
		if err := next.Validate(); err != nil {
			logger.Error("ignoring invalid configuration change", "err", err)
			return
		}
		old.Logging.ApplyDynamic(next.Logging, logger)
		old.Logging = next.Logging
		if kitconfig.Changed(old, next) {
			logger.Warn("service configuration changed and requires a restart")
		}
	})
}
