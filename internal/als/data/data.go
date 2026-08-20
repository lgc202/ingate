// Package data 装配 ALS 使用的 Kafka 和本地磁盘队列
package data

import (
	"log/slog"

	"github.com/google/wire"

	"github.com/lgc202/ingate/internal/als/biz"
	"github.com/lgc202/ingate/internal/als/conf"
	"github.com/lgc202/ingate/internal/als/data/diskqueue"
	"github.com/lgc202/ingate/internal/als/data/kafka"
)

// ProviderSet 绑定 ALS 业务层的主写入和磁盘队列边界
var ProviderSet = wire.NewSet(
	NewKafkaWriter,
	NewDiskQueue,
	wire.Bind(new(biz.RecordWriter), new(*kafka.Writer)),
	wire.Bind(new(biz.RecordQueue), new(*diskqueue.Queue)),
)

// NewKafkaWriter 创建 Kafka 写入端，并把连接释放交给 Wire cleanup
func NewKafkaWriter(config *conf.Data_Kafka) (*kafka.Writer, func(), error) {
	writer, err := kafka.NewWriter(config)
	if err != nil {
		return nil, nil, err
	}
	return writer, writer.Close, nil
}

// NewDiskQueue 打开本地磁盘队列，并把关闭错误统一记录到服务日志
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
			logger.Error("close disk queue failed", "error", err)
		}
	}
	return queue, cleanup, nil
}
