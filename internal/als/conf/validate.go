// Package conf 定义并校验 ingate-als 进程配置。
package conf

import (
	"errors"
	"strings"

	"github.com/lgc202/ingate/internal/pkg/appconfig"
	"github.com/lgc202/ingate/internal/pkg/kafkaclient"
)

// Validate 校验 ALS 进程启动所需的配置。
func (c *Bootstrap) Validate() error {
	server := c.GetServer()
	if server == nil || server.GetGrpc() == nil || server.GetHttp() == nil {
		return errors.New("server gRPC and HTTP config are required")
	}
	grpcServer := server.GetGrpc()
	if strings.TrimSpace(grpcServer.GetAddr()) == "" {
		return errors.New("server gRPC address must not be empty")
	}
	if err := validateServerTLS(grpcServer.GetTls()); err != nil {
		return err
	}
	httpServer := server.GetHttp()
	if strings.TrimSpace(httpServer.GetAddr()) == "" {
		return errors.New("server HTTP address must not be empty")
	}
	if httpServer.GetTimeout() == nil || httpServer.GetTimeout().AsDuration() <= 0 {
		return errors.New("server HTTP timeout must be greater than zero")
	}
	if server.GetShutdownTimeout() == nil || server.GetShutdownTimeout().AsDuration() <= 0 {
		return errors.New("server shutdown timeout must be greater than zero")
	}
	data := c.GetData()
	if data == nil || data.GetKafka() == nil || data.GetDiskQueue() == nil {
		return errors.New("kafka and disk queue config are required")
	}
	kafka := data.GetKafka()
	if len(kafka.GetBrokers()) == 0 {
		return errors.New("kafka brokers must not be empty")
	}
	for _, broker := range kafka.GetBrokers() {
		if strings.TrimSpace(broker) == "" {
			return errors.New("kafka broker address must not be empty")
		}
	}
	if strings.TrimSpace(kafka.GetTopic()) == "" {
		return errors.New("kafka topic must not be empty")
	}
	if kafka.GetWriteTimeout() == nil || kafka.GetWriteTimeout().AsDuration() <= 0 {
		return errors.New("kafka write timeout must be greater than zero")
	}
	if kafka.GetDialTimeout() == nil || kafka.GetDialTimeout().AsDuration() <= 0 {
		return errors.New("kafka dial timeout must be greater than zero")
	}
	if kafka.GetReadinessTimeout() == nil || kafka.GetReadinessTimeout().AsDuration() <= 0 {
		return errors.New("kafka readiness timeout must be greater than zero")
	}
	if err := kafkaSASL(kafka.GetSasl()).Validate(); err != nil {
		return err
	}
	if err := validateKafkaTLS(kafka.GetTls()); err != nil {
		return err
	}
	queue := data.GetDiskQueue()
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
	logging := c.GetLogging()
	if logging == nil {
		return errors.New("logging config is required")
	}
	return appconfig.ValidateLogging(logging)
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

func kafkaSASL(config *Data_Kafka_SASL) kafkaclient.SASL {
	return kafkaclient.SASL{
		Mechanism: config.GetMechanism(),
		Username:  config.GetUsername(),
		Password:  config.GetPassword(),
	}
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
