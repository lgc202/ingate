//go:build wireinject

package adminapi

import (
	"context"
	"log/slog"

	kratos "github.com/go-kratos/kratos/v3"
	"github.com/google/wire"

	"github.com/lgc202/ingate/internal/adminapi/biz"
	aiusagebiz "github.com/lgc202/ingate/internal/adminapi/biz/aiusage"
	callerbiz "github.com/lgc202/ingate/internal/adminapi/biz/caller"
	certificatebiz "github.com/lgc202/ingate/internal/adminapi/biz/certificate"
	gatewaybiz "github.com/lgc202/ingate/internal/adminapi/biz/gateway"
	headertransformationbiz "github.com/lgc202/ingate/internal/adminapi/biz/headertransformation"
	iprestrictionbiz "github.com/lgc202/ingate/internal/adminapi/biz/iprestriction"
	mockresponsebiz "github.com/lgc202/ingate/internal/adminapi/biz/mockresponse"
	pluginsourcebiz "github.com/lgc202/ingate/internal/adminapi/biz/pluginsource"
	ratelimitbiz "github.com/lgc202/ingate/internal/adminapi/biz/ratelimit"
	requestbiz "github.com/lgc202/ingate/internal/adminapi/biz/request"
	routebiz "github.com/lgc202/ingate/internal/adminapi/biz/route"
	tokenquotabiz "github.com/lgc202/ingate/internal/adminapi/biz/tokenquota"
	trafficbiz "github.com/lgc202/ingate/internal/adminapi/biz/traffic"
	upstreambiz "github.com/lgc202/ingate/internal/adminapi/biz/upstream"
	wasmpluginbiz "github.com/lgc202/ingate/internal/adminapi/biz/wasmplugin"
	"github.com/lgc202/ingate/internal/adminapi/conf"
	"github.com/lgc202/ingate/internal/adminapi/data"
	"github.com/lgc202/ingate/internal/adminapi/server"
	aiusageservice "github.com/lgc202/ingate/internal/adminapi/service/aiusage"
	callerservice "github.com/lgc202/ingate/internal/adminapi/service/caller"
	certificateservice "github.com/lgc202/ingate/internal/adminapi/service/certificate"
	gatewayservice "github.com/lgc202/ingate/internal/adminapi/service/gateway"
	headertransformationservice "github.com/lgc202/ingate/internal/adminapi/service/headertransformation"
	healthservice "github.com/lgc202/ingate/internal/adminapi/service/health"
	iprestrictionservice "github.com/lgc202/ingate/internal/adminapi/service/iprestriction"
	mockresponseservice "github.com/lgc202/ingate/internal/adminapi/service/mockresponse"
	pluginsourceservice "github.com/lgc202/ingate/internal/adminapi/service/pluginsource"
	ratelimitservice "github.com/lgc202/ingate/internal/adminapi/service/ratelimit"
	requestservice "github.com/lgc202/ingate/internal/adminapi/service/request"
	routeservice "github.com/lgc202/ingate/internal/adminapi/service/route"
	tokenquotaservice "github.com/lgc202/ingate/internal/adminapi/service/tokenquota"
	trafficservice "github.com/lgc202/ingate/internal/adminapi/service/traffic"
	upstreamservice "github.com/lgc202/ingate/internal/adminapi/service/upstream"
	wasmpluginservice "github.com/lgc202/ingate/internal/adminapi/service/wasmplugin"
)

// Admin API 按资源拆成 biz 子包，而子包依赖根 biz 的共享边界；
// 根包不能反向导入子包，因此由应用装配文件汇总各资源用例。
var bizProviderSet = wire.NewSet(
	biz.NewPolicyUsageFinder,
	biz.NewPluginUsageFinder,
	biz.NewPluginInstallationChecker,
	wire.Bind(new(wasmpluginbiz.PolicyUsageLister), new(*biz.PluginUsageFinder)),
	aiusagebiz.NewUsecase,
	callerbiz.NewUsecase,
	gatewaybiz.NewUsecase,
	headertransformationbiz.NewUsecase,
	mockresponsebiz.NewUsecase,
	routebiz.NewUsecase,
	upstreambiz.NewUsecase,
	certificatebiz.NewUsecase,
	ratelimitbiz.NewUsecase,
	iprestrictionbiz.NewUsecase,
	pluginsourcebiz.NewUsecase,
	requestbiz.NewUsecase,
	trafficbiz.NewUsecase,
	tokenquotabiz.NewUsecase,
	wasmpluginbiz.NewUsecase,
)

// service 子包同样依赖根 service 的协议转换能力，由应用装配文件统一汇总。
var serviceProviderSet = wire.NewSet(
	aiusageservice.NewService,
	callerservice.NewService,
	gatewayservice.NewService,
	headertransformationservice.NewService,
	mockresponseservice.NewService,
	routeservice.NewService,
	upstreamservice.NewService,
	certificateservice.NewService,
	ratelimitservice.NewService,
	iprestrictionservice.NewService,
	pluginsourceservice.NewService,
	requestservice.NewService,
	trafficservice.NewService,
	tokenquotaservice.NewService,
	healthservice.NewService,
	wasmpluginservice.NewService,
)

func wireApp(
	context.Context,
	*conf.Server,
	*conf.Data,
	*slog.Logger,
	serviceInstanceID,
) (*kratos.App, func(), error) {
	panic(wire.Build(
		data.ProviderSet,
		bizProviderSet,
		serviceProviderSet,
		server.ProviderSet,
		newKratosApp,
	))
}
