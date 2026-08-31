// Package clickhouse 实现 Analytics 的请求事实和流量统计存储边界。
package clickhouse

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/ClickHouse/clickhouse-go/v2/lib/driver"

	"github.com/lgc202/ingate/internal/analytics/conf"
	"github.com/lgc202/ingate/internal/pkg/clickhouseclient"
	"github.com/lgc202/ingate/internal/pkg/tlsconfig"
)

const (
	// 表名由内置迁移和查询代码共同维护，不允许部署配置任意替换。
	requestTableName          = "request_records"
	modelCallTableName        = "model_calls"
	minuteMetricsTableName    = "request_metrics_1m"
	minuteMetricsViewName     = "request_metrics_1m_mv"
	modelUsageTableName       = "model_usage_1m"
	modelUsageViewName        = "model_usage_1m_mv"
	requiredSchemaObjectCount = 6

	minimumClickHouseMajor = 26
	minimumClickHouseMinor = 1
)

// Store 保存请求与模型调用事实，并查询 ClickHouse 生成的流量与模型用量统计。
//
// 表名是 Analytics 的内部存储契约，不属于部署配置或用户协议。Store 同时实现
// request 的写入与查询、traffic 查询和 aiusage 查询存储边界。
type Store struct {
	connection         driver.Conn
	database           string
	requestTable       string
	modelCallTable     string
	minuteMetricsTable string
	modelUsageTable    string
	writeConcurrency   int
	writeTimeout       time.Duration
	queryTimeout       time.Duration
}

