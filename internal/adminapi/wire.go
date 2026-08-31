//go:build wireinject

package adminapi

import (
	"context"
	"log/slog"

	kratos "github.com/go-kratos/kratos/v3"
	"github.com/google/wire"

	"github.com/lgc202/ingate/internal/adminapi/conf"
	"github.com/lgc202/ingate/internal/adminapi/data"
	"github.com/lgc202/ingate/internal/adminapi/server"
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
		BizProviderSet,
		ServiceProviderSet,
		server.ProviderSet,
		newKratosApp,
	))
}
