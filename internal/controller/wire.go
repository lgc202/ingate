//go:build wireinject

package controller

import (
	"log/slog"

	kratos "github.com/go-kratos/kratos/v3"
	"github.com/google/wire"

	controllerbiz "github.com/lgc202/ingate/internal/controller/biz"
	"github.com/lgc202/ingate/internal/controller/conf"
	controllerdata "github.com/lgc202/ingate/internal/controller/data"
	controllerserver "github.com/lgc202/ingate/internal/controller/server"
)

func wireApp(
	*conf.Server,
	*conf.Data_APIServer,
	*conf.Data_Wasm,
	*conf.Delivery,
	*conf.ResourceWatch,
	*slog.Logger,
	serviceInstanceID,
) (*kratos.App, error) {
	panic(wire.Build(
		controllerbiz.ProviderSet,
		controllerdata.ProviderSet,
		controllerserver.ProviderSet,
		newKratosApp,
	))
}
