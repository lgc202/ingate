// Package migration 执行 Admin API 的版本化 MySQL Schema 迁移。
package migration

import (
	"context"
	"database/sql"
	"embed"
	"errors"
	"fmt"
	"time"

	mysqldriver "github.com/go-sql-driver/mysql"
	"github.com/pressly/goose/v3"
)

// migrationFiles 与迁移程序编译在一起，避免部署时单独分发 SQL 文件。
//
//go:embed *.sql
var migrationFiles embed.FS

// Up 执行尚未应用的 Admin API 数据库迁移。
func Up(ctx context.Context, dsn string) (err error) {
	driverConfig, err := mysqldriver.ParseDSN(dsn)
	if err != nil {
		return fmt.Errorf("parse MySQL DSN: %w", err)
	}
	driverConfig.ParseTime = true
	driverConfig.Loc = time.UTC
	driverConfig.MultiStatements = false

	connector, err := mysqldriver.NewConnector(driverConfig)
	if err != nil {
		return fmt.Errorf("create MySQL connector: %w", err)
	}

	db := sql.OpenDB(connector)
	defer func() {
		if closeErr := db.Close(); closeErr != nil {
			err = errors.Join(err, fmt.Errorf("close MySQL: %w", closeErr))
		}
	}()

	provider, err := goose.NewProvider(
		goose.DialectMySQL,
		db,
		migrationFiles,
		goose.WithDisableGlobalRegistry(true),
		goose.WithLogger(goose.NopLogger()),
	)
	if err != nil {
		return fmt.Errorf("create migration provider: %w", err)
	}
	if err := provider.Ping(ctx); err != nil {
		return fmt.Errorf("ping MySQL: %w", err)
	}
	if _, err := provider.Up(ctx); err != nil {
		return fmt.Errorf("apply migrations: %w", err)
	}

	return nil
}
