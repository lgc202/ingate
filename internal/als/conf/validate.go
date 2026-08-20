// Package conf 定义并校验 ingate-als 进程配置
package conf

import (
	"errors"
	"strings"
)

// Validate 校验 ALS 进程启动所需的配置
func (c *Bootstrap) Validate() error {
	if c.GetServer() == nil || c.GetServer().GetGrpc() == nil || c.GetServer().GetHttp() == nil {
		return errors.New("server gRPC and HTTP config are required")
	}
	if strings.TrimSpace(c.GetServer().GetGrpc().GetAddr()) == "" {
		return errors.New("server gRPC address must not be empty")
	}
	if err := validateServerTLS(c.GetServer().GetGrpc().GetTls()); err != nil {
		return err
	}
	if strings.TrimSpace(c.GetServer().GetHttp().GetAddr()) == "" {
		return errors.New("server HTTP address must not be empty")
	}
	if c.GetServer().GetHttp().GetTimeout() == nil || c.GetServer().GetHttp().GetTimeout().AsDuration() <= 0 {
		return errors.New("server HTTP timeout must be greater than zero")
	}
	if c.GetServer().GetShutdownTimeout() == nil || c.GetServer().GetShutdownTimeout().AsDuration() <= 0 {
		return errors.New("server shutdown timeout must be greater than zero")
	}
	if c.GetData() == nil || c.GetData().GetKafka() == nil || c.GetData().GetDiskQueue() == nil {
		return errors.New("kafka and disk queue config are required")
	}
	if len(c.GetData().GetKafka().GetBrokers()) == 0 {
		return errors.New("kafka brokers must not be empty")
	}
	for _, broker := range c.GetData().GetKafka().GetBrokers() {
		if strings.TrimSpace(broker) == "" {
			return errors.New("kafka broker address must not be empty")
		}
	}
	if strings.TrimSpace(c.GetData().GetKafka().GetTopic()) == "" {
		return errors.New("kafka topic must not be empty")
	}
	if c.GetData().GetKafka().GetWriteTimeout() == nil || c.GetData().GetKafka().GetWriteTimeout().AsDuration() <= 0 {
		return errors.New("kafka write timeout must be greater than zero")
	}
	if c.GetData().GetKafka().GetDialTimeout() == nil || c.GetData().GetKafka().GetDialTimeout().AsDuration() <= 0 {
		return errors.New("kafka dial timeout must be greater than zero")
	}
	if c.GetData().GetKafka().GetReadinessTimeout() == nil || c.GetData().GetKafka().GetReadinessTimeout().AsDuration() <= 0 {
		return errors.New("kafka readiness timeout must be greater than zero")
	}
	if err := validateKafkaSASL(c.GetData().GetKafka().GetSasl()); err != nil {
		return err
	}
	if err := validateKafkaTLS(c.GetData().GetKafka().GetTls()); err != nil {
		return err
	}
	queue := c.GetData().GetDiskQueue()
	if strings.TrimSpace(queue.GetPath()) == "" {
		return errors.New("disk queue path must not be empty")
	}
	if queue.GetSegmentBytes() <= 0 {
		return errors.New("disk queue segment size must be greater than zero")
	}
	if queue.GetMaxBytes() <= queue.GetSegmentBytes() {
		return errors.New("disk queue max bytes must be greater than segment size")
	}
	if queue.GetReplayBatchSize() == 0 {
		return errors.New("disk queue replay batch size must be greater than zero")
	}
	if queue.GetReplayInterval() == nil || queue.GetReplayInterval().AsDuration() <= 0 {
		return errors.New("disk queue replay interval must be greater than zero")
	}
	if c.GetLogging() == nil {
		return errors.New("logging config is required")
	}
	switch strings.ToLower(c.GetLogging().GetFormat()) {
	case "json", "text":
	default:
		return errors.New("logging format must be json or text")
	}
	switch strings.ToLower(c.GetLogging().GetLevel()) {
	case "debug", "info", "warn", "error":
	default:
		return errors.New("logging level must be debug, info, warn or error")
	}
	return nil
}

func validateServerTLS(config *Server_GRPC_TLS) error {
	if config == nil || !config.GetEnabled() {
		return nil
	}
	if config.GetCertFile() == "" || config.GetKeyFile() == "" {
		return errors.New("server gRPC TLS certificate and key are required")
	}
	return nil
}

func validateKafkaSASL(config *Data_Kafka_SASL) error {
	if config == nil || strings.TrimSpace(config.GetMechanism()) == "" {
		return nil
	}
	switch strings.ToUpper(config.GetMechanism()) {
	case "PLAIN", "SCRAM-SHA-256", "SCRAM-SHA-512":
	default:
		return errors.New("kafka SASL mechanism must be PLAIN, SCRAM-SHA-256 or SCRAM-SHA-512")
	}
	if config.GetUsername() == "" || config.GetPassword() == "" {
		return errors.New("kafka SASL username and password are required")
	}
	return nil
}

func validateKafkaTLS(config *Data_Kafka_TLS) error {
	if config == nil || !config.GetEnabled() {
		return nil
	}
	if (config.GetCertFile() == "") != (config.GetKeyFile() == "") {
		return errors.New("kafka TLS certificate and key must be configured together")
	}
	return nil
}
