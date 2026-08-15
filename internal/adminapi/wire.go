//go:build wireinject

package adminapi

import (
	"log/slog"

	kratos "github.com/go-kratos/kratos/v3"
	"github.com/google/wire"

	"github.com/lgc202/ingate/internal/adminapi/auth"
	"github.com/lgc202/ingate/internal/adminapi/biz"
	certificatebiz "github.com/lgc202/ingate/internal/adminapi/biz/certificate"
	configurationbiz "github.com/lgc202/ingate/internal/adminapi/biz/configuration"
	gatewaybiz "github.com/lgc202/ingate/internal/adminapi/biz/gateway"
	iprestrictionbiz "github.com/lgc202/ingate/internal/adminapi/biz/iprestriction"
	ratelimitbiz "github.com/lgc202/ingate/internal/adminapi/biz/ratelimit"
	routebiz "github.com/lgc202/ingate/internal/adminapi/biz/route"
	upstreambiz "github.com/lgc202/ingate/internal/adminapi/biz/upstream"
	"github.com/lgc202/ingate/internal/adminapi/conf"
	"github.com/lgc202/ingate/internal/adminapi/data"
	"github.com/lgc202/ingate/internal/adminapi/server"
	authenticationservice "github.com/lgc202/ingate/internal/adminapi/service/authentication"
	certificateservice "github.com/lgc202/ingate/internal/adminapi/service/certificate"
	configurationservice "github.com/lgc202/ingate/internal/adminapi/service/configuration"
	gatewayservice "github.com/lgc202/ingate/internal/adminapi/service/gateway"
	healthservice "github.com/lgc202/ingate/internal/adminapi/service/health"
	iprestrictionservice "github.com/lgc202/ingate/internal/adminapi/service/iprestriction"
	ratelimitservice "github.com/lgc202/ingate/internal/adminapi/service/ratelimit"
	routeservice "github.com/lgc202/ingate/internal/adminapi/service/route"
	upstreamservice "github.com/lgc202/ingate/internal/adminapi/service/upstream"
)

// bizProviderSet 汇总各资源的业务服务
var bizProviderSet = wire.NewSet(
	biz.NewPolicyUsageFinder,
	gatewaybiz.NewService,
	routebiz.NewService,
	upstreambiz.NewService,
	certificatebiz.NewService,
	ratelimitbiz.NewService,
	iprestrictionbiz.NewService,
	configurationbiz.NewService,
)

// serviceProviderSet 汇总 Admin API 的协议服务
var serviceProviderSet = wire.NewSet(
	authenticationservice.NewService,
	gatewayservice.NewService,
	routeservice.NewService,
	upstreamservice.NewService,
	certificateservice.NewService,
	ratelimitservice.NewService,
	iprestrictionservice.NewService,
	configurationservice.NewService,
	healthservice.NewService,
)

func wireApp(
	*conf.Server,
	*conf.Data,
	*conf.Authentication,
	*slog.Logger,
	serviceInstanceID,
) (*kratos.App, error) {
	panic(wire.Build(
		auth.ProviderSet,
		data.ProviderSet,
		bizProviderSet,
		serviceProviderSet,
		server.ProviderSet,
		newKratosApp,
	))
}
