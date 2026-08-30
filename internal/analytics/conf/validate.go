// Package conf 定义并校验 ingate-analytics 进程配置。
package conf

import (
	"errors"
	"regexp"
	"strings"
	"time"

	"github.com/lgc202/ingate/internal/pkg/appconfig"
	"github.com/lgc202/ingate/internal/pkg/kafkaclient"
)

var clickHouseIdentifier = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]*$`)

// Validate 校验 Analytics 进程启动所需的配置。
func (c *Bootstrap) Validate() error {
	server := c.GetServer()
	if server == nil || server.GetHttp() == nil || server.GetGrpc() == nil {
		return errors.New("server HTTP and gRPC config are required")
	}
	httpServer := server.GetHttp()
	if strings.TrimSpace(httpServer.GetAddr()) == "" {
		return errors.New("server HTTP address must not be empty")
	}
	if httpServer.GetTimeout() == nil || httpServer.GetTimeout().AsDuration() <= 0 {
		return errors.New("server HTTP timeout must be greater than zero")
	}
	grpcServer := server.GetGrpc()
	if strings.TrimSpace(grpcServer.GetAddr()) == "" {
		return errors.New("server gRPC address must not be empty")
	}
	if grpcServer.GetTimeout() == nil || grpcServer.GetTimeout().AsDuration() <= 0 {
		return errors.New("server gRPC timeout must be greater than zero")
	}
	grpcTLS := grpcServer.GetTls()
	if grpcTLS.GetEnabled() && (grpcTLS.GetCertFile() == "" || grpcTLS.GetKeyFile() == "") {
		return errors.New("server gRPC TLS certificate and key are required")
	}
	if server.GetShutdownTimeout() == nil || server.GetShutdownTimeout().AsDuration() <= 0 {
		return errors.New("server shutdown timeout must be greater than zero")
	}
	data := c.GetData()
	if data == nil || data.GetKafka() == nil || data.GetClickHouse() == nil {
		return errors.New("kafka and ClickHouse config are required")
	}
	if err := validateKafka(data.GetKafka()); err != nil {
		return err
	}
	if err := validateClickHouse(data.GetClickHouse()); err != nil {
		return err
	}
	logging := c.GetLogging()
	if logging == nil {
		return errors.New("logging config is required")
	}
	return appconfig.ValidateLogging(logging)
}

func validateKafka(config *Data_Kafka) error {
	if len(config.GetBrokers()) == 0 {
		return errors.New("kafka brokers must not be empty")
	}
	for _, broker := range config.GetBrokers() {
		if strings.TrimSpace(broker) == "" {
			return errors.New("kafka broker address must not be empty")
		}
	}
	if strings.TrimSpace(config.GetTopic()) == "" {
		return errors.New("kafka topic must not be empty")
	}
	if strings.TrimSpace(config.GetGroupId()) == "" {
		return errors.New("kafka consumer group ID must not be empty")
	}
	if strings.TrimSpace(config.GetClientId()) == "" {
		return errors.New("kafka client ID must not be empty")
	}
	if config.GetBatchMaxRecords() == 0 {
		return errors.New("kafka batch max records must be greater than zero")
	}
	if config.GetFetchMinBytes() <= 0 {
		return errors.New("kafka fetch min bytes must be greater than zero")
	}
	if config.GetFetchMaxWait() == nil || config.GetFetchMaxWait().AsDuration() <= 0 {
		return errors.New("kafka fetch max wait must be greater than zero")
	}
	if config.GetDialTimeout() == nil || config.GetDialTimeout().AsDuration() <= 0 {
		return errors.New("kafka dial timeout must be greater than zero")
	}
	if err := kafkaSASL(config.GetSasl()).Validate(); err != nil {
		return err
	}
	return validateTLS(
		"Kafka",
		config.GetTls().GetEnabled(),
		config.GetTls().GetCertFile(),
		config.GetTls().GetKeyFile(),
	)
}

func validateClickHouse(config *Data_ClickHouse) error {
	if len(config.GetAddresses()) == 0 {
		return errors.New("ClickHouse addresses must not be empty")
	}
	for _, address := range config.GetAddresses() {
		if strings.TrimSpace(address) == "" {
			return errors.New("ClickHouse address must not be empty")
		}
	}
	if !clickHouseIdentifier.MatchString(config.GetDatabase()) {
		return errors.New("ClickHouse database must be a valid identifier")
	}
	if config.GetDialTimeout() == nil || config.GetDialTimeout().AsDuration() <= 0 {
		return errors.New("ClickHouse dial timeout must be greater than zero")
	}
	if config.GetWriteTimeout() == nil || config.GetWriteTimeout().AsDuration() <= 0 {
		return errors.New("ClickHouse write timeout must be greater than zero")
	}
	if config.GetQueryTimeout() == nil || config.GetQueryTimeout().AsDuration() <= 0 {
		return errors.New("ClickHouse query timeout must be greater than zero")
	}
	if config.GetMaxOpenConnections() <= 0 {
		return errors.New("ClickHouse max open connections must be greater than zero")
	}
	if config.GetMaxIdleConnections() > config.GetMaxOpenConnections() {
		return errors.New("ClickHouse max idle connections must not exceed max open connections")
	}
	if config.GetConnectionMaxLifetime() == nil || config.GetConnectionMaxLifetime().AsDuration() <= 0 {
		return errors.New("ClickHouse connection max lifetime must be greater than zero")
	}
	retention := config.GetRetention()
	if retention == nil {
		return errors.New("ClickHouse retention config is required")
	}
	if retention.GetRequestRecords() == nil || retention.GetRequestRecords().AsDuration() < time.Second {
		return errors.New("ClickHouse request record retention must be at least one second")
	}
	if retention.GetRequestMetrics() == nil || retention.GetRequestMetrics().AsDuration() < time.Second {
		return errors.New("ClickHouse request metric retention must be at least one second")
	}
	if retention.GetModelCalls() == nil || retention.GetModelCalls().AsDuration() < time.Second {
		return errors.New("ClickHouse model call retention must be at least one second")
	}
	return validateTLS(
		"ClickHouse",
		config.GetTls().GetEnabled(),
		config.GetTls().GetCertFile(),
		config.GetTls().GetKeyFile(),
	)
}

func kafkaSASL(config *Data_Kafka_SASL) kafkaclient.SASL {
	return kafkaclient.SASL{
		Mechanism: config.GetMechanism(),
		Username:  config.GetUsername(),
		Password:  config.GetPassword(),
	}
}

func validateTLS(system string, enabled bool, certificateFile, keyFile string) error {
	if !enabled {
		return nil
	}
	if (certificateFile == "") != (keyFile == "") {
		return errors.New(system + " TLS certificate and key must be configured together")
	}
	return nil
}
