---
title: Kafka 可靠写入
description: 从 Kafka message、ISR 确认到错误分类解释 ALS 的写入语义
---

ALS 把每条 `RequestRecord` 写成一条独立 Kafka message。Kafka 是正常路径上的持久化边界；只有收到 Producer 成功结果后，Recorder 才把这条记录计入 `kafka_accepted`。

## Kafka message 的内容

`internal/als/data/kafka/publisher.go` 负责消息编码：

```go
messages = append(messages, &kgo.Record{
	Key:     []byte(record.GetId()),
	Value:   value,
	Headers: headers,
})
```

| 部分 | 内容 | 用途 |
| --- | --- | --- |
| key | `RequestRecord.id` | 分区键和下游幂等标识 |
| value | protobuf 编码的 `RequestRecord` | 跨进程业务数据 |
| `content-type` | 固定 protobuf 类型 | 防止消费者误解编码格式 |
| `message-type` | 固定请求记录类型 | 防止其他消息混入 Topic |
| `traceparent`、`tracestate` | 当前批次 Trace Context | 连接 ALS 与 Analytics 的 Trace |

Analytics 会校验 key 与 value 中的 ID 一致，并校验两个固定 header。仅能解析 protobuf 还不够，消息外壳也必须符合这份协议。

## Producer 的实际配置

Kafka 客户端直接使用 franz-go 的重试和幂等 Producer。省略连接参数后，关键 Producer 选项如下：

```go
[]kgo.Opt{
	kgo.DefaultProduceTopic(config.GetTopic()),
	kgo.RequiredAcks(kgo.AllISRAcks()),
	kgo.ProducerBatchCompression(kgo.ZstdCompression()),
	kgo.RecordDeliveryTimeout(config.GetWriteTimeout().AsDuration()),
}
```

这四个 `kgo.Opt` 与 Broker、TLS 和 SASL 连接配置一起传给共用的 `kafkaclient.New`。

franz-go 默认启用幂等 Producer。Ingate 不覆盖它的重试次数和最大在途请求数，避免组合出与幂等要求冲突的参数。项目显式设置的只有：

- 默认 Topic；
- `acks=all`；
- Zstd 压缩；
- 单条记录从进入客户端到得到最终结果的最长时间。

`ProduceSync` 会阻塞到这一批的每条消息都有最终结果。它不是 Kafka 事务：同一批可以有一部分成功、一部分失败。

## 副本、ISR 与确认

![Kafka 三副本、ISR 与 acks all 的确认关系](/ingate/images/als/kafka-isr.svg)

`replication.factor=3` 表示 Topic 的每个分区有三份副本，不表示集群只能部署三个 Broker。一个五 Broker 集群仍可让某个分区只占用其中三个 Broker，其他分区使用不同组合。

每个分区只有一个 Leader。Follower 持续从 Leader 拉取日志，达到 Kafka 的同步条件后进入 ISR（in-sync replicas）。ISR 会随着 Broker 故障和复制延迟变化。

设某分区的副本为 B1、B2、B3：

```text
replicas = {B1, B2, B3}
ISR      = {B1, B2}
leader   = B1
```

此时 `acks=all` 等待 B1 和 B2，不等待已经退出 ISR 的 B3。`min.insync.replicas=2` 允许本次写入。如果 B2 也退出 ISR，只剩 B1，写入会失败，即使 Leader 仍然在线。

| 配置 | 直接约束 | 仍未保证 |
| --- | --- | --- |
| `replication.factor=3` | 每个分区配置三份副本 | 三份副本始终在线或位于不同可用区 |
| `min.insync.replicas=2` | ISR 少于两个时拒绝写入 | Producer 一定请求 ISR 确认 |
| `acks=all` | 等待当前 ISR 全部确认 | 等待配置的全部三份副本 |

生产模式同时要求副本数至少为 3、`min.insync.replicas` 至少为 2，并固定 `acks=all`。这个组合允许一个副本暂时故障，同时阻止只剩 Leader 单份数据时继续确认写入。写入可用性降低的时间由 WAL 承接。

## 幂等 Producer 的范围

Kafka 给 Producer 分配 Producer ID，并为每个分区的写入维护序列号。客户端因网络错误重试同一个批次时，Broker 能识别已经接收的序列，避免在同一 Producer 会话内追加两份。

它不能覆盖以下窗口：

- ALS 重启后建立新的 Producer 会话；
- Kafka 已写入，但 ALS 没收到确认，随后把整批写入 WAL；
- Kafka 写入成功，但 WAL Commit 失败，重启后再次回放；
- Analytics 入库成功，但提交 Kafka offset 前退出。

