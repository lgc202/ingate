// Package kafka 通过 franz-go 发布请求记录并检查目标 Topic。
package kafka

import (
	"github.com/twmb/franz-go/pkg/kgo"
	oteltrace "go.opentelemetry.io/otel/trace"

	"github.com/lgc202/ingate/internal/als/conf"
	alsmetrics "github.com/lgc202/ingate/internal/als/metrics"
	"github.com/lgc202/ingate/internal/pkg/kafkaclient"
	"github.com/lgc202/ingate/internal/pkg/tlsconfig"
)

// Client 共享 Kafka 连接，同时提供记录发布和 Topic 检查能力。
type Client struct {
	kafka  *kgo.Client
	topic  string
	events *alsmetrics.EventCollector
	tracer oteltrace.Tracer
}

// NewClient 创建使用幂等 Producer 和 all ISR 确认的 Kafka 客户端。
//
// franz-go 默认启用幂等 Producer。这里不覆盖其重试和并发默认值，
// 避免破坏 Producer ID 与序列号所保证的批内幂等性。
func NewClient(
	config *conf.Data_Kafka,
	events *alsmetrics.EventCollector,
	tracer oteltrace.Tracer,
) (*Client, error) {
	client, err := kafkaclient.New(kafkaclient.Config{
		Brokers:     config.GetBrokers(),
		DialTimeout: config.GetDialTimeout().AsDuration(),
		SASL: kafkaclient.SASL{
			Mechanism: config.GetSasl().GetMechanism(),
			Username:  config.GetSasl().GetUsername(),
			Password:  config.GetSasl().GetPassword(),
		},
		TLS: tlsconfig.ClientConfig{
			Enabled:         config.GetTls().GetEnabled(),
			CAFile:          config.GetTls().GetCaFile(),
			CertificateFile: config.GetTls().GetCertFile(),
			PrivateKeyFile:  config.GetTls().GetKeyFile(),
			ServerName:      config.GetTls().GetServerName(),
		},
	},
		kgo.DefaultProduceTopic(config.GetTopic()),
		kgo.RequiredAcks(kgo.AllISRAcks()),
		kgo.ProducerBatchCompression(kgo.ZstdCompression()),
		kgo.RecordDeliveryTimeout(config.GetWriteTimeout().AsDuration()),
	)
	if err != nil {
		return nil, err
	}

	return &Client{
		kafka:  client,
		topic:  config.GetTopic(),
		events: events,
		tracer: tracer,
	}, nil
}

// Close 等待客户端结束内部工作并释放连接。
func (c *Client) Close() {
	c.kafka.Close()
}
