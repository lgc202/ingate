//go:build wireinject

package apiserver

import (
	"log/slog"

	kratos "github.com/go-kratos/kratos/v3"
	"github.com/google/wire"

	"github.com/lgc202/ingate/internal/apiserver/conf"
)

func wireApp(
	*conf.Server,
	*conf.Server_HTTP,
	*conf.Data_Etcd,
	*slog.Logger,
	serviceInstanceID,
) (*kratos.App, func(), error) {
	panic(wire.Build(newServer, newKratosApp))
}
