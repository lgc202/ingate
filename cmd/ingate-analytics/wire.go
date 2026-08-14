//go:build wireinject

package main

import (
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
	*conf.Server,
	*conf.Data,
	*slog.Logger,
	serviceInstanceID,
) (*kratos.App, func(), error) {
	panic(wire.Build(
		data.ProviderSet,
		biz.ProviderSet,
		service.ProviderSet,
		server.ProviderSet,
		newApp,
	))
}
