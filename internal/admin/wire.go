//go:build wireinject

package admin

import (
	"context"
	"log/slog"

	"github.com/google/wire"

	"github.com/lgc202/ingate/internal/admin/accesskeyindex"
	"github.com/lgc202/ingate/internal/admin/handler"
	"github.com/lgc202/ingate/internal/admin/store"
)

func initializeServer(ctx context.Context, config Config, logger *slog.Logger) (*Server, func(), error) {
	wire.Build(
		newInfrastructure,
		wire.FieldsOf(new(*infrastructure), "database", "redisClient"),
		newResourceClient,
		store.New,
		accesskeyindex.New,
		newServices,
		handler.New,
		newHTTPServer,
		newServer,
	)
	return nil, nil, nil
}
