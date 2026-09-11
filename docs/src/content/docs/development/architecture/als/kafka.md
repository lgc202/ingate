---
title: Kafka 可靠写入
description: 从一条消息的客户端生命周期、ISR 确认和幂等序列解释 ALS 的 Kafka 写入语义
---

Kafka 是 ALS 正常路径上的持久化边界。`Recorder` 只有拿到 Producer 对每条消息的成功结果，才把相应记录计入 `kafka_accepted`。如果结果失败或无法判断，原批次会写入本地 WAL，等待以后重放。

这句话包含三个容易混淆的概念：

- **发送**：客户端把 Produce 请求写入网络连接。
- **Broker 追加**：分区 Leader 把消息追加到自己的日志。
- **确认**：Producer 收到满足 `acks` 条件的成功响应。

发送成功不等于 Broker 已追加，Broker 已追加也不等于 Producer 收到了确认。ALS 的故障语义正是由这三个时刻之间的空隙决定的。

## 先建立 Kafka 的基本模型

一个 Kafka Topic 被拆成若干 Partition。每个 Partition 是一条只能在尾部追加的有序日志。Partition 可以有多个 Replica，其中一个是 Leader，其余是 Follower。

```text
Topic: ingate.request-records

Partition 0:  [0][1][2][3]...   Leader=B1, Followers=B2/B3
Partition 1:  [0][1][2]...      Leader=B2, Followers=B1/B4
Partition 2:  [0][1][2][3][4]   Leader=B3, Followers=B2/B5
```

Producer 只向目标 Partition 的 Leader 写入。Follower 不接收客户端的 Produce 请求，而是从 Leader 拉取日志。Consumer 也按 Partition 读取，因此 Kafka 只保证**单个 Partition 内的顺序**，不保证整个 Topic 的全局顺序。

ALS 不依赖 Kafka 的全局顺序。查询侧使用 `started_at` 等显式字段排序，不能把“先出现在 Kafka”解释成“请求一定先发生”。

## 一条 `RequestRecord` 如何变成 Kafka message

`internal/als/data/kafka/publisher.go` 将每条记录编码成一条独立 message：

```go
message := &kgo.Record{
	Key:     []byte(record.GetId()),
	Value:   value,
	Headers: headers,
}
```

message 由四部分组成：

| 部分 | 当前内容 | 作用 |
| --- | --- | --- |
| Topic | 客户端配置的 `ingate.request-records` | 指定逻辑消息流 |
| Key | `RequestRecord.id` | 参与分区选择，并作为下游幂等标识 |
| Value | protobuf 编码的 `RequestRecord` | 承载请求分析数据 |
| Headers | 消息类型、编码类型、Trace Context | 校验协议并关联下游 Trace |

固定 header 包含：

```text
content-type  = application/x-protobuf
message-type  = ingate.als.v1.RequestRecord
traceparent   = 00-<trace-id>-<span-id>-<flags>   # 采样时存在
tracestate    = ...                               # 可选
```

Analytics 不会只看 Value 能否反序列化。它还会检查固定 header，并验证 Kafka Key 与 Value 中的记录 ID 相等。这样能发现“消息发到了正确 Topic，但外壳协议或主键不一致”的数据污染。

## franz-go 发布链路

当前代码调用：

```go
results := c.kafka.ProduceSync(ctx, messages...)
```

`ProduceSync` 的“同步”只表示调用方等待这一组记录各自得到最终结果。客户端内部仍然会并发组批和发送。一次发布大致经过以下步骤：

```text
RequestRecord
  -> protobuf 编码
  -> 根据 key 选择 Partition
  -> 放入该 Partition 的客户端缓冲
  -> 与同 Partition 的其他记录组成 RecordBatch
  -> Zstd 压缩 RecordBatch
  -> 找到 Partition Leader 所在 Broker
  -> 复用或建立 Broker TCP 连接
  -> 发送 ProduceRequest
  -> Leader 追加，Follower 复制
  -> 满足 acks=all 后返回 ProduceResponse
  -> ProduceSync 汇总每条记录的结果
```

下面分别解释其中容易被忽略的部分。

### 1. 分区选择

Ingate 没有覆盖 franz-go v1.21.0 的默认 Partitioner。该版本默认使用：

```go
UniformBytesPartitioner(64<<10, true, true, nil)
```

参数中的 `keys=true` 表示有 Key 的记录使用 Kafka 兼容的 Murmur2 哈希。可以把选择过程近似理解为：

