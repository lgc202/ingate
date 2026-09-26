// Package mysql 保存 Admin API 的管理员账户。
package mysql

import (
	"database/sql"
	"fmt"
	"time"

	mysqldriver "github.com/go-sql-driver/mysql"
	gormmysql "gorm.io/driver/mysql"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// Store 持有管理员账户使用的 MySQL 连接池。
// 使用完毕后调用 Close 释放连接池。
type Store struct {
	db    *gorm.DB
	sqlDB *sql.DB
}

// Open 打开 MySQL 连接并创建 Store。
func Open(dsn string) (*Store, error) {
	driverConfig, err := mysqldriver.ParseDSN(dsn)
	if err != nil {
		return nil, fmt.Errorf("parse MySQL DSN: %w", err)
	}
	driverConfig.ParseTime = true
	driverConfig.Loc = time.UTC
	driverConfig.MultiStatements = false

	db, err := gorm.Open(
		gormmysql.Open(driverConfig.FormatDSN()),
		&gorm.Config{
			TranslateError: true,
			Logger:         logger.Default.LogMode(logger.Silent),
			NowFunc:        func() time.Time { return time.Now().UTC().Truncate(time.Microsecond) },
		},
	)
	if err != nil {
		return nil, fmt.Errorf("open MySQL: %w", err)
	}

	sqlDB, err := db.DB()
	if err != nil {
		return nil, fmt.Errorf("get MySQL connection pool: %w", err)
	}

	return &Store{db: db, sqlDB: sqlDB}, nil
}

// Close 关闭底层 MySQL 连接池。
func (s *Store) Close() error {
	return s.sqlDB.Close()
}
