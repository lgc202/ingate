package apiserver

import (
	"fmt"

	"k8s.io/client-go/tools/clientcmd"

	clientset "github.com/lgc202/ingate/pkg/generated/clientset/versioned"
)

// NewClient 创建声明式资源 API 客户端
func NewClient(master, kubeconfig string) (clientset.Interface, error) {
	restConfig, err := clientcmd.BuildConfigFromFlags(master, kubeconfig)
	if err != nil {
		return nil, fmt.Errorf("build API Server client config: %w", err)
	}
	client, err := clientset.NewForConfig(restConfig)
	if err != nil {
		return nil, fmt.Errorf("create API Server resource client: %w", err)
	}
	return client, nil
}
