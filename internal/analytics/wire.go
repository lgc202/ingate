//go:build wireinject

package analytics

import (
	"context"
	"log/slog"

	kratos "github.com/go-kratos/kratos/v3"
	"github.com/google/wire"

	"github.com/lgc202/ingate/internal/analytics/biz"
	"github.com/lgc202/ingate/internal/analytics/conf"
	"github.com/lgc202/ingate/internal/analytics/data"
	"github.com/lgc202/ingate/internal/analytics/server"
	"github.com/lgc202/ingate/internal/analytics/service"
)

func wireApp(
	context.Context,
	*conf.Server,
	*conf.Data_Kafka,
	*conf.Data_ClickHouse,
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
