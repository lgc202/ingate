// Package clickhouse 实现 Analytics 的请求事实和流量统计存储边界。
package clickhouse

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"errors"
	"fmt"
	"os"
	"time"

	clickhousego "github.com/ClickHouse/clickhouse-go/v2"
	"github.com/ClickHouse/clickhouse-go/v2/lib/driver"

	"github.com/lgc202/ingate/internal/analytics/conf"
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
	tlsConfig, err := newTLSConfig(config.GetTls())
	if err != nil {
		return nil, err
	}
	connection, err := clickhousego.Open(&clickhousego.Options{
		Addr: config.GetAddresses(),
		Auth: clickhousego.Auth{
			Database: config.GetDatabase(),
			Username: config.GetUsername(),
			Password: config.GetPassword(),
		},
		TLS:             tlsConfig,
		Compression:     &clickhousego.Compression{Method: clickhousego.CompressionLZ4},
		DialTimeout:     config.GetDialTimeout().AsDuration(),
		ReadTimeout:     config.GetWriteTimeout().AsDuration(),
		MaxOpenConns:    int(config.GetMaxOpenConnections()),
		MaxIdleConns:    int(config.GetMaxIdleConnections()),
		ConnMaxLifetime: config.GetConnectionMaxLifetime().AsDuration(),
	})
	if err != nil {
		return nil, fmt.Errorf("open ClickHouse: %w", err)
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

func newTLSConfig(config *conf.Data_ClickHouse_TLS) (*tls.Config, error) {
	if config == nil || !config.GetEnabled() {
		return nil, nil
	}
	tlsConfig := &tls.Config{
		MinVersion: tls.VersionTLS12,
		ServerName: config.GetServerName(),
	}
	if config.GetCaFile() != "" {
		pem, err := os.ReadFile(config.GetCaFile())
		if err != nil {
			return nil, fmt.Errorf("read ClickHouse CA certificate: %w", err)
		}
		roots := x509.NewCertPool()
		if !roots.AppendCertsFromPEM(pem) {
			return nil, errors.New("parse ClickHouse CA certificate")
		}
		tlsConfig.RootCAs = roots
	}
	if config.GetCertFile() != "" {
		certificate, err := tls.LoadX509KeyPair(config.GetCertFile(), config.GetKeyFile())
		if err != nil {
			return nil, fmt.Errorf("load ClickHouse client certificate: %w", err)
		}
		tlsConfig.Certificates = []tls.Certificate{certificate}
	}
	return tlsConfig, nil
}
