//go:build wireinject

package console

import (
	"log/slog"

	kratos "github.com/go-kratos/kratos/v3"
	"github.com/google/wire"

	"github.com/lgc202/ingate/internal/console/conf"
	"github.com/lgc202/ingate/internal/console/server"
)

func wireApp(
	*conf.Server,
	*conf.Data_AdminAPI,
	*conf.Data_Assistant,
	*slog.Logger,
	serviceInstanceID,
) (*kratos.App, error) {
	panic(wire.Build(
		server.ProviderSet,
		newKratosApp,
	))
}