```text
partition = positive(murmur2(record.id)) % partition_count
```

因此，同一个 `RequestRecord.id` 在**分区数不变**时会落到同一个 Partition。WAL 重放使用原 ID，所以常见重投仍落在原分区。若运维期间增加 Partition，取模基数发生变化，同一个 Key 可能映射到另一个 Partition；业务去重不能依赖“重复消息永远位于同一分区”。

UUID v4 的 Key 接近均匀分布，适合把 ALS 记录分散到多个 Partition。代价是相邻请求通常进入不同 Partition，所以 Kafka 不提供跨请求的业务顺序。

### 2. 客户端缓冲与组批

franz-go 为各 Topic/Partition 维护待发送记录。记录不会必然对应一个独立网络请求。客户端会在批次达到大小限制、等待时间到期或其他发送条件满足时冻结批次，再把多个 Partition 的批次组合进发往同一 Broker 的 ProduceRequest。

Ingate 未覆盖的 v1.21.0 关键默认值包括：

| 参数 | franz-go 默认值 | 含义 |
| --- | ---: | --- |
| `ProducerLinger` | 10 ms | 异步 Produce 时，一个 Partition 等待更多记录加入批次的默认时间 |
| `MaxBufferedRecords` | 10,000 | 客户端尚未完成的记录数上限 |
| `ProducerBatchMaxBytes` | 约 1,000,012 bytes | 单个未压缩 RecordBatch 的目标上限 |
| `RecordRetries` | 近似无限 | 在安全条件下持续重试，由投递超时约束总时长 |

`ProduceSync` 有一项专门行为：本次调用把所有记录入队后，会解除这些已知 Partition 的 linger 并主动触发 drain，避免同步等待方再空等完整的 linger 时间。并发的异步 `Produce` 所建立的其他 linger 不受影响。因此当前路径可以在同一次调用的记录之间组批，但通常不会机械增加 10 ms 等待。

由此可以得到两个结论：

1. ALS 调用 `ProduceSync`，不表示“一条记录一个请求”；同步发生在调用边界，网络层仍然组批。
2. `RecordDeliveryTimeout` 约束的是记录在客户端内的整个生命周期，不只是一次网络 RTT。

### 3. 压缩发生在批次层

当前显式配置 Zstd：

```go
kgo.ProducerBatchCompression(kgo.ZstdCompression())
```

Kafka 压缩的是 RecordBatch，而不是先独立压缩每一条 Value。相似字段越多、批次越大，压缩率通常越好；批次过小则固定头部和压缩开销占比更高。Broker 保存压缩后的批次，Consumer 拉取后再解压，因此压缩通常同时减少 Producer 网络流量、Broker 磁盘占用和 Consumer 网络流量，但会增加两端 CPU。

ALS 目前没有证据需要手工调整 linger 或批次大小。若要调优，应同时测量：发布 p95/p99、每秒记录数、平均批次大小、压缩率、CPU 和恢复时的净回放速度，而不是只看单项吞吐。

## Replica、ISR 和 High Watermark

### Replica 与 `replication.factor`

`replication.factor=3` 表示**每个 Partition 保存三份副本**。它不表示集群只能有三个 Broker，也不表示每份数据会出现在全部 Broker。

例如集群有五个 Broker：

```text
Partition 0 replicas: B1, B2, B3
Partition 1 replicas: B2, B3, B4
Partition 2 replicas: B3, B4, B5
```

Partition 0 的数据不会因为 B4、B5 也属于集群就自动复制到它们。Kafka 在不同 Partition 间分散副本，才让整个集群利用更多 Broker。

### ISR 的运行时含义

ISR 是 **in-sync replicas**，即当前被 Leader 认为复制进度满足同步条件的副本集合。Leader 自己也属于 ISR。

```text
configured replicas = {B1, B2, B3}
current ISR         = {B1,B2}
leader              = B1
```

B3 仍是配置副本，但由于宕机或复制落后，已经退出 ISR。它恢复并追上 Leader 后可以重新进入 ISR。ISR 是运行时集合，会变化；`replication.factor` 是 Topic 元数据中的配置，不随临时故障改变。

### High Watermark 与消费可见性

每个副本都保存自己的日志末端位置。Leader 根据 ISR 的共同复制进度推进 High Watermark。普通 Consumer 只读取已提交到 High Watermark 的消息，避免读到只存在于旧 Leader、Leader 切换后可能消失的数据。

