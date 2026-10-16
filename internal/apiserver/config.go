package apiserver

import (
	"errors"
	"fmt"
	"net"

	"k8s.io/apimachinery/pkg/runtime"
	genericapiserver "k8s.io/apiserver/pkg/server"
)

const componentName = "ingate-apiserver"

// Config 保存尚可由启动选项修改的 Generic API Server 配置。
type Config struct {
	GenericConfig *genericapiserver.RecommendedConfig
	Scheme        *runtime.Scheme

	// 保存本地 Informer 工厂，将退出清理责任随配置移交给 Server.New；Admission 关闭时为 nil。
	namespaceInitializer *namespaceLifecycleInitializer
}

type completedConfig struct {
	genericConfig        genericapiserver.CompletedConfig
	scheme               *runtime.Scheme
	namespaceInitializer *namespaceLifecycleInitializer
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
		genericConfig:        c.GenericConfig.Complete(),
		scheme:               c.Scheme,
		namespaceInitializer: c.namespaceInitializer,
	}

	return CompletedConfig{completedConfig: completed}
}

// closeSecureServing 回收配置阶段已取得的 Listener，或容忍原生 HTTP Server 已经关闭它。
// Secure Serving 可由上游选项禁用；本函数不创建监听器，也不管理运行循环。
func closeSecureServing(serving *genericapiserver.SecureServingInfo) error {
	if serving == nil || serving.Listener == nil {
		return nil
	}

	if err := serving.Listener.Close(); err != nil && !errors.Is(err, net.ErrClosed) {
		return fmt.Errorf("close HTTPS listener: %w", err)
	}

	return nil
}
