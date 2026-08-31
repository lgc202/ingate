package clickhouse

import (
	"context"
	"database/sql"
	"embed"
	"errors"
	"fmt"
	"io/fs"
	"time"

	"github.com/pressly/goose/v3"

	"github.com/lgc202/ingate/internal/analytics/conf"
	"github.com/lgc202/ingate/internal/pkg/clickhouseclient"
)

const (
	schemaMigrationTableName = "ingate_schema_migrations"
	// requiredSchemaVersion 必须与 migrations 目录中的最高版本保持一致。
	// 已发布的 migration 不得原地修改，表结构变更必须追加新版本。
	requiredSchemaVersion int64 = 2
)

// migrationFiles 随 Analytics 二进制发布，部署时不依赖源码目录。
//
//go:embed migrations/*.sql
var migrationFiles embed.FS

// Migrate 按版本应用 Analytics 的 ClickHouse 表结构变更。
//
// 正常服务进程不执行 DDL，生产环境可以为运行账号移除建表权限。调用方负责在
// 服务启动前完成迁移，Migrate 返回本次实际应用的版本数量。
func Migrate(ctx context.Context, config *conf.Data_ClickHouse) (applied int, err error) {
	db, err := clickhouseclient.OpenDB(clientConfig(config))
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
	migrationSources := provider.ListSources()
	embeddedVersion := migrationSources[len(migrationSources)-1].Version
	if embeddedVersion != requiredSchemaVersion {
		return 0, fmt.Errorf(
			"embedded ClickHouse migrations end at version %d, but Analytics requires version %d",
			embeddedVersion,
			requiredSchemaVersion,
		)
	}
	results, err := provider.Up(ctx)
	if err != nil {
		return 0, fmt.Errorf("migrate ClickHouse analytics schema: %w", err)
	}
	if err := applyRetention(ctx, db, config.GetRetention()); err != nil {
		return 0, err
	}
	return len(results), nil
}

func applyRetention(ctx context.Context, db *sql.DB, retention *conf.Data_ClickHouse_Retention) error {
	tables := []struct {
		name      string
		timestamp string
		seconds   int64
	}{
		{
			name:      requestTableName,
			timestamp: "started_at",
			seconds:   int64(retention.GetRequestRecords().AsDuration() / time.Second),
		},
		{
			name:      minuteMetricsTableName,
			timestamp: "started_at",
			seconds:   int64(retention.GetRequestMetrics().AsDuration() / time.Second),
		},
		{
			name:      modelCallTableName,
			timestamp: "started_at",
			seconds:   int64(retention.GetModelCalls().AsDuration() / time.Second),
		},
	}
	for _, table := range tables {
		// 表名和时间列来自上方常量，保留秒数已经过配置校验；这里只拼接 ClickHouse DDL，
		// 不把任何请求数据或未校验的外部文本带入 SQL。
		statement := fmt.Sprintf(
			"ALTER TABLE %s MODIFY TTL %s + toIntervalSecond(%d)",
			table.name,
			table.timestamp,
			table.seconds,
		)
		if _, err := db.ExecContext(ctx, statement); err != nil {
			return fmt.Errorf("configure %s retention: %w", table.name, err)
		}
	}
	return nil
}
