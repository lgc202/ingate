// Package data 实现 biz 层依赖的数据访问
package data

import (
	"github.com/google/wire"

	"github.com/lgc202/ingate/internal/adminapi/biz"
	"github.com/lgc202/ingate/internal/adminapi/biz/aiusage"
	"github.com/lgc202/ingate/internal/adminapi/biz/caller"
	"github.com/lgc202/ingate/internal/adminapi/biz/certificate"
	"github.com/lgc202/ingate/internal/adminapi/biz/gateway"
	"github.com/lgc202/ingate/internal/adminapi/biz/headertransformation"
	"github.com/lgc202/ingate/internal/adminapi/biz/iprestriction"
	"github.com/lgc202/ingate/internal/adminapi/biz/mockresponse"
	"github.com/lgc202/ingate/internal/adminapi/biz/pluginsource"
	"github.com/lgc202/ingate/internal/adminapi/biz/ratelimit"
	requestbiz "github.com/lgc202/ingate/internal/adminapi/biz/request"
	"github.com/lgc202/ingate/internal/adminapi/biz/route"
	"github.com/lgc202/ingate/internal/adminapi/biz/tokenquota"
	trafficbiz "github.com/lgc202/ingate/internal/adminapi/biz/traffic"
	"github.com/lgc202/ingate/internal/adminapi/biz/upstream"
	"github.com/lgc202/ingate/internal/adminapi/biz/wasmplugin"
	dataaiextproc "github.com/lgc202/ingate/internal/adminapi/data/aiextproc"
	dataanalytics "github.com/lgc202/ingate/internal/adminapi/data/analytics"
	"github.com/lgc202/ingate/internal/adminapi/data/apiserver"
	"github.com/lgc202/ingate/internal/adminapi/data/plugincatalog"
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
	apiserver.NewTokenQuotaPolicyRepository,
	apiserver.NewWasmPluginRepository,
	apiserver.NewPluginSourceRepository,
	apiserver.NewHeaderTransformationPolicyRepository,
	apiserver.NewMockResponsePolicyRepository,
	// 根 biz 只保留跨领域策略能力所需的只读边界
	wire.Bind(new(biz.GatewayGetter), new(*apiserver.GatewayRepository)),
	wire.Bind(new(biz.RouteGetter), new(*apiserver.RouteRepository)),
	wire.Bind(new(biz.CallerGetter), new(*apiserver.CallerRepository)),
	wire.Bind(new(biz.RateLimitPolicyLister), new(*apiserver.RateLimitPolicyRepository)),
	wire.Bind(new(biz.IPRestrictionPolicyLister), new(*apiserver.IPRestrictionPolicyRepository)),
	wire.Bind(new(biz.HeaderTransformationPolicyLister), new(*apiserver.HeaderTransformationPolicyRepository)),
	wire.Bind(new(biz.MockResponsePolicyLister), new(*apiserver.MockResponsePolicyRepository)),
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
	wire.Bind(new(headertransformation.Repository), new(*apiserver.HeaderTransformationPolicyRepository)),
	wire.Bind(new(mockresponse.Repository), new(*apiserver.MockResponsePolicyRepository)),
	wire.Bind(new(caller.Repository), new(*apiserver.CallerRepository)),
	wire.Bind(new(tokenquota.Repository), new(*apiserver.TokenQuotaPolicyRepository)),
	wire.Bind(new(wasmplugin.Repository), new(*apiserver.WasmPluginRepository)),
	wire.Bind(new(pluginsource.Repository), new(*apiserver.PluginSourceRepository)),
	wire.Bind(new(caller.RouteRepository), new(*apiserver.RouteRepository)),
)

var analyticsProviderSet = wire.NewSet(
	dataanalytics.NewClient,
	dataanalytics.NewAIUsageRepository,
	dataanalytics.NewRequestRepository,
	dataanalytics.NewTrafficRepository,
	wire.Bind(new(aiusage.Repository), new(*dataanalytics.AIUsageRepository)),
	wire.Bind(new(requestbiz.Repository), new(*dataanalytics.RequestRepository)),
	wire.Bind(new(trafficbiz.Repository), new(*dataanalytics.TrafficRepository)),
)

var aiExtProcProviderSet = wire.NewSet(
	dataaiextproc.NewClient,
	dataaiextproc.NewTokenQuotaUsageReader,
	wire.Bind(new(tokenquota.UsageReader), new(*dataaiextproc.TokenQuotaUsageReader)),
)

var pluginCatalogProviderSet = wire.NewSet(
	plugincatalog.NewCatalog,
	wire.Bind(new(wasmplugin.Catalog), new(*plugincatalog.Catalog)),
	wire.Bind(new(pluginsource.Catalog), new(*plugincatalog.Catalog)),
)

// ProviderSet 汇总 Admin API 的数据访问实现
var ProviderSet = wire.NewSet(apiserverProviderSet, analyticsProviderSet, aiExtProcProviderSet, pluginCatalogProviderSet)
