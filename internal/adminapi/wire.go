//go:build wireinject

package adminapi

import (
	"log/slog"

	kratos "github.com/go-kratos/kratos/v3"
	"github.com/google/wire"

	"github.com/lgc202/ingate/internal/adminapi/biz"
	aiusagebiz "github.com/lgc202/ingate/internal/adminapi/biz/aiusage"
	callerbiz "github.com/lgc202/ingate/internal/adminapi/biz/caller"
	certificatebiz "github.com/lgc202/ingate/internal/adminapi/biz/certificate"
	gatewaybiz "github.com/lgc202/ingate/internal/adminapi/biz/gateway"
	iprestrictionbiz "github.com/lgc202/ingate/internal/adminapi/biz/iprestriction"
	ratelimitbiz "github.com/lgc202/ingate/internal/adminapi/biz/ratelimit"
	requestbiz "github.com/lgc202/ingate/internal/adminapi/biz/request"
	routebiz "github.com/lgc202/ingate/internal/adminapi/biz/route"
	tokenquotabiz "github.com/lgc202/ingate/internal/adminapi/biz/tokenquota"
	trafficbiz "github.com/lgc202/ingate/internal/adminapi/biz/traffic"
	upstreambiz "github.com/lgc202/ingate/internal/adminapi/biz/upstream"
	"github.com/lgc202/ingate/internal/adminapi/conf"
	"github.com/lgc202/ingate/internal/adminapi/data"
	"github.com/lgc202/ingate/internal/adminapi/server"
	aiusageservice "github.com/lgc202/ingate/internal/adminapi/service/aiusage"
	callerservice "github.com/lgc202/ingate/internal/adminapi/service/caller"
	certificateservice "github.com/lgc202/ingate/internal/adminapi/service/certificate"
	gatewayservice "github.com/lgc202/ingate/internal/adminapi/service/gateway"
	healthservice "github.com/lgc202/ingate/internal/adminapi/service/health"
	iprestrictionservice "github.com/lgc202/ingate/internal/adminapi/service/iprestriction"
	ratelimitservice "github.com/lgc202/ingate/internal/adminapi/service/ratelimit"
	requestservice "github.com/lgc202/ingate/internal/adminapi/service/request"
	routeservice "github.com/lgc202/ingate/internal/adminapi/service/route"
	tokenquotaservice "github.com/lgc202/ingate/internal/adminapi/service/tokenquota"
	trafficservice "github.com/lgc202/ingate/internal/adminapi/service/traffic"
	upstreamservice "github.com/lgc202/ingate/internal/adminapi/service/upstream"
)

// bizProviderSet 汇总各资源的业务服务
var bizProviderSet = wire.NewSet(
	biz.NewPolicyUsageFinder,
	aiusagebiz.NewService,
	callerbiz.NewService,
	gatewaybiz.NewService,
	routebiz.NewService,
	upstreambiz.NewService,
	certificatebiz.NewService,
	ratelimitbiz.NewService,
	iprestrictionbiz.NewService,
	requestbiz.NewService,
	trafficbiz.NewService,
	tokenquotabiz.NewService,
)

// serviceProviderSet 汇总 Admin API 的协议服务
var serviceProviderSet = wire.NewSet(
	aiusageservice.NewService,
	callerservice.NewService,
	gatewayservice.NewService,
	routeservice.NewService,
	upstreamservice.NewService,
	certificateservice.NewService,
	ratelimitservice.NewService,
	iprestrictionservice.NewService,
	requestservice.NewService,
	trafficservice.NewService,
	tokenquotaservice.NewService,
	healthservice.NewService,
)

func wireApp(
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
