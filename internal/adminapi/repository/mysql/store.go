// Package mysql 保存管理员账户和声明式资源的查询模型。
package mysql

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	mysqldriver "github.com/go-sql-driver/mysql"
	gormmysql "gorm.io/driver/mysql"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// ErrNotFound 表示查询模型中不存在指定资源。
var ErrNotFound = errors.New("resource not found")

// Store 持有管理员账户与资源查询模型使用的 MySQL 连接池。
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

	store, err := New(db)
	if err != nil {
		return nil, err
	}

	return store, nil
}

// New 使用已有的 GORM 连接创建 Store。
func New(db *gorm.DB) (*Store, error) {
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

// readOnlyTransaction 让相关查询读取同一个可重复读视图。
func (s *Store) readOnlyTransaction(ctx context.Context, query func(*gorm.DB) error) error {
	return s.db.WithContext(ctx).Transaction(query, &sql.TxOptions{
		ReadOnly:  true,
		Isolation: sql.LevelRepeatableRead,
	})
}

func page(db *gorm.DB, size, number int32) *gorm.DB {
	return db.Offset(int(int64(number-1) * int64(size))).Limit(int(size))
}
