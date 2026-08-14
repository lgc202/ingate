// Package data 装配 Analytics 使用的数据访问实现
package data

import (
	"log/slog"

	"github.com/google/wire"

	"github.com/lgc202/ingate/internal/analytics/biz/request"
	"github.com/lgc202/ingate/internal/analytics/biz/traffic"
	"github.com/lgc202/ingate/internal/analytics/conf"
	"github.com/lgc202/ingate/internal/analytics/data/clickhouse"
)

// ProviderSet 汇总 Analytics 的数据访问实现
var ProviderSet = wire.NewSet(
	NewClickHouseStore,
	wire.Bind(new(request.RecordStore), new(*clickhouse.Store)),
	wire.Bind(new(request.QueryStore), new(*clickhouse.Store)),
	wire.Bind(new(traffic.QueryStore), new(*clickhouse.Store)),
)

// NewClickHouseStore 创建 ClickHouse 存储，并把连接释放交给 Wire cleanup
func NewClickHouseStore(
	config *conf.Data,
	logger *slog.Logger,
) (*clickhouse.Store, func(), error) {
	store, err := clickhouse.NewStore(config.GetClickHouse())
	if err != nil {
		return nil, nil, err
	}
	cleanup := func() {
		if err := store.Close(); err != nil {
			logger.Error("close ClickHouse failed", "error", err)
		}
	}
	return store, cleanup, nil
}
