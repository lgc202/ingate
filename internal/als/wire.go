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
	"github.com/lgc202/ingate/internal/pkg/telemetry"
)

func wireApp(
	*conf.Server,
	*conf.Data,
	*slog.Logger,
	*telemetry.Tracing,
	serviceInstanceID,
) (*kratos.App, func(), error) {
	panic(wire.Build(
		wire.FieldsOf(new(*conf.Data), "Kafka", "DiskQueue", "ReliabilityMode"),
		data.ProviderSet,
		biz.ProviderSet,
		service.ProviderSet,
		server.ProviderSet,
		newTopicContract,
		newKratosApp,
	))
}
