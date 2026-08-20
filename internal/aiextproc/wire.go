//go:build wireinject

package aiextproc

import (
	"log/slog"

	kratos "github.com/go-kratos/kratos/v3"
	"github.com/google/wire"

	"github.com/lgc202/ingate/internal/aiextproc/conf"
	"github.com/lgc202/ingate/internal/aiextproc/modelservice"
	"github.com/lgc202/ingate/internal/aiextproc/server"
	"github.com/lgc202/ingate/internal/aiextproc/service"
)

func wireApp(
	*conf.Server,
	*conf.Data_APIServer,
	*slog.Logger,
	serviceInstanceID,
) (*kratos.App, error) {
	panic(wire.Build(
		modelservice.NewCache,
		service.ProviderSet,
		server.ProviderSet,
		newKratosApp,
	))
}
