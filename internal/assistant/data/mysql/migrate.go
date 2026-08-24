package mysql

import (
	"context"
	"database/sql"
	"embed"
	"errors"
	"fmt"
	"io/fs"
	"time"

	drivermysql "github.com/go-sql-driver/mysql"
	"github.com/pressly/goose/v3"

	"github.com/lgc202/ingate/internal/assistant/conf"
)

const schemaMigrationTableName = "ingate_assistant_schema_migrations"

//go:embed migrations/*.sql
var migrationFiles embed.FS

// Migrate 按版本应用运维助手的 MySQL 表结构变更。
func Migrate(ctx context.Context, config *conf.Data_MySQL) (applied int, err error) {
	dsnConfig := drivermysql.Config{
		User: config.GetUsername(), Passwd: config.GetPassword(), Net: "tcp",
		Addr: config.GetAddress(), DBName: config.GetDatabase(), ParseTime: true,
		Loc: time.UTC, Timeout: config.GetDialTimeout().AsDuration(),
	}
	dsn := dsnConfig.FormatDSN()
	connection, err := sql.Open("mysql", dsn)
	if err != nil {
		return 0, fmt.Errorf("open MySQL migration connection: %w", err)
	}
	defer func() {
		if closeErr := connection.Close(); closeErr != nil {
			err = errors.Join(err, fmt.Errorf("close MySQL migration connection: %w", closeErr))
		}
	}()
	files, err := fs.Sub(migrationFiles, "migrations")
	if err != nil {
		return 0, fmt.Errorf("open embedded MySQL migrations: %w", err)
	}
	provider, err := goose.NewProvider(
		goose.DialectMySQL,
		connection,
		files,
		goose.WithTableName(schemaMigrationTableName),
		goose.WithDisableGlobalRegistry(true),
	)
	if err != nil {
		return 0, fmt.Errorf("create MySQL migration provider: %w", err)
	}
	results, err := provider.Up(ctx)
	if err != nil {
		return 0, fmt.Errorf("migrate assistant schema: %w", err)
	}
	return len(results), nil
}
