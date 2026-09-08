package biz

import (
	"context"

	alsv1 "github.com/lgc202/ingate/api/als/v1"
)

// PublishClass 描述 Kafka 未确认记录的重试语义。
// 非零值按批次处理优先级递增，永久失败优先于结果不确定和临时失败。
type PublishClass uint8

const (
	// PublishTemporary 表示 Kafka 明确未接收记录，稍后可以安全重试。
	PublishTemporary PublishClass = iota + 1
	// PublishUncertain 表示客户端无法确认 Kafka 是否已经接收记录，重试可能产生重复消息。
	PublishUncertain
	// PublishPermanent 表示当前配置或记录无法通过重试恢复。
	PublishPermanent
)

// PublishResult 汇总一批记录的逐条 Kafka 投递结果。
// Err 仅在全部记录都确认成功时为 nil。
type PublishResult struct {
	// Confirmed 是已经得到 ISR 确认的记录数。
	Confirmed int
	// Failed 是未得到确认的记录数。
	Failed int
	// Class 是失败记录中优先级最高的重试分类；全部成功时为零值。
	Class PublishClass
	// Err 保留优先级最高分类中的首个失败原因。
	Err error
}

// RecordPublisher 是请求记录的 Kafka 发布边界。
type RecordPublisher interface {
	Publish(context.Context, []*alsv1.RequestRecord) PublishResult
}

// String 返回适合日志和指标标签使用的稳定名称。
func (c PublishClass) String() string {
	switch c {
	case PublishTemporary:
		return "temporary"
	case PublishUncertain:
		return "uncertain"
	case PublishPermanent:
		return "permanent"
	default:
		return "none"
	}
}
