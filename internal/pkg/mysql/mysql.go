// Package mysql 初始化 MySQL 客户端，不承担账户或资源查询模型的存储操作。
package mysql

import (
	"fmt"
	"time"

	mysqldriver "github.com/go-sql-driver/mysql"
	gormmysql "gorm.io/driver/mysql"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// Open 创建 GORM 客户端，保留服务端版本查询和自动 Ping。
// 初始化失败时 GORM 关闭连接池；成功后调用方通过 db.DB() 取得连接池并负责关闭。
func Open(dsn string) (*gorm.DB, error) {
	config, err := ParseDSN(dsn)
	if err != nil {
		return nil, err
	}

	db, err := gorm.Open(
		gormmysql.Open(config.FormatDSN()),
		&gorm.Config{
			TranslateError: true,
			Logger:         logger.Default.LogMode(logger.Silent),
			NowFunc:        func() time.Time { return time.Now().UTC().Truncate(time.Microsecond) },
		},
	)
	if err != nil {
		return nil, fmt.Errorf("open MySQL: %w", err)
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
