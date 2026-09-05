// Package apiserver 通过声明式 API 为 Controller 提供资源事实和状态持久化。
package apiserver

import (
	"cmp"
	"context"
	"fmt"
	"slices"
	"time"

	"github.com/samber/lo"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"

	"github.com/lgc202/ingate/internal/controller/biz/compiler"
	gatewayv1 "github.com/lgc202/ingate/internal/pkg/apis/gateway/v1"
	clientset "github.com/lgc202/ingate/internal/pkg/generated/clientset/versioned"
	informers "github.com/lgc202/ingate/internal/pkg/generated/informers/externalversions"
	gatewaylisters "github.com/lgc202/ingate/internal/pkg/generated/listers/gateway/v1"
)

// ResourceWatcher 管理声明式资源 informer 及其只读本地缓存。
type ResourceWatcher struct {
	factory informers.SharedInformerFactory
	changes chan struct{}

	gateways                     gatewaylisters.GatewayLister
	certificates                 gatewaylisters.CertificateLister
	routes                       gatewaylisters.RouteLister
	upstreams                    gatewaylisters.UpstreamLister
	rateLimitPolicies            gatewaylisters.RateLimitPolicyLister
	ipRestrictionPolicies        gatewaylisters.IPRestrictionPolicyLister
	headerTransformationPolicies gatewaylisters.HeaderTransformationPolicyLister
	mockResponsePolicies         gatewaylisters.MockResponsePolicyLister
	wasmPlugins                  gatewaylisters.WasmPluginLister
}

// NewResourceWatcher 创建全配置域资源监听器。
func NewResourceWatcher(
	client clientset.Interface,
	resyncPeriod time.Duration,
) (*ResourceWatcher, error) {
	factory := informers.NewSharedInformerFactory(client, resyncPeriod)
	gatewayInformers := factory.Gateway().V1()
	gatewayInformer := gatewayInformers.Gateways()
	certificateInformer := gatewayInformers.Certificates()
	routeInformer := gatewayInformers.Routes()
	upstreamInformer := gatewayInformers.Upstreams()
	rateLimitPolicyInformer := gatewayInformers.RateLimitPolicies()
	ipRestrictionPolicyInformer := gatewayInformers.IPRestrictionPolicies()
	headerTransformationPolicyInformer := gatewayInformers.HeaderTransformationPolicies()
	mockResponsePolicyInformer := gatewayInformers.MockResponsePolicies()
	wasmPluginInformer := gatewayInformers.WasmPlugins()

	resources := &ResourceWatcher{
		factory:                      factory,
		changes:                      make(chan struct{}, 1),
		gateways:                     gatewayInformer.Lister(),
		certificates:                 certificateInformer.Lister(),
		routes:                       routeInformer.Lister(),
		upstreams:                    upstreamInformer.Lister(),
		rateLimitPolicies:            rateLimitPolicyInformer.Lister(),
		ipRestrictionPolicies:        ipRestrictionPolicyInformer.Lister(),
		headerTransformationPolicies: headerTransformationPolicyInformer.Lister(),
		mockResponsePolicies:         mockResponsePolicyInformer.Lister(),
		wasmPlugins:                  wasmPluginInformer.Lister(),
	}
	if err := resources.registerEventHandlers([]eventRegistration{
		{name: "Gateway", informer: gatewayInformer.Informer()},
		{name: "Certificate", informer: certificateInformer.Informer()},
		{name: "Route", informer: routeInformer.Informer()},
		{name: "Upstream", informer: upstreamInformer.Informer()},
		{name: "RateLimitPolicy", informer: rateLimitPolicyInformer.Informer()},
		{name: "IPRestrictionPolicy", informer: ipRestrictionPolicyInformer.Informer()},
		{name: "HeaderTransformationPolicy", informer: headerTransformationPolicyInformer.Informer()},
		{name: "MockResponsePolicy", informer: mockResponsePolicyInformer.Informer()},
		{name: "WasmPlugin", informer: wasmPluginInformer.Informer()},
	}); err != nil {
		return nil, err
	}
	return resources, nil
}

