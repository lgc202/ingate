// Package controllermanager 组装 Namespace 清理、共享数据面、资源查询投影与 xDS 服务。
package controllermanager

import (
	"fmt"
	"maps"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/apimachinery/pkg/util/validation"
	"k8s.io/client-go/discovery"
	corev1client "k8s.io/client-go/kubernetes/typed/core/v1"
	"k8s.io/client-go/metadata"
	"k8s.io/client-go/rest"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/cache"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/healthz"

	mysqlstore "github.com/lgc202/ingate/internal/adminapi/repository/mysql"
	gatewaycontroller "github.com/lgc202/ingate/internal/controllermanager/controller/gateway"
	namespacecontroller "github.com/lgc202/ingate/internal/controllermanager/controller/namespace"
	"github.com/lgc202/ingate/internal/controllermanager/controller/namespace/deletion"
	"github.com/lgc202/ingate/internal/controllermanager/controller/projection"
	"github.com/lgc202/ingate/internal/controllermanager/dataplane/envoy"
	"github.com/lgc202/ingate/internal/pkg/aidataplane"
)

// Config 保存 Controller Manager 的 API 连接和运行选项。
type Config struct {
	RESTConfig        *rest.Config
	XDSBindAddress    string
	AIConfigNamespace string
	ManagerOptions    ctrl.Options
}

// New 创建并配置 Manager，但不启动 Controller 或网络监听。
func (c *Config) New(store *mysqlstore.Store) (ctrl.Manager, error) {
	if messages := validation.IsDNS1123Label(c.AIConfigNamespace); len(messages) != 0 {
		return nil, fmt.Errorf("AI configuration namespace %q must be a valid DNS label", c.AIConfigNamespace)
	}

	manager, err := ctrl.NewManager(c.RESTConfig, c.managerOptions())
	if err != nil {
		return nil, fmt.Errorf("create controller manager: %w", err)
	}

	if err := manager.AddHealthzCheck("healthz", healthz.Ping); err != nil {
		return nil, fmt.Errorf("add health check: %w", err)
	}

	publisher := envoy.NewPublisher()
	xdsServer := envoy.NewServer(c.XDSBindAddress, publisher, manager.GetLogger().WithName("xds"))
	if err := manager.Add(xdsServer); err != nil {
		return nil, fmt.Errorf("add xDS server: %w", err)
	}

	reconciler := gatewaycontroller.New(manager.GetClient(), manager.GetAPIReader(), publisher, c.AIConfigNamespace)
	if err := reconciler.SetupWithManager(manager); err != nil {
		return nil, fmt.Errorf("setup Gateway controller: %w", err)
	}
	if err := setupNamespaceController(manager); err != nil {
		return nil, fmt.Errorf("setup Namespace controller: %w", err)
	}

	if err := projection.NewGateway(manager.GetClient(), store).SetupWithManager(manager); err != nil {
		return nil, fmt.Errorf("setup Gateway projection controller: %w", err)
	}
	if err := projection.NewHTTPRoute(manager.GetClient(), store).SetupWithManager(manager); err != nil {
		return nil, fmt.Errorf("setup HTTPRoute projection controller: %w", err)
	}
	if err := projection.NewBackend(manager.GetClient(), store).SetupWithManager(manager); err != nil {
		return nil, fmt.Errorf("setup Backend projection controller: %w", err)
	}
	if err := projection.NewAIBackend(manager.GetClient(), store).SetupWithManager(manager); err != nil {
		return nil, fmt.Errorf("setup AIBackend projection controller: %w", err)
	}
	if err := projection.NewModelRoute(manager.GetClient(), store).SetupWithManager(manager); err != nil {
		return nil, fmt.Errorf("setup ModelRoute projection controller: %w", err)
	}
	if err := projection.NewMCPRoute(manager.GetClient(), store).SetupWithManager(manager); err != nil {
		return nil, fmt.Errorf("setup MCPRoute projection controller: %w", err)
	}

	if err := manager.AddReadyzCheck("xds", xdsServer.CheckReady); err != nil {
		return nil, fmt.Errorf("add xDS readiness check: %w", err)
	}

	return manager, nil
}

// managerOptions 只裁剪 ConfigMap 的 List/Watch，不限制领域资源或 Namespace 清理。
// 复制调用方的配置表，并收回同一 ConfigMap 类型的其他指针键，避免重复注册或放宽范围。
func (c *Config) managerOptions() ctrl.Options {
	options := c.ManagerOptions
	options.Cache.ByObject = maps.Clone(options.Cache.ByObject)
	if options.Cache.ByObject == nil {
		options.Cache.ByObject = make(map[client.Object]cache.ByObject)
	}
	for object := range options.Cache.ByObject {
		if _, isConfigMap := object.(*corev1.ConfigMap); isConfigMap {
			delete(options.Cache.ByObject, object)
		}
	}
	options.Cache.ByObject[&corev1.ConfigMap{}] = cache.ByObject{
		Namespaces: map[string]cache.Config{
			c.AIConfigNamespace: {FieldSelector: fields.OneTermEqualSelector("metadata.name", aidataplane.ConfigMapName)},
		},
	}

	return options
}

// setupNamespaceController 让发现、Namespace 写入和内容删除都使用 Manager 的 Ingate 连接配置。
// 原生客户端构造不发起请求；不使用 API Server 的宿主 Kubernetes 连接。
func setupNamespaceController(manager ctrl.Manager) error {
	config := manager.GetConfig()
	coreClient, err := corev1client.NewForConfig(config)
	if err != nil {
		return fmt.Errorf("create Namespace client: %w", err)
	}

	metadataClient, err := metadata.NewForConfig(config)
	if err != nil {
		return fmt.Errorf("create namespace metadata client: %w", err)
	}

	// 独立使用原生 Discovery 构造器，保留其请求超时默认值，不另建超时或发现框架。
	discoveryClient, err := discovery.NewDiscoveryClientForConfig(config)
	if err != nil {
		return fmt.Errorf("create namespace discovery client: %w", err)
	}

	discoverResources := discoveryClient.ServerPreferredNamespacedResourcesWithContext
	deleter := deletion.New(coreClient.Namespaces(), metadataClient, discoverResources)
	reconciler := namespacecontroller.New(manager.GetClient(), deleter)
	return reconciler.SetupWithManager(manager)
}
