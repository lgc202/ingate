//go:build wireinject

package main

import (
	"log/slog"

	kratos "github.com/go-kratos/kratos/v3"
	"github.com/google/wire"

	"github.com/lgc202/ingate/internal/adminapi/biz"
	accesscontrolbiz "github.com/lgc202/ingate/internal/adminapi/biz/accesscontrol"
	accesskeybiz "github.com/lgc202/ingate/internal/adminapi/biz/accesskey"
	certificatebiz "github.com/lgc202/ingate/internal/adminapi/biz/certificate"
	configurationbiz "github.com/lgc202/ingate/internal/adminapi/biz/configuration"
	gatewaybiz "github.com/lgc202/ingate/internal/adminapi/biz/gateway"
	ratelimitbiz "github.com/lgc202/ingate/internal/adminapi/biz/ratelimit"
	routebiz "github.com/lgc202/ingate/internal/adminapi/biz/route"
	tokenquotabiz "github.com/lgc202/ingate/internal/adminapi/biz/tokenquota"
	upstreambiz "github.com/lgc202/ingate/internal/adminapi/biz/upstream"
	"github.com/lgc202/ingate/internal/adminapi/conf"
	"github.com/lgc202/ingate/internal/adminapi/data"
	"github.com/lgc202/ingate/internal/adminapi/server"
	accesscontrolservice "github.com/lgc202/ingate/internal/adminapi/service/accesscontrol"
	accesskeyservice "github.com/lgc202/ingate/internal/adminapi/service/accesskey"
	certificateservice "github.com/lgc202/ingate/internal/adminapi/service/certificate"
	configurationservice "github.com/lgc202/ingate/internal/adminapi/service/configuration"
	gatewayservice "github.com/lgc202/ingate/internal/adminapi/service/gateway"
	healthservice "github.com/lgc202/ingate/internal/adminapi/service/health"
	ratelimitservice "github.com/lgc202/ingate/internal/adminapi/service/ratelimit"
	routeservice "github.com/lgc202/ingate/internal/adminapi/service/route"
	tokenquotaservice "github.com/lgc202/ingate/internal/adminapi/service/tokenquota"
	upstreamservice "github.com/lgc202/ingate/internal/adminapi/service/upstream"
)

func wireApp(*conf.Server, *conf.Data, *slog.Logger, serviceInstanceID) (*kratos.App, func(), error) {
	panic(wire.Build(
		server.NewHTTPServer,
		data.ProviderSet,
		biz.NewPolicyUsageFinder,
		gatewaybiz.NewUsecase,
		routebiz.NewUsecase,
		upstreambiz.NewUsecase,
		certificatebiz.NewUsecase,
		accesskeybiz.NewUsecase,
		ratelimitbiz.NewUsecase,
		accesscontrolbiz.NewUsecase,
		tokenquotabiz.NewUsecase,
		configurationbiz.NewUsecase,
		gatewayservice.NewService,
		routeservice.NewService,
		upstreamservice.NewService,
		certificateservice.NewService,
		accesskeyservice.NewService,
		ratelimitservice.NewService,
		accesscontrolservice.NewService,
		tokenquotaservice.NewService,
		configurationservice.NewService,
		healthservice.NewService,
		newApp,
	))
}
