// Package kafka 通过 franz-go 将请求记录写入 Kafka
package kafka

import (
	"context"
	"fmt"

	"github.com/twmb/franz-go/pkg/kgo"
	"google.golang.org/protobuf/proto"

	alsv1 "github.com/lgc202/ingate/api/als/v1"
	"github.com/lgc202/ingate/internal/als/conf"
	"github.com/lgc202/ingate/pkg/kafkax"
	"github.com/lgc202/ingate/pkg/tlsx"
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
	client, err := kafkax.NewClient(kafkax.Config{
		Brokers:     config.GetBrokers(),
		DialTimeout: config.GetDialTimeout().AsDuration(),
		SASL: kafkax.SASL{
			Mechanism: config.GetSasl().GetMechanism(),
			Username:  config.GetSasl().GetUsername(),
			Password:  config.GetSasl().GetPassword(),
		},
		TLS: tlsx.ClientConfig{
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