```text
Leader B1:    [0][1][2][3][4]   log end = 5
Follower B2:  [0][1][2][3]      log end = 4
Follower B3:  [0][1][2][3]      log end = 4
HighWatermark:             ^      offsets < 4 可见
```

“Leader 已经追加”与“消息已经对 Consumer 可见”不是同一个时刻。

## `acks=all` 与 `min.insync.replicas`

Ingate Producer 固定使用：

```go
kgo.RequiredAcks(kgo.AllISRAcks())
```

`acks=all` 表示 Leader 只有在当前 ISR 中所有副本都确认追加后，才向 Producer 返回成功。`min.insync.replicas` 则规定使用 `acks=all` 时，至少要有多少个 ISR 成员才能接受写入。

生产建议组合为：

```text
replication.factor = 3
min.insync.replicas = 2
acks = all
```

不同运行状态下的结果如下：

| 当前 ISR | `acks=all` 等待谁 | 写入结果 |
| --- | --- | --- |
| `{B1,B2,B3}` | B1、B2、B3 | 三者都确认后成功 |
| `{B1,B2}` | B1、B2 | 两者都确认后成功 |
| `{B1}` | 不满足最小 ISR | 拒绝写入 |

`min.insync.replicas=2` 不是“只等任意两个副本”。若 ISR 有三个，`acks=all` 仍然等三个；它只决定 ISR 缩小到什么程度后必须停止接受写入。

这个组合把一个常见取舍写进配置：允许一个副本暂时故障，但拒绝只剩一份在线副本时继续确认。Kafka 的写入可用性下降后，ALS 用 WAL 承接新记录。

## 幂等 Producer 到底如何去重

franz-go 默认启用幂等 Producer。Kafka 为一次 Producer 会话维护三类信息：

- **Producer ID（PID）**：标识 Producer 会话。
- **Producer epoch**：区分同一 PID 的新旧世代，旧 epoch 可以被拒绝。
- **Sequence number**：每个 Partition 单独递增的批次序列。

Producer 发送的 RecordBatch 会携带 PID、epoch 和 base sequence。Broker 为对应 Partition 记住最近接受的序列范围：

```text
PID=42, epoch=3, partition=7

batch A: baseSequence=10, records=3 -> sequence 10..12 -> 首次追加
batch A: baseSequence=10, records=3 -> sequence 10..12 -> 识别为重试，不再次追加
batch B: baseSequence=13, records=2 -> sequence 13..14 -> 继续追加
```

如果序列出现无法解释的空洞或倒退，Broker 会返回 sequence 相关错误。epoch 则防止旧 Producer 实例在新世代已经接管后继续写入。这类“fencing”保护的是协议会话，不是业务主键。

### 幂等 Producer 对 `acks=all` 的依赖

假设只等待 Leader：

1. 旧 Leader 追加消息并向 Producer 返回成功。
2. Follower 还没复制到消息和相应序列状态，旧 Leader 就宕机。
3. 缺少这批数据的 Follower 成为新 Leader。
4. Producer 继续写入时，新 Leader 无法基于完整序列历史判断上一批。

幂等不只是“消息内容去重”，还依赖 Broker 保存连续的 Producer 序列状态。franz-go 因此拒绝“启用幂等但 `acks` 不是 all”的配置。

### 在途请求与分区顺序

若同一 Partition 同时有多个 Produce 请求在途，请求 A 临时失败、请求 B 先成功，随后重试 A 就可能把顺序颠倒。幂等协议通过序列号和安全的在途数量限制来维持每个 Partition 的顺序。

Ingate 没有自行设置 `MaxProduceRequestsInflightPerBroker`。franz-go 在启用幂等时使用协议允许的安全值，并协调失败批次的序列。随意覆盖在途数、重试数和 `acks`，容易得到“吞吐看似提高，但序列保证被破坏”的组合。

### 幂等 Producer 不等于端到端 Exactly Once

PID 和 Sequence 只覆盖客户端**同一会话内**按 Kafka 协议重发同一批次。下面这些重复超出了它的范围：

- ALS 重启后创建新的 Producer 会话；
- Kafka 已追加，但 ALS 没收到确认，随后把原批次写入 WAL；
- Kafka 已确认 WAL 记录，但 WAL Commit 失败，重启后再次回放；
- Analytics 入库成功，但提交 Kafka offset 前退出。

