// Package data 实现 biz 层依赖的数据访问
package data

import (
	"github.com/google/wire"

	"github.com/lgc202/ingate/internal/adminapi/biz"
	"github.com/lgc202/ingate/internal/adminapi/biz/caller"
	"github.com/lgc202/ingate/internal/adminapi/biz/certificate"
	"github.com/lgc202/ingate/internal/adminapi/biz/gateway"
	"github.com/lgc202/ingate/internal/adminapi/biz/iprestriction"
	"github.com/lgc202/ingate/internal/adminapi/biz/ratelimit"
	requestbiz "github.com/lgc202/ingate/internal/adminapi/biz/request"
	"github.com/lgc202/ingate/internal/adminapi/biz/route"
	trafficbiz "github.com/lgc202/ingate/internal/adminapi/biz/traffic"
	"github.com/lgc202/ingate/internal/adminapi/biz/upstream"
	dataanalytics "github.com/lgc202/ingate/internal/adminapi/data/analytics"
	"github.com/lgc202/ingate/internal/adminapi/data/apiserver"
)

var apiserverProviderSet = wire.NewSet(
	apiserver.NewClient,
	apiserver.NewGatewayRepository,
	apiserver.NewRouteRepository,
	apiserver.NewUpstreamRepository,
	apiserver.NewCertificateRepository,
	apiserver.NewRateLimitPolicyRepository,
	apiserver.NewIPRestrictionPolicyRepository,
	apiserver.NewCallerRepository,
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
	wire.Bind(new(route.CallerRepository), new(*apiserver.CallerRepository)),
	wire.Bind(new(upstream.Repository), new(*apiserver.UpstreamRepository)),
	wire.Bind(new(upstream.RouteRepository), new(*apiserver.RouteRepository)),
	wire.Bind(new(certificate.Repository), new(*apiserver.CertificateRepository)),
	wire.Bind(new(certificate.GatewayRepository), new(*apiserver.GatewayRepository)),
	wire.Bind(new(ratelimit.Repository), new(*apiserver.RateLimitPolicyRepository)),
	wire.Bind(new(iprestriction.Repository), new(*apiserver.IPRestrictionPolicyRepository)),
	wire.Bind(new(caller.Repository), new(*apiserver.CallerRepository)),
	wire.Bind(new(caller.RouteRepository), new(*apiserver.RouteRepository)),
)

var analyticsProviderSet = wire.NewSet(
	dataanalytics.NewClient,
	dataanalytics.NewRequestRepository,
	dataanalytics.NewTrafficRepository,
	wire.Bind(new(requestbiz.Repository), new(*dataanalytics.RequestRepository)),
	wire.Bind(new(trafficbiz.Repository), new(*dataanalytics.TrafficRepository)),
)

// ProviderSet 汇总 Admin API 的数据访问实现
var ProviderSet = wire.NewSet(apiserverProviderSet, analyticsProviderSet)
