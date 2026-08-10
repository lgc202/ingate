package reconcile

import (
	"cmp"
	"context"
	"fmt"
	"slices"
	"time"

	"k8s.io/apimachinery/pkg/labels"

	"github.com/lgc202/ingate/internal/controller/compiler"
	gatewayv1 "github.com/lgc202/ingate/pkg/apis/gateway/v1"
	clientset "github.com/lgc202/ingate/pkg/generated/clientset/versioned"
	informers "github.com/lgc202/ingate/pkg/generated/informers/externalversions"
	gatewaylisters "github.com/lgc202/ingate/pkg/generated/listers/gateway/v1"
)

// resourceCache 管理声明式资源 informer 及其只读本地缓存
type resourceCache struct {
	factory         informers.SharedInformerFactory
	onDesiredChange func()

	gateways              gatewaylisters.GatewayLister
	certificates          gatewaylisters.CertificateLister
	routes                gatewaylisters.RouteLister
	upstreams             gatewaylisters.UpstreamLister
	rateLimitPolicies     gatewaylisters.RateLimitPolicyLister
	ipRestrictionPolicies gatewaylisters.IPRestrictionPolicyLister
	tokenQuotaPolicies    gatewaylisters.TokenQuotaPolicyLister
}

func newResourceCache(
	client clientset.Interface,
	resyncPeriod time.Duration,
	onDesiredChange func(),
) (*resourceCache, error) {
	factory := informers.NewSharedInformerFactory(client, resyncPeriod)
	gatewayInformers := factory.Gateway().V1()
	gatewayInformer := gatewayInformers.Gateways()
	certificateInformer := gatewayInformers.Certificates()
	routeInformer := gatewayInformers.Routes()
	upstreamInformer := gatewayInformers.Upstreams()
	rateLimitPolicyInformer := gatewayInformers.RateLimitPolicies()
	ipRestrictionPolicyInformer := gatewayInformers.IPRestrictionPolicies()
	tokenQuotaPolicyInformer := gatewayInformers.TokenQuotaPolicies()

	resources := &resourceCache{
		factory:               factory,
		onDesiredChange:       onDesiredChange,
		gateways:              gatewayInformer.Lister(),
		certificates:          certificateInformer.Lister(),
		routes:                routeInformer.Lister(),
		upstreams:             upstreamInformer.Lister(),
		rateLimitPolicies:     rateLimitPolicyInformer.Lister(),
		ipRestrictionPolicies: ipRestrictionPolicyInformer.Lister(),
		tokenQuotaPolicies:    tokenQuotaPolicyInformer.Lister(),
	}
	if err := resources.registerEventHandlers([]eventRegistration{
		{name: "Gateway", informer: gatewayInformer.Informer()},
		{name: "Certificate", informer: certificateInformer.Informer()},
		{name: "Route", informer: routeInformer.Informer()},
		{name: "Upstream", informer: upstreamInformer.Informer()},
		{name: "RateLimitPolicy", informer: rateLimitPolicyInformer.Informer()},
		{name: "IPRestrictionPolicy", informer: ipRestrictionPolicyInformer.Informer()},
		{name: "TokenQuotaPolicy", informer: tokenQuotaPolicyInformer.Informer()},
	}); err != nil {
		return nil, err
	}
	return resources, nil
}

func (c *resourceCache) start(ctx context.Context) error {
	c.factory.Start(ctx.Done())
	for resourceType, synced := range c.factory.WaitForCacheSync(ctx.Done()) {
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

func (c *resourceCache) shutdown() {
	c.factory.Shutdown()
}

// list 返回深拷贝且顺序稳定的资源集合，避免编译过程修改 informer 共享对象
func (c *resourceCache) list() (compiler.Resources, error) {
	gateways, err := c.gateways.List(labels.Everything())
	if err != nil {
		return compiler.Resources{}, fmt.Errorf("list Gateways: %w", err)
	}
	certificates, err := c.certificates.List(labels.Everything())
	if err != nil {
		return compiler.Resources{}, fmt.Errorf("list Certificates: %w", err)
	}
	routes, err := c.routes.List(labels.Everything())
	if err != nil {
		return compiler.Resources{}, fmt.Errorf("list Routes: %w", err)
	}
	upstreams, err := c.upstreams.List(labels.Everything())
	if err != nil {
		return compiler.Resources{}, fmt.Errorf("list Upstreams: %w", err)
	}
	rateLimitPolicies, err := c.rateLimitPolicies.List(labels.Everything())
	if err != nil {
		return compiler.Resources{}, fmt.Errorf("list RateLimitPolicies: %w", err)
	}
	ipRestrictionPolicies, err := c.ipRestrictionPolicies.List(labels.Everything())
	if err != nil {
		return compiler.Resources{}, fmt.Errorf("list IPRestrictionPolicies: %w", err)
	}
	tokenQuotaPolicies, err := c.tokenQuotaPolicies.List(labels.Everything())
	if err != nil {
		return compiler.Resources{}, fmt.Errorf("list TokenQuotaPolicies: %w", err)
	}
	resources := compiler.Resources{
		Gateways:              make([]*gatewayv1.Gateway, 0, len(gateways)),
		Certificates:          make([]*gatewayv1.Certificate, 0, len(certificates)),
		Routes:                make([]*gatewayv1.Route, 0, len(routes)),
		Upstreams:             make([]*gatewayv1.Upstream, 0, len(upstreams)),
		RateLimitPolicies:     make([]*gatewayv1.RateLimitPolicy, 0, len(rateLimitPolicies)),
		IPRestrictionPolicies: make([]*gatewayv1.IPRestrictionPolicy, 0, len(ipRestrictionPolicies)),
		TokenQuotaPolicies:    make([]*gatewayv1.TokenQuotaPolicy, 0, len(tokenQuotaPolicies)),
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
	for _, resource := range tokenQuotaPolicies {
		resources.TokenQuotaPolicies = append(resources.TokenQuotaPolicies, resource.DeepCopy())
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
	slices.SortFunc(resources.TokenQuotaPolicies, func(a, b *gatewayv1.TokenQuotaPolicy) int {
		return cmp.Compare(a.Name, b.Name)
	})
	return resources, nil
}
