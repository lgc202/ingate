package apiserver

import (
	"fmt"

	clientset "github.com/lgc202/ingate/pkg/generated/clientset/versioned"
	gatewayclient "github.com/lgc202/ingate/pkg/generated/clientset/versioned/typed/gateway/v1alpha1"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
)

const userAgent = "ingate-admin-api"

// Store 持有 Admin API 访问声明式 API Server 的类型化客户端。
type Store struct {
	client clientset.Interface
}

// New 根据 kubeconfig 创建 Store；路径为空时使用集群内配置。
func New(kubeconfig string) (*Store, error) {
	config, err := loadConfig(kubeconfig)
	if err != nil {
		return nil, err
	}

	config = rest.AddUserAgent(config, userAgent)

	client, err := clientset.NewForConfig(config)
	if err != nil {
		return nil, fmt.Errorf("create API server client: %w", err)
	}

	return &Store{client: client}, nil
}

// Gateways 返回指定命名空间的 Gateway 资源客户端。
func (s *Store) Gateways(namespace string) gatewayclient.GatewayInterface {
	return s.client.GatewayV1alpha1().Gateways(namespace)
}

// HTTPRoutes 返回指定命名空间的 HTTPRoute 资源客户端。
func (s *Store) HTTPRoutes(namespace string) gatewayclient.HTTPRouteInterface {
	return s.client.GatewayV1alpha1().HTTPRoutes(namespace)
}

// Backends 返回指定命名空间的 Backend 资源客户端。
func (s *Store) Backends(namespace string) gatewayclient.BackendInterface {
	return s.client.GatewayV1alpha1().Backends(namespace)
}

// AIBackends 返回指定命名空间的 AIBackend 资源客户端。
func (s *Store) AIBackends(namespace string) gatewayclient.AIBackendInterface {
	return s.client.GatewayV1alpha1().AIBackends(namespace)
}

// ModelRoutes 返回指定命名空间的 ModelRoute 资源客户端。
func (s *Store) ModelRoutes(namespace string) gatewayclient.ModelRouteInterface {
	return s.client.GatewayV1alpha1().ModelRoutes(namespace)
}

// MCPRoutes 返回指定命名空间的 MCPRoute 资源客户端。
func (s *Store) MCPRoutes(namespace string) gatewayclient.MCPRouteInterface {
	return s.client.GatewayV1alpha1().MCPRoutes(namespace)
}

func loadConfig(kubeconfig string) (*rest.Config, error) {
	if kubeconfig == "" {
		config, err := rest.InClusterConfig()
		if err != nil {
			return nil, fmt.Errorf("load in-cluster API server config: %w", err)
		}

		return config, nil
	}

	config, err := clientcmd.BuildConfigFromFlags("", kubeconfig)
	if err != nil {
		return nil, fmt.Errorf("load API server config from %q: %w", kubeconfig, err)
	}

	return config, nil
}
