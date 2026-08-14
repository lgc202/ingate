// Package clickhouse 实现 Analytics 的请求事实和流量统计存储边界。
package clickhouse

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/ClickHouse/clickhouse-go/v2/lib/driver"

	"github.com/lgc202/ingate/internal/analytics/conf"
	"github.com/lgc202/ingate/pkg/clickhousex"
	"github.com/lgc202/ingate/pkg/tlsx"
)

var errNotImplemented = errors.New("ClickHouse analytics storage is not implemented")

// Store 保存请求事实并查询 ClickHouse 生成的流量统计。
//
// 连接和表名在这里收口，biz 不感知 ClickHouse client、SQL 或表结构。
type Store struct {
	connection         driver.Conn
	requestTable       string
	minuteMetricsTable string
	writeTimeout       time.Duration
	queryTimeout       time.Duration
}

// NewStore 创建 ClickHouse 存储。
func NewStore(config *conf.Data_ClickHouse) (*Store, error) {
	connection, err := clickhousex.NewClient(clickhousex.Config{
		Addresses:             config.GetAddresses(),
		Database:              config.GetDatabase(),
		Username:              config.GetUsername(),
		Password:              config.GetPassword(),
		DialTimeout:           config.GetDialTimeout().AsDuration(),
		ReadTimeout:           config.GetWriteTimeout().AsDuration(),
		MaxOpenConnections:    int(config.GetMaxOpenConnections()),
		MaxIdleConnections:    int(config.GetMaxIdleConnections()),
		ConnectionMaxLifetime: config.GetConnectionMaxLifetime().AsDuration(),
		TLS: tlsx.ClientConfig{
			Enabled:         config.GetTls().GetEnabled(),
			CAFile:          config.GetTls().GetCaFile(),
			CertificateFile: config.GetTls().GetCertFile(),
			PrivateKeyFile:  config.GetTls().GetKeyFile(),
			ServerName:      config.GetTls().GetServerName(),
		},
	})
	if err != nil {
		return nil, err
	}
	return &Store{
		connection:         connection,
		requestTable:       config.GetDatabase() + "." + config.GetRequestTable(),
		minuteMetricsTable: config.GetDatabase() + "." + config.GetMinuteMetricsTable(),
		writeTimeout:       config.GetWriteTimeout().AsDuration(),
		queryTimeout:       config.GetQueryTimeout().AsDuration(),
	}, nil
}

// Ping 验证至少一个 ClickHouse 节点可以完成连接和鉴权。
func (s *Store) Ping(ctx context.Context) error {
	if err := s.connection.Ping(ctx); err != nil {
		return fmt.Errorf("ping ClickHouse: %w", err)
	}
	return nil
}

// Close 释放 ClickHouse 连接池。
func (s *Store) Close() error {
	if err := s.connection.Close(); err != nil {
		return fmt.Errorf("close ClickHouse: %w", err)
	}
	return nil
}