因此，Producer 幂等减少会话内重复；`RequestRecord.id` 和消费端去重处理跨进程重复。二者解决的范围不同。

幂等模式需要 `acks=all`。若 Leader 在 Follower 复制消息和序列状态前就返回成功，随后 Leader 故障，新 Leader 可能同时缺少消息和对应序列。客户端此时无法只靠 Producer ID 判断上一批已经写入。

## 逐条结果汇总

`ProduceSync` 返回每条 message 的结果。ALS 对成功项计数，对失败项保留最高优先级的错误类别：

```go
for _, produced := range c.kafka.ProduceSync(ctx, messages...) {
	if produced.Err == nil {
		result.Confirmed++
		continue
	}

	result.Failed++
	class := classifyPublishError(produced.Err)
	if class > result.Class {
		result.Class = class
		result.Err = fmt.Errorf("produce request records: %w", produced.Err)
	}
}
```

`PublishClass` 的数值顺序是 `temporary < uncertain < permanent`，所以简单的 `class > result.Class` 就能让批次采用最严重结果。`Err == nil` 只表示全部消息都成功；部分成功时 `Confirmed` 仍会保留真实数量，用于指标。

## 错误分类

分类先判断消息已经写入的可能性，再判断 Kafka 是否允许重试：

```go
func classifyPublishError(err error) biz.PublishClass {
	switch {
	case mayHavePublished(err):
		return biz.PublishUncertain
	case errors.Is(err, kgo.ErrMaxBuffered), kerr.IsRetriable(err):
		return biz.PublishTemporary
	default:
		return biz.PublishPermanent
	}
}
```

| 类别 | 例子 | Recorder 行为 | Replayer 行为 |
| --- | --- | --- | --- |
| `temporary` | 本地缓冲满、Kafka 明确可重试错误 | 原批次写 WAL | 退避后重试 |
| `uncertain` | 网络错误、请求超时、`NotEnoughReplicasAfterAppend` | 原批次写 WAL，允许重复 | 退避后重试 |
| `permanent` | 消息或配置无法靠重试修复 | 原批次仍先写 WAL | 队首再次失败后暂停 |

`uncertain` 必须优先于 `kerr.IsRetriable`。网络超时即使可以重试，也不能证明 Broker 没有写入消息。把它归成普通临时错误会掩盖重复窗口。

只要批次包含失败项，Recorder 就把原批次完整写入 WAL。已经确认的子集可能再次出现，但其 ID 不变。保存整批比根据不完整结果拼出失败子集更保守，下游能够识别由此产生的重复。

## Topic 契约缓存

TopicMonitor 在 transport 启动前做第一次检查，之后每分钟刷新。检查成功时调用 `TopicContract.Update`；检查请求失败时不覆盖旧值：

```go
topology, err := m.reader.ReadTopology(checkCtx)
if err != nil {
	m.checkFailed = true
	return
}

status := m.contract.Update(topology)
```

进程刚启动且检查失败时，状态为 unknown，Recorder 先写 WAL。已有合规结果后发生元数据请求错误，缓存会一直保持上次结果，直到下一次成功检查。若 Topic 在这段时间内被改坏而 Produce 仍可执行，Recorder 可能继续按旧的合规结果直写；当前状态中也没有暴露“上次成功检查时间”。因此平台侧修改 Topic 后应立即用 Kafka 工具验证配置，不能只依赖 ALS 的缓存状态。真实网络故障则会在 Kafka 写入时触发 WAL 降级。

契约检查读取每个分区的最小副本数和 Topic 的 `min.insync.replicas`。它不读取运行时 ISR，也不校验机架分布和非同步副本选主。ISR 不足由实际写入错误和 `kafka_isr_failures_total` 反映。

## 源码入口

- `internal/als/data/kafka/client.go`：Producer 配置
- `internal/als/data/kafka/publisher.go`：消息编码、逐条结果和错误分类
- `internal/als/data/kafka/topic.go`：Kafka 元数据读取
- `internal/als/biz/topic.go`：可靠性契约与原子缓存
- `internal/als/server/topic_monitor.go`：首次检查和周期刷新

## 上游资料

- [Apache Kafka Producer 配置](https://kafka.apache.org/40/generated/producer_config.html)
- [Apache Kafka Topic 配置](https://kafka.apache.org/41/generated/topic_config.html)
- [Apache Kafka 的持久性说明](https://kafka.apache.org/20/design/design/#availability-and-durability-guarantees)
- [franz-go 生产与消费说明](https://github.com/twmb/franz-go/blob/master/docs/producing-and-consuming.md)
