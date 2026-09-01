//go:build wireinject

package assistant

import (
	"context"
	"log/slog"

	kratos "github.com/go-kratos/kratos/v3"
	"github.com/google/wire"

	"github.com/lgc202/ingate/internal/assistant/biz"
	"github.com/lgc202/ingate/internal/assistant/biz/system"
	"github.com/lgc202/ingate/internal/assistant/conf"
	"github.com/lgc202/ingate/internal/assistant/data"
	"github.com/lgc202/ingate/internal/assistant/server"
	"github.com/lgc202/ingate/internal/assistant/service"
)

func wireApp(
	context.Context,
	*conf.Server,
	*conf.Data_MySQL,
	*conf.Temporal,
	*conf.Model,
	*slog.Logger,
	serviceInstanceID,
) (*kratos.App, func(), error) {
	panic(wire.Build(
		data.ProviderSet,
		biz.ProviderSet,
		service.ProviderSet,
		server.ProviderSet,
		wire.Bind(new(system.WorkerChecker), new(*server.Worker)),
		newKratosApp,
	))
}
