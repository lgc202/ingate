//go:build wireinject

package controller

import (
	"log/slog"

	kratos "github.com/go-kratos/kratos/v3"
	"github.com/google/wire"

	"github.com/lgc202/ingate/internal/controller/conf"
	controllerserver "github.com/lgc202/ingate/internal/controller/server"
)

func wireApp(
	*conf.Server,
	*conf.Data_APIServer,
	*conf.Delivery,
	*conf.ResourceWatch,
	*slog.Logger,
	serviceInstanceID,
) (*kratos.App, error) {
	panic(wire.Build(
		newAPIClient,
		newSnapshotCache,
		newDelivery,
		newReconciler,
		newXDSService,
		controllerserver.NewHTTPServer,
		controllerserver.NewGRPCServer,
		newKratosApp,
	))
}
