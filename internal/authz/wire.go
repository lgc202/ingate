//go:build wireinject

package authz

import (
	"log/slog"

	kratos "github.com/go-kratos/kratos/v3"
	"github.com/google/wire"

	"github.com/lgc202/ingate/internal/authz/caller"
	"github.com/lgc202/ingate/internal/authz/conf"
	"github.com/lgc202/ingate/internal/authz/server"
	"github.com/lgc202/ingate/internal/authz/service"
)

func wireApp(
	*conf.Server,
	*conf.Data_APIServer,
	*slog.Logger,
	serviceInstanceID,
) (*kratos.App, error) {
	panic(wire.Build(
		caller.NewIndex,
		service.ProviderSet,
		server.ProviderSet,
		newKratosApp,
	))
}
