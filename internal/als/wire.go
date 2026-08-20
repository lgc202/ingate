//go:build wireinject

package als

import (
	"log/slog"

	kratos "github.com/go-kratos/kratos/v3"
	"github.com/google/wire"

	"github.com/lgc202/ingate/internal/als/biz"
	"github.com/lgc202/ingate/internal/als/conf"
	"github.com/lgc202/ingate/internal/als/data"
	"github.com/lgc202/ingate/internal/als/server"
	"github.com/lgc202/ingate/internal/als/service"
)

func wireApp(
	*conf.Server,
	*conf.Data_Kafka,
	*conf.Data_DiskQueue,
	*slog.Logger,
	serviceInstanceID,
) (*kratos.App, func(), error) {
	panic(wire.Build(
		data.ProviderSet,
		biz.ProviderSet,
		service.ProviderSet,
		server.ProviderSet,
		newKratosApp,
	))
}
