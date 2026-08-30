// Package clickhouseclient 提供 Ingate 组件共用的 ClickHouse 客户端连接能力。
package clickhouseclient

import (
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/ClickHouse/clickhouse-go/v2"
	"github.com/ClickHouse/clickhouse-go/v2/lib/driver"

	"github.com/lgc202/ingate/internal/pkg/tlsconfig"
)

// Config 定义 ClickHouse native 协议连接和连接池参数。
type Config struct {
	Addresses             []string
	Database              string
	Username              string
	Password              string
	DialTimeout           time.Duration
	ReadTimeout           time.Duration
	MaxOpenConnections    int
	MaxIdleConnections    int
	ConnectionMaxLifetime time.Duration
	TLS                   tlsconfig.ClientConfig
}

// Open 创建启用 LZ4 压缩的 ClickHouse 原生客户端连接池。
func Open(config Config) (driver.Conn, error) {
	options, err := options(config)
	if err != nil {
		return nil, err
	}
	client, err := clickhouse.Open(options)
	if err != nil {
		return nil, fmt.Errorf("open ClickHouse: %w", err)
	}
	return client, nil
}

// OpenDB 创建使用 database/sql 接口的 ClickHouse 连接池
// 迁移工具使用标准接口，业务读写继续使用支持原生批处理的 Open。
func OpenDB(config Config) (*sql.DB, error) {
	options, err := options(config)
	if err != nil {
		return nil, err
	}
	return clickhouse.OpenDB(options), nil
}

func options(config Config) (*clickhouse.Options, error) {
	if len(config.Addresses) == 0 {
		return nil, errors.New("ClickHouse addresses must not be empty")
	}
	if config.DialTimeout <= 0 || config.ReadTimeout <= 0 {
		return nil, errors.New("ClickHouse dial and read timeout must be greater than zero")
	}
	if config.MaxOpenConnections <= 0 || config.MaxIdleConnections < 0 ||
		config.MaxIdleConnections > config.MaxOpenConnections {
		return nil, errors.New("ClickHouse connection pool limits are invalid")
	}
	if config.ConnectionMaxLifetime <= 0 {
		return nil, errors.New("ClickHouse connection max lifetime must be greater than zero")
	}

	tlsConfig, err := tlsconfig.NewClient(config.TLS)
	if err != nil {
		return nil, fmt.Errorf("configure ClickHouse TLS: %w", err)
	}
	return &clickhouse.Options{
		Addr: config.Addresses,
		Auth: clickhouse.Auth{
			Database: config.Database,
			Username: config.Username,
			Password: config.Password,
		},
		TLS:             tlsConfig,
		Compression:     &clickhouse.Compression{Method: clickhouse.CompressionLZ4},
		DialTimeout:     config.DialTimeout,
		ReadTimeout:     config.ReadTimeout,
		MaxOpenConns:    config.MaxOpenConnections,
		MaxIdleConns:    config.MaxIdleConnections,
		ConnMaxLifetime: config.ConnectionMaxLifetime,
	}, nil
}
