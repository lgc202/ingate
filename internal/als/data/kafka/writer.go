// Package kafka 通过 franz-go 将请求记录写入 Kafka
package kafka

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"os"
	"strings"

	"github.com/twmb/franz-go/pkg/kgo"
	"github.com/twmb/franz-go/pkg/sasl"
	"github.com/twmb/franz-go/pkg/sasl/plain"
	"github.com/twmb/franz-go/pkg/sasl/scram"
	"google.golang.org/protobuf/proto"

	alsv1 "github.com/lgc202/ingate/api/als/v1"
	"github.com/lgc202/ingate/internal/als/conf"
)

const (
	protobufContentType = "application/x-protobuf"
	requestRecordType   = "ingate.als.v1.RequestRecord"
)

// Writer 将 protobuf 请求记录发布到 Kafka
type Writer struct {
	client *kgo.Client
}

// NewWriter 创建具备幂等生产语义的 Kafka 写入端
//
// franz-go 默认启用幂等 producer；AllISRAcks 使成功返回代表当前 ISR 已确认，
// ALS 才能据此安全删除本地 WAL 中对应的积压记录
func NewWriter(config *conf.Data_Kafka) (*Writer, error) {
	options := []kgo.Opt{
		kgo.SeedBrokers(config.GetBrokers()...),
		kgo.DefaultProduceTopic(config.GetTopic()),
		kgo.RequiredAcks(kgo.AllISRAcks()),
		kgo.ProducerBatchCompression(kgo.ZstdCompression()),
		kgo.RecordDeliveryTimeout(config.GetWriteTimeout().AsDuration()),
		kgo.DialTimeout(config.GetDialTimeout().AsDuration()),
	}
	mechanism, err := saslMechanism(config.GetSasl())
	if err != nil {
		return nil, err
	}
	if mechanism != nil {
		options = append(options, kgo.SASL(mechanism))
	}
	tlsConfig, err := kafkaTLSConfig(config.GetTls())
	if err != nil {
		return nil, err
	}
	if tlsConfig != nil {
		options = append(options, kgo.DialTLSConfig(tlsConfig))
	}
	client, err := kgo.NewClient(options...)
	if err != nil {
		return nil, fmt.Errorf("create Kafka client: %w", err)
	}
	return &Writer{client: client}, nil
}

// Ping 验证至少一个 Kafka broker 当前可以完成连接和鉴权
func (w *Writer) Ping(ctx context.Context) error {
	if err := w.client.Ping(ctx); err != nil {
		return fmt.Errorf("ping Kafka: %w", err)
	}
	return nil
}

// Write 同步等待整批记录得到 Kafka 的最终投递结果
//
// 一批消息可能部分成功后返回错误，调用方会把整批写入 WAL，因此消费者仍需按 RequestRecord.id 去重
func (w *Writer) Write(ctx context.Context, records []*alsv1.RequestRecord) error {
	messages := make([]*kgo.Record, 0, len(records))
	for _, record := range records {
		value, err := proto.Marshal(record)
		if err != nil {
			return fmt.Errorf("marshal request record: %w", err)
		}
		messages = append(messages, &kgo.Record{
			Key:   []byte(record.GetId()),
			Value: value,
			Headers: []kgo.RecordHeader{
				{Key: "content-type", Value: []byte(protobufContentType)},
				{Key: "message-type", Value: []byte(requestRecordType)},
			},
		})
	}
	if err := w.client.ProduceSync(ctx, messages...).FirstErr(); err != nil {
		return fmt.Errorf("produce request records: %w", err)
	}
	return nil
}

// Close 等待客户端结束内部工作并释放连接
func (w *Writer) Close() {
	w.client.Close()
}

func saslMechanism(config *conf.Data_Kafka_SASL) (sasl.Mechanism, error) {
	if config == nil || config.GetMechanism() == "" {
		return nil, nil
	}
	switch strings.ToUpper(config.GetMechanism()) {
	case "PLAIN":
		return plain.Auth{User: config.GetUsername(), Pass: config.GetPassword()}.AsMechanism(), nil
	case "SCRAM-SHA-256":
		return scram.Auth{User: config.GetUsername(), Pass: config.GetPassword()}.AsSha256Mechanism(), nil
	case "SCRAM-SHA-512":
		return scram.Auth{User: config.GetUsername(), Pass: config.GetPassword()}.AsSha512Mechanism(), nil
	default:
		return nil, fmt.Errorf("unsupported Kafka SASL mechanism %q", config.GetMechanism())
	}
}

func kafkaTLSConfig(config *conf.Data_Kafka_TLS) (*tls.Config, error) {
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
			return nil, fmt.Errorf("read Kafka CA certificate: %w", err)
		}
		roots := x509.NewCertPool()
		if !roots.AppendCertsFromPEM(pem) {
			return nil, fmt.Errorf("parse Kafka CA certificate")
		}
		tlsConfig.RootCAs = roots
	}
	if config.GetCertFile() != "" {
		certificate, err := tls.LoadX509KeyPair(config.GetCertFile(), config.GetKeyFile())
		if err != nil {
			return nil, fmt.Errorf("load Kafka client certificate: %w", err)
		}
		tlsConfig.Certificates = []tls.Certificate{certificate}
	}
	return tlsConfig, nil
}
