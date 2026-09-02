// Package data 实现 biz 层依赖的数据访问。
package data

import (
	"github.com/google/wire"

	"github.com/lgc202/ingate/internal/adminapi/biz/aiusage"
	"github.com/lgc202/ingate/internal/adminapi/biz/caller"
	"github.com/lgc202/ingate/internal/adminapi/biz/certificate"
	"github.com/lgc202/ingate/internal/adminapi/biz/gateway"
	"github.com/lgc202/ingate/internal/adminapi/biz/headertransformation"
	"github.com/lgc202/ingate/internal/adminapi/biz/iprestriction"
	"github.com/lgc202/ingate/internal/adminapi/biz/mockresponse"
	"github.com/lgc202/ingate/internal/adminapi/biz/plugin"
	"github.com/lgc202/ingate/internal/adminapi/biz/pluginsource"
	"github.com/lgc202/ingate/internal/adminapi/biz/policy"
	"github.com/lgc202/ingate/internal/adminapi/biz/ratelimit"
	requestbiz "github.com/lgc202/ingate/internal/adminapi/biz/request"
	"github.com/lgc202/ingate/internal/adminapi/biz/route"
	servicebiz "github.com/lgc202/ingate/internal/adminapi/biz/service"
	"github.com/lgc202/ingate/internal/adminapi/biz/tokenquota"
	trafficbiz "github.com/lgc202/ingate/internal/adminapi/biz/traffic"
	"github.com/lgc202/ingate/internal/adminapi/biz/wasmplugin"
	dataaiextproc "github.com/lgc202/ingate/internal/adminapi/data/aiextproc"
	dataanalytics "github.com/lgc202/ingate/internal/adminapi/data/analytics"
	"github.com/lgc202/ingate/internal/adminapi/data/apiserver"
	"github.com/lgc202/ingate/internal/adminapi/data/plugincatalog"
)

var apiserverProviderSet = wire.NewSet(
	apiserver.NewClient,
	apiserver.NewGatewayStore,
	apiserver.NewRouteStore,
	apiserver.NewUpstreamStore,
	apiserver.NewCertificateStore,
	apiserver.NewRateLimitPolicyStore,
	apiserver.NewIPRestrictionPolicyStore,
	apiserver.NewCallerStore,
	apiserver.NewTokenQuotaPolicyStore,
	apiserver.NewWasmPluginStore,
	apiserver.NewPluginSourceStore,
	apiserver.NewHeaderTransformationPolicyStore,
	apiserver.NewMockResponsePolicyStore,
	// 共享策略和插件角色只依赖各自消费的窄读取边界。
	wire.Bind(new(policy.GatewayReader), new(*apiserver.GatewayStore)),
	wire.Bind(new(policy.RouteReader), new(*apiserver.RouteStore)),
	wire.Bind(new(policy.CallerReader), new(*apiserver.CallerStore)),
	wire.Bind(new(policy.RateLimitPolicyLister), new(*apiserver.RateLimitPolicyStore)),
	wire.Bind(new(policy.IPRestrictionPolicyLister), new(*apiserver.IPRestrictionPolicyStore)),
	wire.Bind(new(policy.HeaderTransformationPolicyLister), new(*apiserver.HeaderTransformationPolicyStore)),
	wire.Bind(new(policy.MockResponsePolicyLister), new(*apiserver.MockResponsePolicyStore)),
	wire.Bind(new(plugin.WasmPluginGetter), new(*apiserver.WasmPluginStore)),
	// 每个领域声明自己真实消费的边界，避免 biz 子包相互依赖。
	wire.Bind(new(gateway.Store), new(*apiserver.GatewayStore)),
	wire.Bind(new(gateway.RouteLister), new(*apiserver.RouteStore)),
	wire.Bind(new(gateway.CertificateReader), new(*apiserver.CertificateStore)),
	wire.Bind(new(route.Store), new(*apiserver.RouteStore)),
	wire.Bind(new(route.GatewayReader), new(*apiserver.GatewayStore)),
	wire.Bind(new(route.ServiceReader), new(*apiserver.UpstreamStore)),
	wire.Bind(new(route.CallerLister), new(*apiserver.CallerStore)),
	wire.Bind(new(servicebiz.Store), new(*apiserver.UpstreamStore)),
	wire.Bind(new(servicebiz.RouteLister), new(*apiserver.RouteStore)),
	wire.Bind(new(certificate.Store), new(*apiserver.CertificateStore)),
	wire.Bind(new(certificate.GatewayLister), new(*apiserver.GatewayStore)),
	wire.Bind(new(ratelimit.Store), new(*apiserver.RateLimitPolicyStore)),
	wire.Bind(new(iprestriction.Store), new(*apiserver.IPRestrictionPolicyStore)),
	wire.Bind(new(headertransformation.Store), new(*apiserver.HeaderTransformationPolicyStore)),
	wire.Bind(new(mockresponse.Store), new(*apiserver.MockResponsePolicyStore)),
	wire.Bind(new(caller.Store), new(*apiserver.CallerStore)),
	wire.Bind(new(caller.TokenQuotaPolicyLister), new(*apiserver.TokenQuotaPolicyStore)),
	wire.Bind(new(tokenquota.Store), new(*apiserver.TokenQuotaPolicyStore)),
	wire.Bind(new(tokenquota.CallerReader), new(*apiserver.CallerStore)),
	wire.Bind(new(wasmplugin.Store), new(*apiserver.WasmPluginStore)),
	wire.Bind(new(pluginsource.Store), new(*apiserver.PluginSourceStore)),
	wire.Bind(new(caller.RouteReader), new(*apiserver.RouteStore)),
)

var analyticsProviderSet = wire.NewSet(
	dataanalytics.NewClient,
	dataanalytics.NewAIUsageRepository,
	dataanalytics.NewRequestRepository,
	dataanalytics.NewTrafficRepository,
	wire.Bind(new(aiusage.Analyzer), new(*dataanalytics.AIUsageRepository)),
	wire.Bind(new(requestbiz.Reader), new(*dataanalytics.RequestRepository)),
	wire.Bind(new(trafficbiz.Analyzer), new(*dataanalytics.TrafficRepository)),
)

var aiExtProcProviderSet = wire.NewSet(
	dataaiextproc.NewClient,
	wire.Bind(new(tokenquota.UsageReader), new(*dataaiextproc.Client)),
)

var pluginCatalogProviderSet = wire.NewSet(
	plugincatalog.NewCatalog,
	wire.Bind(new(wasmplugin.Catalog), new(*plugincatalog.Catalog)),
	wire.Bind(new(pluginsource.Catalog), new(*plugincatalog.Catalog)),
)

// ProviderSet 汇总 Admin API 的数据访问实现。
var ProviderSet = wire.NewSet(
	apiserverProviderSet,
	analyticsProviderSet,
	aiExtProcProviderSet,
	pluginCatalogProviderSet,
)