// Start 启动 informer 并等待首次缓存同步。
func (w *ResourceWatcher) Start(ctx context.Context) error {
	w.factory.Start(ctx.Done())
	for resourceType, synced := range w.factory.WaitForCacheSync(ctx.Done()) {
		if synced {
			continue
		}
		if ctx.Err() != nil {
			return nil
		}
		return fmt.Errorf("sync informer cache for %v", resourceType)
	}
	return nil
}

// Stop 等待 informer 内部 goroutine 退出。
func (w *ResourceWatcher) Stop() {
	w.factory.Shutdown()
}

// Changes 返回期望配置变化通知，重复事件会在消费前合并。
func (w *ResourceWatcher) Changes() <-chan struct{} {
	return w.changes
}

// List 返回深拷贝且顺序稳定的资源集合，避免编译过程修改 informer 共享对象。
func (w *ResourceWatcher) List() (compiler.Resources, error) {
	gateways, err := listResourceCopies("Gateways", w.gateways.List, (*gatewayv1.Gateway).DeepCopy)
	if err != nil {
		return compiler.Resources{}, err
	}
	certificates, err := listResourceCopies(
		"Certificates", w.certificates.List, (*gatewayv1.Certificate).DeepCopy,
	)
	if err != nil {
		return compiler.Resources{}, err
	}
	routes, err := listResourceCopies("Routes", w.routes.List, (*gatewayv1.Route).DeepCopy)
	if err != nil {
		return compiler.Resources{}, err
	}
	upstreams, err := listResourceCopies("Upstreams", w.upstreams.List, (*gatewayv1.Upstream).DeepCopy)
	if err != nil {
		return compiler.Resources{}, err
	}
	rateLimitPolicies, err := listResourceCopies(
		"RateLimitPolicies", w.rateLimitPolicies.List, (*gatewayv1.RateLimitPolicy).DeepCopy,
	)
	if err != nil {
		return compiler.Resources{}, err
	}
	ipRestrictionPolicies, err := listResourceCopies(
		"IPRestrictionPolicies", w.ipRestrictionPolicies.List, (*gatewayv1.IPRestrictionPolicy).DeepCopy,
	)
	if err != nil {
		return compiler.Resources{}, err
	}
	headerTransformationPolicies, err := listResourceCopies(
		"HeaderTransformationPolicies",
		w.headerTransformationPolicies.List,
		(*gatewayv1.HeaderTransformationPolicy).DeepCopy,
	)
	if err != nil {
		return compiler.Resources{}, err
	}
	mockResponsePolicies, err := listResourceCopies(
		"MockResponsePolicies", w.mockResponsePolicies.List, (*gatewayv1.MockResponsePolicy).DeepCopy,
	)
	if err != nil {
		return compiler.Resources{}, err
	}
	wasmPlugins, err := listResourceCopies("WasmPlugins", w.wasmPlugins.List, (*gatewayv1.WasmPlugin).DeepCopy)
	if err != nil {
		return compiler.Resources{}, err
	}
	return compiler.Resources{
		Gateways:                     gateways,
		Certificates:                 certificates,
		Routes:                       routes,
		Upstreams:                    upstreams,
		RateLimitPolicies:            rateLimitPolicies,
		IPRestrictionPolicies:        ipRestrictionPolicies,
		HeaderTransformationPolicies: headerTransformationPolicies,
		MockResponsePolicies:         mockResponsePolicies,
		WasmPlugins:                  wasmPlugins,
	}, nil
}

func listResourceCopies[T metav1.Object](
	resourceType string,
	list func(labels.Selector) ([]T, error),
	deepCopy func(T) T,
) ([]T, error) {
	shared, err := list(labels.Everything())
	if err != nil {
		return nil, fmt.Errorf("list %s: %w", resourceType, err)
	}

	resources := lo.Map(shared, func(resource T, _ int) T {
		return deepCopy(resource)
	})
	slices.SortFunc(resources, func(a, b T) int {
		return cmp.Compare(a.GetName(), b.GetName())
	})
	return resources, nil
}
