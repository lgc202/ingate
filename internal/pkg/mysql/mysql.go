// Package mysql 初始化 MySQL 客户端和连接池，不承担业务存储操作。
package mysql

import (
	"database/sql"
	"errors"
	"fmt"
	"time"

	mysqldriver "github.com/go-sql-driver/mysql"
	gormmysql "gorm.io/driver/mysql"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// Config 配置 MySQL 连接和连接池。
type Config struct {
	DSN                   string
	MaxOpenConnections    int
	MaxIdleConnections    int
	ConnectionMaxLifetime time.Duration
	ConnectionMaxIdleTime time.Duration
}

// Open 配置 MySQL 客户端和连接池，但不执行网络探测或数据库迁移。
// 调用方通过 db.DB() 取得连接池，负责使用带超时的 Context 探测并在停止时关闭。
func Open(config Config) (*gorm.DB, error) {
	if err := config.validate(); err != nil {
		return nil, err
	}

	driverConfig, err := ParseDSN(config.DSN)
	if err != nil {
		return nil, err
	}

	connector, err := mysqldriver.NewConnector(driverConfig)
	if err != nil {
		return nil, fmt.Errorf("create MySQL connector: %w", err)
	}

	sqlDB := sql.OpenDB(connector)
	sqlDB.SetMaxOpenConns(config.MaxOpenConnections)
	sqlDB.SetMaxIdleConns(config.MaxIdleConnections)
	sqlDB.SetConnMaxLifetime(config.ConnectionMaxLifetime)
	sqlDB.SetConnMaxIdleTime(config.ConnectionMaxIdleTime)

	db, err := gorm.Open(
		gormmysql.New(gormmysql.Config{
			Conn: sqlDB,
			// 本包不执行数据库迁移；连接验证由调用方显式执行，无需初始化时探测版本。
			SkipInitializeWithVersion: true,
		}),
		&gorm.Config{
			DisableAutomaticPing: true,
			// 由调用方处理错误，禁用带参数的 SQL 日志，避免泄露敏感数据。
			Logger: logger.Default.LogMode(logger.Silent),
			NowFunc: func() time.Time {
				return time.Now().UTC().Truncate(time.Microsecond)
			},
			TranslateError: true,
		},
	)
	if err != nil {
		return nil, closeAfterError(sqlDB, fmt.Errorf("open GORM database: %w", err))
	}

	return db, nil
}

// ParseDSN 解析 DSN，统一启用时间解析、使用 UTC 并禁止多语句。
// 返回独立的驱动原生配置，不建立连接；其他选项保留驱动的解析结果。
func ParseDSN(dsn string) (*mysqldriver.Config, error) {
	config, err := mysqldriver.ParseDSN(dsn)
	if err != nil {
		return nil, fmt.Errorf("parse MySQL DSN: %w", err)
	}

	config.ParseTime = true
	config.Loc = time.UTC
	config.MultiStatements = false

	return config, nil
}

func (c Config) validate() error {
	switch {
	case c.DSN == "":
		return errors.New("MySQL DSN is required")
	case c.MaxOpenConnections <= 0:
		return errors.New("MySQL max open connections must be positive")
	case c.MaxIdleConnections < 0:
		return errors.New("MySQL max idle connections must not be negative")
	case c.MaxIdleConnections > c.MaxOpenConnections:
		return errors.New("MySQL max idle connections must not exceed max open connections")
	case c.ConnectionMaxLifetime <= 0:
		return errors.New("MySQL connection max lifetime must be positive")
	case c.ConnectionMaxIdleTime <= 0:
		return errors.New("MySQL connection max idle time must be positive")
	default:
		return nil
	}
}

func closeAfterError(db *sql.DB, operationErr error) error {
	if err := db.Close(); err != nil {
		return errors.Join(operationErr, fmt.Errorf("close MySQL after failure: %w", err))
	}

	return operationErr
}
