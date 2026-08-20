// Package apiserver 通过声明式 API 为 Controller 提供资源事实和状态持久化
package apiserver

import (
	"cmp"
	"context"
	"fmt"
	"slices"
	"time"

	"k8s.io/apimachinery/pkg/labels"

	"github.com/lgc202/ingate/internal/controller/biz/compiler"
	gatewayv1 "github.com/lgc202/ingate/pkg/apis/gateway/v1"
	clientset "github.com/lgc202/ingate/pkg/generated/clientset/versioned"
	informers "github.com/lgc202/ingate/pkg/generated/informers/externalversions"
	gatewaylisters "github.com/lgc202/ingate/pkg/generated/listers/gateway/v1"
)

// ResourceWatcher 管理声明式资源 informer 及其只读本地缓存
type ResourceWatcher struct {
	factory informers.SharedInformerFactory
	changes chan struct{}

	gateways              gatewaylisters.GatewayLister
	certificates          gatewaylisters.CertificateLister
	routes                gatewaylisters.RouteLister
	upstreams             gatewaylisters.UpstreamLister
	rateLimitPolicies     gatewaylisters.RateLimitPolicyLister
	ipRestrictionPolicies gatewaylisters.IPRestrictionPolicyLister
}

// NewResourceWatcher 创建全配置域资源监听器
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

	resources := &ResourceWatcher{
		factory:               factory,
		changes:               make(chan struct{}, 1),
		gateways:              gatewayInformer.Lister(),
		certificates:          certificateInformer.Lister(),
		routes:                routeInformer.Lister(),
		upstreams:             upstreamInformer.Lister(),
		rateLimitPolicies:     rateLimitPolicyInformer.Lister(),
		ipRestrictionPolicies: ipRestrictionPolicyInformer.Lister(),
	}
	if err := resources.registerEventHandlers([]eventRegistration{
		{name: "Gateway", informer: gatewayInformer.Informer()},
		{name: "Certificate", informer: certificateInformer.Informer()},
		{name: "Route", informer: routeInformer.Informer()},
		{name: "Upstream", informer: upstreamInformer.Informer()},
		{name: "RateLimitPolicy", informer: rateLimitPolicyInformer.Informer()},
		{name: "IPRestrictionPolicy", informer: ipRestrictionPolicyInformer.Informer()},
	}); err != nil {
		return nil, err
	}
	return resources, nil
}

// Start 启动 informer 并等待首次缓存同步
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

// Stop 等待 informer 内部 goroutine 退出
func (w *ResourceWatcher) Stop() {
	w.factory.Shutdown()
}

// Changes 返回期望配置变化通知，重复事件会在消费前合并
func (w *ResourceWatcher) Changes() <-chan struct{} {
	return w.changes
}

// List 返回深拷贝且顺序稳定的资源集合，避免编译过程修改 informer 共享对象
func (w *ResourceWatcher) List() (compiler.Resources, error) {
	gateways, err := w.gateways.List(labels.Everything())
	if err != nil {
		return compiler.Resources{}, fmt.Errorf("list Gateways: %w", err)
	}
	certificates, err := w.certificates.List(labels.Everything())
	if err != nil {
		return compiler.Resources{}, fmt.Errorf("list Certificates: %w", err)
	}
	routes, err := w.routes.List(labels.Everything())
	if err != nil {
		return compiler.Resources{}, fmt.Errorf("list Routes: %w", err)
	}
	upstreams, err := w.upstreams.List(labels.Everything())
	if err != nil {
		return compiler.Resources{}, fmt.Errorf("list Upstreams: %w", err)
	}
	rateLimitPolicies, err := w.rateLimitPolicies.List(labels.Everything())
	if err != nil {
		return compiler.Resources{}, fmt.Errorf("list RateLimitPolicies: %w", err)
	}
	ipRestrictionPolicies, err := w.ipRestrictionPolicies.List(labels.Everything())
	if err != nil {
		return compiler.Resources{}, fmt.Errorf("list IPRestrictionPolicies: %w", err)
	}
	resources := compiler.Resources{
		Gateways:              make([]*gatewayv1.Gateway, 0, len(gateways)),
		Certificates:          make([]*gatewayv1.Certificate, 0, len(certificates)),
		Routes:                make([]*gatewayv1.Route, 0, len(routes)),
		Upstreams:             make([]*gatewayv1.Upstream, 0, len(upstreams)),
		RateLimitPolicies:     make([]*gatewayv1.RateLimitPolicy, 0, len(rateLimitPolicies)),
		IPRestrictionPolicies: make([]*gatewayv1.IPRestrictionPolicy, 0, len(ipRestrictionPolicies)),
	}
	for _, resource := range gateways {
		resources.Gateways = append(resources.Gateways, resource.DeepCopy())
	}
	for _, resource := range certificates {
		resources.Certificates = append(resources.Certificates, resource.DeepCopy())
	}
	for _, resource := range routes {
		resources.Routes = append(resources.Routes, resource.DeepCopy())
	}
	for _, resource := range upstreams {
		resources.Upstreams = append(resources.Upstreams, resource.DeepCopy())
	}
	for _, resource := range rateLimitPolicies {
		resources.RateLimitPolicies = append(resources.RateLimitPolicies, resource.DeepCopy())
	}
	for _, resource := range ipRestrictionPolicies {
		resources.IPRestrictionPolicies = append(resources.IPRestrictionPolicies, resource.DeepCopy())
	}

	slices.SortFunc(resources.Gateways, func(a, b *gatewayv1.Gateway) int {
		return cmp.Compare(a.Name, b.Name)
	})
	slices.SortFunc(resources.Certificates, func(a, b *gatewayv1.Certificate) int {
		return cmp.Compare(a.Name, b.Name)
	})
	slices.SortFunc(resources.Routes, func(a, b *gatewayv1.Route) int {
		return cmp.Compare(a.Name, b.Name)
	})
	slices.SortFunc(resources.Upstreams, func(a, b *gatewayv1.Upstream) int {
		return cmp.Compare(a.Name, b.Name)
	})
	slices.SortFunc(resources.RateLimitPolicies, func(a, b *gatewayv1.RateLimitPolicy) int {
		return cmp.Compare(a.Name, b.Name)
	})
	slices.SortFunc(resources.IPRestrictionPolicies, func(a, b *gatewayv1.IPRestrictionPolicy) int {
		return cmp.Compare(a.Name, b.Name)
	})
	return resources, nil
}
