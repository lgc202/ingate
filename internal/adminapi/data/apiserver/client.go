// Package apiserver 通过 Ingate API Server 读写声明式资源。
package apiserver

import (
	"fmt"

	"github.com/lgc202/ingate/internal/adminapi/conf"
	"github.com/lgc202/ingate/internal/pkg/apiserverclient"
	clientset "github.com/lgc202/ingate/internal/pkg/generated/clientset/versioned"
	"github.com/lgc202/ingate/internal/pkg/version"
)

// NewClient 创建 Admin API 使用的声明式资源客户端。
func NewClient(config *conf.Data) (clientset.Interface, error) {
	apiServer := config.GetApiserver()
	restConfig, err := apiserverclient.NewConfig(apiserverclient.Options{
		MasterURL:      apiServer.GetMaster(),
		KubeconfigPath: apiServer.GetKubeconfig(),
		BearerToken:    apiServer.GetBearerToken(),
		UserAgent:      "ingate-admin-api/" + version.String(),
	})
	if err != nil {
		return nil, err
	}
	client, err := clientset.NewForConfig(restConfig)
	if err != nil {
		return nil, fmt.Errorf("create API Server resource client: %w", err)
	}
	return client, nil
}
