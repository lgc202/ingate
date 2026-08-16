// Package data 实现 biz 层依赖的数据访问
package data

import (
	"fmt"

	"github.com/google/wire"
	"k8s.io/client-go/tools/clientcmd"

	"github.com/lgc202/ingate/internal/adminapi/biz"
	"github.com/lgc202/ingate/internal/adminapi/biz/certificate"
	"github.com/lgc202/ingate/internal/adminapi/biz/configuration"
	"github.com/lgc202/ingate/internal/adminapi/biz/gateway"
	"github.com/lgc202/ingate/internal/adminapi/biz/iprestriction"
	"github.com/lgc202/ingate/internal/adminapi/biz/ratelimit"
	requestbiz "github.com/lgc202/ingate/internal/adminapi/biz/request"
	"github.com/lgc202/ingate/internal/adminapi/biz/route"
	trafficbiz "github.com/lgc202/ingate/internal/adminapi/biz/traffic"
	"github.com/lgc202/ingate/internal/adminapi/biz/upstream"
	"github.com/lgc202/ingate/internal/adminapi/conf"
	dataanalytics "github.com/lgc202/ingate/internal/adminapi/data/analytics"
	"github.com/lgc202/ingate/internal/adminapi/data/apiserver"
	clientset "github.com/lgc202/ingate/pkg/generated/clientset/versioned"
)

// ProviderSet 汇总 Admin API 的数据访问实现
var ProviderSet = wire.NewSet(
	NewResourceClient,
	dataanalytics.NewClient,
	dataanalytics.NewRequestRepository,
	dataanalytics.NewTrafficRepository,
	apiserver.NewGatewayRepository,
	apiserver.NewRouteRepository,
	apiserver.NewUpstreamRepository,
	apiserver.NewCertificateRepository,
	apiserver.NewRateLimitPolicyRepository,
	apiserver.NewIPRestrictionPolicyRepository,
	// 根 biz 只保留跨领域策略能力所需的只读边界
	wire.Bind(new(biz.GatewayGetter), new(*apiserver.GatewayRepository)),
	wire.Bind(new(biz.RouteGetter), new(*apiserver.RouteRepository)),
	wire.Bind(new(biz.RateLimitPolicyLister), new(*apiserver.RateLimitPolicyRepository)),
	wire.Bind(new(biz.IPRestrictionPolicyLister), new(*apiserver.IPRestrictionPolicyRepository)),
	// 每个领域声明自己真实消费的 Repository，避免 biz 子包相互依赖
	wire.Bind(new(gateway.Repository), new(*apiserver.GatewayRepository)),
	wire.Bind(new(gateway.RouteRepository), new(*apiserver.RouteRepository)),
	wire.Bind(new(gateway.CertificateRepository), new(*apiserver.CertificateRepository)),
	wire.Bind(new(route.Repository), new(*apiserver.RouteRepository)),
	wire.Bind(new(route.GatewayRepository), new(*apiserver.GatewayRepository)),
	wire.Bind(new(route.UpstreamRepository), new(*apiserver.UpstreamRepository)),
	wire.Bind(new(upstream.Repository), new(*apiserver.UpstreamRepository)),
	wire.Bind(new(upstream.RouteRepository), new(*apiserver.RouteRepository)),
	wire.Bind(new(certificate.Repository), new(*apiserver.CertificateRepository)),
	wire.Bind(new(certificate.GatewayRepository), new(*apiserver.GatewayRepository)),
	wire.Bind(new(ratelimit.Repository), new(*apiserver.RateLimitPolicyRepository)),
	wire.Bind(new(iprestriction.Repository), new(*apiserver.IPRestrictionPolicyRepository)),
	wire.Bind(new(requestbiz.Repository), new(*dataanalytics.RequestRepository)),
	wire.Bind(new(trafficbiz.Repository), new(*dataanalytics.TrafficRepository)),
	wire.Bind(new(configuration.GatewayRepository), new(*apiserver.GatewayRepository)),
	wire.Bind(new(configuration.RouteRepository), new(*apiserver.RouteRepository)),
	wire.Bind(new(configuration.UpstreamRepository), new(*apiserver.UpstreamRepository)),
	wire.Bind(new(configuration.CertificateRepository), new(*apiserver.CertificateRepository)),
	wire.Bind(new(configuration.RateLimitPolicyRepository), new(*apiserver.RateLimitPolicyRepository)),
	wire.Bind(new(configuration.IPRestrictionPolicyRepository), new(*apiserver.IPRestrictionPolicyRepository)),
)

// NewResourceClient 创建 Admin API 使用的声明式资源客户端
func NewResourceClient(config *conf.Data) (clientset.Interface, error) {
	restConfig, err := clientcmd.BuildConfigFromFlags(config.GetApiserver().GetMaster(), config.GetApiserver().GetKubeconfig())
	if err != nil {
		return nil, fmt.Errorf("build apiserver client config: %w", err)
	}
	resourceClient, err := clientset.NewForConfig(restConfig)
	if err != nil {
		return nil, fmt.Errorf("create apiserver client: %w", err)
	}
	return resourceClient, nil
}
