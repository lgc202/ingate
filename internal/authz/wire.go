//go:build wireinject

package authz

import (
	"log/slog"

	kratos "github.com/go-kratos/kratos/v3"
	"github.com/google/wire"

	"github.com/lgc202/ingate/internal/authz/biz"
	"github.com/lgc202/ingate/internal/authz/conf"
	"github.com/lgc202/ingate/internal/authz/data"
	"github.com/lgc202/ingate/internal/authz/server"
	"github.com/lgc202/ingate/internal/authz/service"
)

func wireApp(
	*conf.Server,
	*conf.Data_APIServer,
	*conf.Data_Redis,
	*slog.Logger,
	serviceInstanceID,
) (*kratos.App, error) {
	panic(wire.Build(
		data.ProviderSet,
		biz.ProviderSet,
		service.ProviderSet,
		server.ProviderSet,
		wire.Bind(new(server.Readiness), new(*data.Readiness)),
		newKratosApp,
	))
}
