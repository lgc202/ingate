package biz

import (
	"context"
	"sync/atomic"
)

const (
	productionReplicationFactor = 3
	productionMinISR            = 2
)

// ReliabilityMode 表示 ALS 对 Kafka Topic 的可靠性要求。
type ReliabilityMode uint8

const (
	// ReliabilityDevelopment 允许本地单 Broker Topic。
	ReliabilityDevelopment ReliabilityMode = iota + 1
	// ReliabilityProduction 要求生产 Topic 至少保留三副本、两个同步副本。
	ReliabilityProduction
)

// TopicTopology 是从 Kafka 读取的 Topic 副本事实。
type TopicTopology struct {
	// Exists 表示 Topic 已经显式创建。
	Exists bool
	// ReplicationFactor 是所有分区中的最小副本数。
	ReplicationFactor int
	// MinInSyncReplicas 是 Topic 生效的 min.insync.replicas。
	MinInSyncReplicas int
}

// TopicStatus 是最近一次成功检查得到的 Kafka Topic 契约状态。
type TopicStatus struct {
	// Checked 表示至少完成过一次能够判定 Topic 契约的检查。
	Checked bool
	// Compliant 表示 Topic 满足当前可靠性模式的副本要求。
	Compliant bool
	// ReplicationFactor 是所有分区中的最小副本数。
	ReplicationFactor int
	// MinInSyncReplicas 是 Topic 生效的 min.insync.replicas。
	MinInSyncReplicas int
}

// TopicReader 读取请求记录 Topic 的当前拓扑。
type TopicReader interface {
	ReadTopology(context.Context) (TopicTopology, error)
}

// TopicContract 保存可靠性规则及最近一次可确定的检查结果。
// 原子缓存使请求写入和就绪探针无需同步查询 Kafka；临时检查失败不会覆盖已有结果。
type TopicContract struct {
	mode   ReliabilityMode
	status atomic.Pointer[TopicStatus]
}

// NewTopicContract 创建指定可靠性模式的 Topic 契约。
func NewTopicContract(mode ReliabilityMode) *TopicContract {
	return &TopicContract{mode: mode}
}

// Update 使用最新 Kafka 拓扑替换当前契约状态。
func (c *TopicContract) Update(topology TopicTopology) TopicStatus {
	status := TopicStatus{
		Checked:           true,
		ReplicationFactor: topology.ReplicationFactor,
		MinInSyncReplicas: topology.MinInSyncReplicas,
	}
	if topology.Exists {
		status.Compliant = c.compliant(topology)
	}
	c.status.Store(new(status))
	return status
}

// Status 返回最近一次可确定的契约状态，不访问 Kafka。
func (c *TopicContract) Status() TopicStatus {
	status := c.status.Load()
	if status == nil {
		return TopicStatus{}
	}
	return *status
}

func (c *TopicContract) compliant(topology TopicTopology) bool {
	// min.insync.replicas 高于副本数时，任何 acks=all 写入都不可能成功；
	// 即使开发模式允许单副本，也不能把这种自相矛盾的拓扑视为可写。
	if topology.MinInSyncReplicas > topology.ReplicationFactor {
		return false
	}
	if c.mode == ReliabilityProduction {
		return topology.ReplicationFactor >= productionReplicationFactor &&
			topology.MinInSyncReplicas >= productionMinISR
	}
	return topology.ReplicationFactor > 0 && topology.MinInSyncReplicas > 0
}
