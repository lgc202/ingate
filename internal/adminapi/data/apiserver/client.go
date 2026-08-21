// Package apiserver 通过 Ingate API Server 读写声明式资源
package apiserver

import (
	"fmt"

	"k8s.io/client-go/tools/clientcmd"

	"github.com/lgc202/ingate/internal/adminapi/conf"
	clientset "github.com/lgc202/ingate/internal/pkg/generated/clientset/versioned"
)

// NewClient 创建 Admin API 使用的声明式资源客户端
func NewClient(config *conf.Data) (clientset.Interface, error) {
	restConfig, err := clientcmd.BuildConfigFromFlags(
		config.GetApiserver().GetMaster(),
		config.GetApiserver().GetKubeconfig(),
	)
	if err != nil {
		return nil, fmt.Errorf("build apiserver client config: %w", err)
	}
	client, err := clientset.NewForConfig(restConfig)
	if err != nil {
		return nil, fmt.Errorf("create apiserver client: %w", err)
	}
	return client, nil
}