// NewStore 创建 ClickHouse 存储并确认服务版本与迁移结果兼容。
//
// 字段、引擎和视图定义只由不可变迁移管理。服务启动只验证依赖能力、
// 迁移版本和必要对象，不隐式执行 DDL；部署过程应先运行 -migrate。
func NewStore(ctx context.Context, config *conf.Data_ClickHouse) (*Store, error) {
	writeTimeout := config.GetWriteTimeout().AsDuration()
	queryTimeout := config.GetQueryTimeout().AsDuration()
	connection, err := clickhouseclient.Open(clientConfig(config))
	if err != nil {
		return nil, err
	}
	store := &Store{
		connection:         connection,
		database:           config.GetDatabase(),
		requestTable:       config.GetDatabase() + "." + requestTableName,
		modelCallTable:     config.GetDatabase() + "." + modelCallTableName,
		minuteMetricsTable: config.GetDatabase() + "." + minuteMetricsTableName,
		modelUsageTable:    config.GetDatabase() + "." + modelUsageTableName,
		// 查询与写入共用连接池，连接数允许时为控制台查询保留一个连接。
		writeConcurrency: max(1, int(config.GetMaxOpenConnections())-1),
		writeTimeout:     writeTimeout,
		queryTimeout:     queryTimeout,
	}
	checkCtx, cancel := context.WithTimeout(ctx, queryTimeout)
	defer cancel()
	if err := store.checkInstallation(checkCtx); err != nil {
		if closeErr := connection.Close(); closeErr != nil {
			return nil, errors.Join(err, fmt.Errorf("close ClickHouse after installation check failed: %w", closeErr))
		}
		return nil, err
	}
	return store, nil
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

func (s *Store) checkInstallation(ctx context.Context) error {
	var version string
	if err := s.connection.QueryRow(ctx, "SELECT version()").Scan(&version); err != nil {
		return fmt.Errorf("read ClickHouse version: %w", err)
	}
	major, minor, err := clickHouseRelease(version)
	if err != nil {
		return fmt.Errorf("parse ClickHouse version %q: %w", version, err)
	}
	// 26.1 首次保证异步插入去重会一致传递到依赖物化视图；
	// 更早版本会让 Kafka 重投重复累计流量和模型用量。
	if major < minimumClickHouseMajor ||
		major == minimumClickHouseMajor && minor < minimumClickHouseMinor {
		return fmt.Errorf(
			"ClickHouse %d.%d or newer is required; server reports %q",
			minimumClickHouseMajor,
			minimumClickHouseMinor,
			version,
		)
	}

	if err := s.checkSchemaVersion(ctx); err != nil {
		return err
	}
	return s.checkSchemaObjects(ctx)
}

func (s *Store) checkSchemaVersion(ctx context.Context) error {
	query := fmt.Sprintf(`
SELECT maxIf(version_id, is_applied = 1)
FROM %s.%s`, s.database, schemaMigrationTableName)
	var appliedVersion int64
	if err := s.connection.QueryRow(ctx, query).Scan(&appliedVersion); err != nil {
		return fmt.Errorf("read ClickHouse analytics schema version: %w", err)
	}
	switch {
	case appliedVersion < requiredSchemaVersion:
		return fmt.Errorf(
			"ClickHouse analytics schema is at version %d, but version %d is required; run ingate-analytics -migrate",
			appliedVersion,
			requiredSchemaVersion,
		)
	case appliedVersion > requiredSchemaVersion:
		return fmt.Errorf(
			"ClickHouse analytics schema version %d is newer than supported version %d",
			appliedVersion,
			requiredSchemaVersion,
		)
	default:
		return nil
	}
}

func (s *Store) checkSchemaObjects(ctx context.Context) error {
	var objectCount uint64
	if err := s.connection.QueryRow(ctx, `
SELECT count()
FROM system.tables
WHERE database = ? AND name IN (?, ?, ?, ?, ?, ?)`,
		s.database,
		requestTableName,
		modelCallTableName,
		minuteMetricsTableName,
		minuteMetricsViewName,
		modelUsageTableName,
		modelUsageViewName,
	).Scan(&objectCount); err != nil {
		return fmt.Errorf("check ClickHouse analytics schema: %w", err)
	}
	if objectCount != requiredSchemaObjectCount {
		return errors.New("ClickHouse analytics schema is incomplete; run ingate-analytics -migrate")
	}
	return nil
}

func clickHouseRelease(version string) (int, int, error) {
	components := strings.SplitN(strings.TrimSpace(version), ".", 3)
	if len(components) < 2 {
		return 0, 0, errors.New("major and minor components are required")
	}
	major, err := strconv.Atoi(components[0])
	if err != nil {
		return 0, 0, fmt.Errorf("parse major version: %w", err)
	}
	minor, err := strconv.Atoi(components[1])
	if err != nil {
		return 0, 0, fmt.Errorf("parse minor version: %w", err)
	}
	return major, minor, nil
}

// clientConfig 把 Analytics 进程配置转换为公共 ClickHouse 客户端配置。
//
// 底层连接的读取期限覆盖写入和查询两类操作中更长的一方，各方法仍通过 Context
// 施加自己的业务超时。
func clientConfig(config *conf.Data_ClickHouse) clickhouseclient.Config {
	writeTimeout := config.GetWriteTimeout().AsDuration()
	queryTimeout := config.GetQueryTimeout().AsDuration()
	return clickhouseclient.Config{
		Addresses:             config.GetAddresses(),
		Database:              config.GetDatabase(),
		Username:              config.GetUsername(),
		Password:              config.GetPassword(),
		DialTimeout:           config.GetDialTimeout().AsDuration(),
		ReadTimeout:           max(writeTimeout, queryTimeout),
		MaxOpenConnections:    int(config.GetMaxOpenConnections()),
		MaxIdleConnections:    int(config.GetMaxIdleConnections()),
		ConnectionMaxLifetime: config.GetConnectionMaxLifetime().AsDuration(),
		TLS: tlsconfig.ClientConfig{
			Enabled:         config.GetTls().GetEnabled(),
			CAFile:          config.GetTls().GetCaFile(),
			CertificateFile: config.GetTls().GetCertFile(),
			PrivateKeyFile:  config.GetTls().GetKeyFile(),
			ServerName:      config.GetTls().GetServerName(),
		},
	}
}