这些窗口由稳定的 `RequestRecord.id` 和消费端幂等处理解决。Kafka Producer 幂等与业务 ID 是两层机制，不能互相替代。

Kafka 事务也不能直接解决当前问题。事务适合把 Kafka 消费 offset 与新的 Kafka 输出原子提交；ALS 的另一条持久边界是本地文件系统，Analytics 的目标又是 ClickHouse。Kafka 无法替它们完成一个跨 Kafka、磁盘和 ClickHouse 的分布式事务。

## 超时产生的不确定结果

当前 Producer 选项为：

```go
[]kgo.Opt{
	kgo.DefaultProduceTopic(config.GetTopic()),
	kgo.RequiredAcks(kgo.AllISRAcks()),
	kgo.ProducerBatchCompression(kgo.ZstdCompression()),
	kgo.AllowIdempotentProduceCancellation(),
	kgo.RecordDeliveryTimeout(config.GetWriteTimeout().AsDuration()),
}
```

最关键的故障窗口如下：

```text
ALS           Leader          Followers
 | Produce       |                |
 |-------------->| append         |
 |               |--------------->| replicate
 |               |<---------------| done
 |               | send success   |
 |        X connection breaks      |
 | timeout       |                |
```

ALS 看到超时时，Broker 可能已经满足了复制条件，只是响应丢失。客户端不能通过错误类型反推出 Broker 一定没有数据。

franz-go 的默认幂等策略会尽量保住当前序列窗口：已经发出且结果不明的请求，即使 delivery timeout 到期，也可能继续等待一个可确定结果。这样能减少应用层重复发布，但等待时间没有明确上界。

ALS 需要在 Kafka 长时间不响应时切换到 WAL，因此启用 `AllowIdempotentProduceCancellation`。它允许上下文取消或投递超时结束不确定的在途请求。随后 Recorder 将原批次写入 WAL。若 Broker 实际已经追加，回放时会产生同 ID 重复。

这是明确的工程取舍：

| 方案 | 等待边界 | 重复风险 | 对 ALS 的影响 |
| --- | --- | --- | --- |
| franz-go 默认幂等行为 | 模糊确认时可能继续等待 | 较低 | WAL 可能迟迟无法接管 |
| 当前 ALS 配置 | 接近 `write_timeout`，但不是硬实时 | 边界故障允许同 ID 重复 | 能及时转入本地持久队列 |

`RecordDeliveryTimeout` 是近似上限，原因包括：

- 客户端在发请求前或收到响应后检查超时，不会中断每一个内部步骤；
- Broker sink 的退避可能让返回稍晚；
- 同一客户端批次中的记录继承首条记录的超时时刻；
- 调用 Context 更早取消时，也可能先结束发布。

它表示“ALS 不愿继续等待 Kafka 确认的时间预算”，不表示“超时发生时 Broker 一定没有写入”。

## 部分成功后的整批 WAL 保存

`ProduceSync` 返回逐条结果，不提供整批事务。一个 ALS 批次可能出现：

```text
10 条记录
  6 条 success
  2 条 timeout / uncertain
  2 条 permanent error
```

Publisher 汇总成功数、失败数和最高严重级别：

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

只要存在失败项，Recorder 就将**原批次**写 WAL。已经确认的六条以后可能再次出现，但 ID 保持不变。这样做的理由是：不确定错误无法证明某条消息未写入，而回调结果又可能跨 Partition 交错。保留原批次形成的是可识别重复；错误地筛选所谓“失败子集”更容易留下无法识别的缺口。

## 错误分类的作用

错误分类先判断“可能已经写入”，再判断“是否值得重试”：

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

| 类别 | 含义 | 常见例子 | 直写失败后的行为 | WAL 回放时的行为 |
| --- | --- | --- | --- | --- |
| `temporary` | 当前失败，重试有机会恢复 | 元数据刷新、本地缓冲满、明确可重试错误 | 原批次写 WAL | 指数退避后重试 |
| `uncertain` | 无法判断 Broker 是否已追加 | 网络错误、请求超时、`NotEnoughReplicasAfterAppend` | 原批次写 WAL，接受重复 | 指数退避后重试 |
| `permanent` | 相同数据和配置下继续重试通常无效 | 授权、消息格式或不可恢复配置错误 | 仍先尝试保存原批次到 WAL | 暂停队首，等待人工处理 |

`uncertain` 的判断必须早于“Kafka 是否将它标记为 retriable”。“可以重试”和“此前没有写入”是两个不同问题。网络超时通常可以重试，同时也可能已经写入。

