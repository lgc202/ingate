package apiserver

import (
	"fmt"

	"k8s.io/apiserver/pkg/admission"
	"k8s.io/apiserver/pkg/admission/plugin/namespace/lifecycle"
	genericapiserver "k8s.io/apiserver/pkg/server"
	"k8s.io/apiserver/pkg/server/healthz"
	"k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes"
)

const namespaceInformerHook = "start-ingate-namespace-informer"

// namespaceLifecycleInitializer 只替换 NamespaceLifecycle 的数据源，保留其它插件的宿主依赖。
// 官方插件在写入前检查目标空间：不存在时拒绝创建，Terminating 时阻止新增，
// 但保留已有对象的更新和删除通道。它不负责清理空间内容，也不替代认证和鉴权。
type namespaceLifecycleInitializer struct {
	// client 供插件在缓存未命中或可能过时时，实时查询 Ingate 的 Namespace API。
	client kubernetes.Interface
	// informers 通过 List/Watch 维护本地 Namespace 缓存，供插件的 Lister 查询。
	informers informers.SharedInformerFactory
}

// newNamespaceLifecycleInitializer 准备本地依赖和启动 Hook，此时不发送请求或启动 Informer。
// 成功后，工厂随 Config 交给 Server.New 注册销毁；装配失败则由装配方关闭。
func newNamespaceLifecycleInitializer(
	config *genericapiserver.RecommendedConfig,
) (*namespaceLifecycleInitializer, error) {
	if config.LoopbackClientConfig == nil {
		return nil, fmt.Errorf("NamespaceLifecycle requires a loopback client config")
	}

	// Loopback 指向当前 Ingate API Server，不能使用指向宿主 Kubernetes 的 ClientConfig；
	// 两者管理不同的 Namespace，同名也不代表同一个空间。
	client, err := kubernetes.NewForConfig(config.LoopbackClientConfig)
	if err != nil {
		return nil, fmt.Errorf("create NamespaceLifecycle loopback client: %w", err)
	}
	// 0 关闭周期性 resync，不关闭 List/Watch。缓存仍按官方 Informer 机制持续更新。
	factory := informers.NewSharedInformerFactory(client, 0)
	// 服务启动后才能通过 Loopback 请求自身；退出时由框架取消此 Context，停止 List/Watch。
	if err := config.AddPostStartHook(namespaceInformerHook, func(ctx genericapiserver.PostStartHookContext) error {
		factory.StartWithContext(ctx.Context)
		return nil
	}); err != nil {
		return nil, fmt.Errorf("register NamespaceLifecycle informer start: %w", err)
	}

	// 将已启动 Informer 的首次同步纳入 readyz，避免缓存尚未就绪就对外报告可用。
	// 独立名称用于区分本地工厂与框架已有的宿主工厂检查。
	syncCheck := healthz.NewInformerSyncHealthz(factory)
	config.AddReadyzChecks(healthz.NamedCheck("namespace-lifecycle-informer-sync", syncCheck.Check))

	return &namespaceLifecycleInitializer{client: client, informers: factory}, nil
}

// Initialize 在框架通用初始化之后，给原生 NamespaceLifecycle 注入 Ingate Client 和 Lister。
// 只匹配这个具体插件，避免把 Webhook/Policy 等插件的宿主客户端也切换到 Ingate。
func (i *namespaceLifecycleInitializer) Initialize(plugin admission.Interface) {
	lifecyclePlugin, ok := plugin.(*lifecycle.Lifecycle)
	if !ok {
		return
	}

	// 官方接口虽命名为 ExternalKube，实际接受注入的客户端和工厂，并不限定为宿主集群。
	// 两者必须来自同一个 API Server，否则缓存判断与实时查询可能互相矛盾。
	lifecyclePlugin.SetExternalKubeClientSet(i.client)
	lifecyclePlugin.SetExternalKubeInformerFactory(i.informers)
}
