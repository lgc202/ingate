package clickhouse

import (
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/ClickHouse/clickhouse-go/v2"
	"github.com/ClickHouse/clickhouse-go/v2/lib/driver"

	"github.com/lgc202/ingate/internal/pkg/tlsconfig"
)

type connectionConfig struct {
	addresses       []string
	database        string
	username        string
	password        string
	dialTimeout     time.Duration
	readTimeout     time.Duration
	maxOpenConns    int
	maxIdleConns    int
	connMaxLifetime time.Duration
	tls             tlsconfig.ClientConfig
}

func openConnection(config connectionConfig) (driver.Conn, error) {
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

// openDatabase 创建使用 database/sql 接口的 ClickHouse 连接池。
// 迁移工具使用标准接口，业务读写继续使用支持原生批处理的 Open。
func openDatabase(config connectionConfig) (*sql.DB, error) {
	options, err := options(config)
	if err != nil {
		return nil, err
	}
	return clickhouse.OpenDB(options), nil
}

func options(config connectionConfig) (*clickhouse.Options, error) {
	if len(config.addresses) == 0 {
		return nil, errors.New("ClickHouse addresses must not be empty")
	}
	if config.dialTimeout <= 0 || config.readTimeout <= 0 {
		return nil, errors.New("ClickHouse dial and read timeout must be greater than zero")
	}
	if config.maxOpenConns <= 0 || config.maxIdleConns < 0 ||
		config.maxIdleConns > config.maxOpenConns {
		return nil, errors.New("ClickHouse connection pool limits are invalid")
	}
	if config.connMaxLifetime <= 0 {
		return nil, errors.New("ClickHouse connection max lifetime must be greater than zero")
	}

	tlsConfig, err := tlsconfig.NewClient(config.tls)
	if err != nil {
		return nil, fmt.Errorf("configure ClickHouse TLS: %w", err)
	}
	return &clickhouse.Options{
		Addr: config.addresses,
		Auth: clickhouse.Auth{
			Database: config.database,
			Username: config.username,
			Password: config.password,
		},
		TLS:             tlsConfig,
		Compression:     &clickhouse.Compression{Method: clickhouse.CompressionLZ4},
		DialTimeout:     config.dialTimeout,
		ReadTimeout:     config.readTimeout,
		MaxOpenConns:    config.maxOpenConns,
		MaxIdleConns:    config.maxIdleConns,
		ConnMaxLifetime: config.connMaxLifetime,
	}, nil
}
