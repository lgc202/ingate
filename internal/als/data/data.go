// Package data 装配 ALS 使用的 Kafka 和本地磁盘队列。
package data

import (
	"log/slog"

	"github.com/google/wire"

	"github.com/lgc202/ingate/internal/als/biz"
	"github.com/lgc202/ingate/internal/als/conf"
	"github.com/lgc202/ingate/internal/als/data/diskqueue"
	"github.com/lgc202/ingate/internal/als/data/kafka"
)

// ProviderSet 绑定 ALS 业务层的主写入和磁盘队列边界。
var ProviderSet = wire.NewSet(
	NewKafkaClient,
	NewDiskQueue,
	wire.Bind(new(biz.RecordPublisher), new(*kafka.Client)),
	wire.Bind(new(biz.TopicInspector), new(*kafka.Client)),
	wire.Bind(new(biz.RecordQueue), new(*diskqueue.Queue)),
)

// NewKafkaClient 创建 Kafka 客户端，并把连接释放交给 Wire cleanup。
func NewKafkaClient(config *conf.Data_Kafka) (*kafka.Client, func(), error) {
	client, err := kafka.NewClient(config)
	if err != nil {
		return nil, nil, err
	}

	return client, client.Close, nil
}

// NewDiskQueue 打开本地磁盘队列，并把关闭错误统一记录到服务日志。
func NewDiskQueue(
	config *conf.Data_DiskQueue,
	logger *slog.Logger,
) (*diskqueue.Queue, func(), error) {
	queue, err := diskqueue.NewQueue(config)
	if err != nil {
		return nil, nil, err
	}
	cleanup := func() {
		if err := queue.Close(); err != nil {
			logger.Error("close disk queue failed", "err", err)
		}
	}
	return queue, cleanup, nil
}
