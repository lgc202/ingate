//go:build wireinject

package main

import (
	"log/slog"

	kratos "github.com/go-kratos/kratos/v3"
	"github.com/google/wire"

	"github.com/lgc202/ingate/internal/aiproxy/accesskey"
	"github.com/lgc202/ingate/internal/aiproxy/conf"
	"github.com/lgc202/ingate/internal/aiproxy/data"
	"github.com/lgc202/ingate/internal/aiproxy/extproc"
	"github.com/lgc202/ingate/internal/aiproxy/server"
)

func wireApp(*conf.Server, *conf.Data, *slog.Logger, serviceInstanceID) (*kratos.App, func(), error) {
	panic(wire.Build(
		data.ProviderSet,
		accesskey.NewAuthenticator,
		extproc.NewServer,
		server.NewHTTPServer,
		server.NewGRPCServer,
		newApp,
	))
}
