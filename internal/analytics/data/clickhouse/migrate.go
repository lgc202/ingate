package clickhouse

import (
	"context"
	"embed"
	"errors"
	"fmt"
	"io/fs"

	"github.com/pressly/goose/v3"

	"github.com/lgc202/ingate/internal/analytics/conf"
	"github.com/lgc202/ingate/internal/pkg/clickhousex"
)

const schemaMigrationTableName = "ingate_schema_migrations"

// migrationFiles 随 Analytics 二进制发布，部署时不依赖源码目录
//
//go:embed migrations/*.sql
var migrationFiles embed.FS

// Migrate 按版本应用 Analytics 的 ClickHouse 表结构变更
//
// 正常服务进程不执行 DDL，生产环境可以为运行账号移除建表权限。调用方负责在
// 服务启动前完成迁移，Migrate 返回本次实际应用的版本数量
func Migrate(ctx context.Context, config *conf.Data_ClickHouse) (applied int, err error) {
	db, err := clickhousex.NewDB(clientConfig(config))
	if err != nil {
		return 0, err
	}
	defer func() {
		if closeErr := db.Close(); closeErr != nil {
			err = errors.Join(err, fmt.Errorf("close ClickHouse migration connection: %w", closeErr))
		}
	}()

	files, err := fs.Sub(migrationFiles, "migrations")
	if err != nil {
		return 0, fmt.Errorf("open embedded ClickHouse migrations: %w", err)
	}
	provider, err := goose.NewProvider(
		goose.DialectClickHouse,
		db,
		files,
		goose.WithTableName(schemaMigrationTableName),
		goose.WithDisableGlobalRegistry(true),
	)
	if err != nil {
		return 0, fmt.Errorf("create ClickHouse migration provider: %w", err)
	}
	results, err := provider.Up(ctx)
	if err != nil {
		return 0, fmt.Errorf("migrate ClickHouse analytics schema: %w", err)
	}
	return len(results), nil
}
