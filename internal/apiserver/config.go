package apiserver

import (
	"k8s.io/apimachinery/pkg/runtime"
	genericapiserver "k8s.io/apiserver/pkg/server"
)

const componentName = "ingate-apiserver"

// Config 保存尚可由启动选项修改的 Generic API Server 配置。
type Config struct {
	GenericConfig *genericapiserver.RecommendedConfig
	Scheme        *runtime.Scheme
}

type completedConfig struct {
	genericConfig genericapiserver.CompletedConfig
	scheme        *runtime.Scheme
}

// CompletedConfig 保存已经完成默认值计算的配置。
// 私有指针防止包外绕过 Config.Complete 构造有效值。
type CompletedConfig struct {
	*completedConfig
}

// Complete 完成 Generic API Server 配置的默认值计算。
// 它会修改 Config.GenericConfig，应在应用全部启动选项后调用。
func (c *Config) Complete() CompletedConfig {
	completed := &completedConfig{
		genericConfig: c.GenericConfig.Complete(),
		scheme:        c.Scheme,
	}

	return CompletedConfig{completedConfig: completed}
}