## Topic 契约检查与实际写入检查

`TopicMonitor` 在接收流量前检查一次 Topic，之后每分钟刷新本地缓存。生产模式检查：

- 每个 Partition 的副本数至少为 3；
- Topic 的 `min.insync.replicas` 至少为 2。

它不会同步查询每次写入时的 ISR，也不检查机架或可用区分布。原因是 ISR 会高频变化，把元数据查询放进每批写入会增加延迟和新的故障点。运行时 ISR 是否足够，由真实 Produce 结果判断。

检查请求失败时保留上一次成功结果。这避免 Kafka 短暂元数据故障立即把合规 Topic 标成不合规，但也意味着缓存可能短时间陈旧。平台侧修改 Topic 配置后，应使用 Kafka 管理工具验证实际配置，不能只看 ALS 缓存。

## 参数调优时应回答的问题

| 参数 | 调大后的主要收益 | 主要代价 |
| --- | --- | --- |
| Partition 数 | 提高并行写入和消费上限 | 更多文件、连接和协调开销；Key 映射可能变化 |
| linger | 更容易形成大批次和高压缩率 | 增加低流量时的单条延迟 |
| batch bytes | 提高批处理效率 | 增加瞬时内存和失败批次规模 |
| `write_timeout` | 给 Kafka 更多重试时间 | WAL 接管更慢，gRPC 批次处理时间更长 |
| WAL replay batch | 减少发布和 Commit 次数 | 单次内存、Kafka 请求和重复范围增大 |

一次有意义的压测至少要记录：输入记录数与大小分布、Partition 数、Broker 数、ISR 状态、网络 RTT、发布 p95/p99、压缩前后字节数、错误类别、WAL 增长速度及恢复后的净排空速度。只报告“每秒写入多少条”无法解释瓶颈来自编码、CPU、网络、Broker 磁盘还是副本复制。

## 关键边界

### Broker 数超过三个时的副本数量

副本因子作用于每个 Partition。更多 Broker 用于承载其他 Partition、副本和故障迁移，不会自动把每条消息复制到所有 Broker。

### `acks=all` 实际等待的副本

它等待当前 ISR 全体。ISR 是三个就等三个，是两个就等两个；少于 `min.insync.replicas` 时拒绝。

### Producer 成功确认的持久性边界

成功表示 Kafka 满足了当前副本确认协议。它仍依赖 Broker、文件系统、磁盘 flush、选主策略和运维配置正确；跨可用区故障能力还取决于副本放置。

### 幂等 Producer 仍然出现重复的原因

Kafka 幂等覆盖同一 Producer 会话内的协议重试。ALS 主动取消不确定请求后转写 WAL、ALS 重启和消费端重投都属于更外层的重试，需要业务 ID 去重。

### Kafka 事务不适用于当前跨系统原子边界

当前原子边界跨越 Kafka、本地 WAL 和 ClickHouse。Kafka 事务无法原子提交本地文件截断或 ClickHouse 写入；引入事务也不会消除 Envoy 到 ALS 之间的丢失窗口。

## 源码入口

- `internal/als/data/kafka/client.go`：Producer 连接与可靠性选项
- `internal/als/data/kafka/publisher.go`：消息编码、逐条结果和错误分类
- `internal/als/data/kafka/topic.go`：Topic 元数据读取
- `internal/als/biz/topic.go`：可靠性契约和并发安全缓存
- `internal/als/server/topic_monitor.go`：首次检查和周期刷新

## 上游资料

- [Apache Kafka Topic 配置：`min.insync.replicas`](https://kafka.apache.org/40/generated/topic_config.html)
- [Apache Kafka Producer 配置](https://kafka.apache.org/40/generated/producer_config.html)
- [Apache Kafka 设计：复制与持久性](https://kafka.apache.org/documentation/#design_replicatedlog)
- [franz-go v1.21.0 Producer 配置源码](https://github.com/twmb/franz-go/blob/v1.21.0/pkg/kgo/config.go)
- [franz-go v1.21.0 默认 Partitioner 源码](https://github.com/twmb/franz-go/blob/v1.21.0/pkg/kgo/partitioner.go)
- [franz-go v1.21.0 生产与消费说明](https://github.com/twmb/franz-go/blob/v1.21.0/docs/producing-and-consuming.md)
- [franz-go #1202：允许取消不确定的幂等写](https://github.com/twmb/franz-go/issues/1202)
