package apiserver

import (
	"fmt"

	"github.com/lgc202/ingate/internal/pkg/apiserverclient"
	clientset "github.com/lgc202/ingate/internal/pkg/generated/clientset/versioned"
	"github.com/lgc202/ingate/internal/pkg/version"
)

// NewClient 创建声明式资源 API 客户端。
func NewClient(options apiserverclient.Options) (clientset.Interface, error) {
	options.UserAgent = "ingate-controller/" + version.String()
	restConfig, err := apiserverclient.NewConfig(options)
	if err != nil {
		return nil, err
	}
	client, err := clientset.NewForConfig(restConfig)
	if err != nil {
		return nil, fmt.Errorf("create API Server resource client: %w", err)
	}
	return client, nil
}
