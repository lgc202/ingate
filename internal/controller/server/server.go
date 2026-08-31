// Package server 装配 ingate-controller 的 Kratos transport 和 xDS 服务。
package server

import (
	"log/slog"

	cachev3 "github.com/envoyproxy/go-control-plane/pkg/cache/v3"
	"github.com/google/wire"

	"github.com/lgc202/ingate/internal/controller/biz/delivery"
	"github.com/lgc202/ingate/internal/controller/server/xds"
)

// ProviderSet 汇总 Controller 的运维 HTTP、ADS gRPC 和 xDS 发布实现。
var ProviderSet = wire.NewSet(
	NewSnapshotCache,
	xds.NewPublisher,
	wire.Bind(new(delivery.Publisher), new(*xds.Publisher)),
	NewXDSService,
	NewHTTPServer,
	NewGRPCServer,
)

// NewSnapshotCache 创建 Controller 配置域使用的 xDS Snapshot Cache。
func NewSnapshotCache(logger *slog.Logger) cachev3.SnapshotCache {
	return xds.NewSnapshotCache(xds.NewSlogLogger(logger.With("component", "xds")))
}

// NewXDSService 创建共享 Snapshot Cache 和配置发布状态的 ADS 服务。
func NewXDSService(
	cache cachev3.SnapshotCache,
	configDelivery *delivery.Delivery,
	logger *slog.Logger,
) *xds.Service {
	xdsLogger := logger.With("component", "xds")
	callbacks := xds.NewCallbacks(configDelivery.HandleXDSEvent, xdsLogger)
	return xds.NewService(
		cache,
		callbacks,
		xds.NewSlogLogger(xdsLogger),
	)
}
