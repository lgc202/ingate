---
title: 阅读路线与术语
description: 为第一次接触消息系统、持久日志和可观测性的读者建立 ALS 知识地图
---

ALS 同时涉及 Envoy gRPC、Kafka、文件系统、并发状态和可观测性。直接从某个函数开始读，容易把不同系统的确认语义混在一起。本页先给出一条学习路线，再统一文档中反复出现的术语。

## 两层阅读路线

第一次阅读只需要先建立一条主线：

```text
Envoy 完成请求
  -> 批量发送 access log
  -> ALS 转成 RequestRecord
  -> Kafka 成功：本批结束
  -> Kafka 失败：写入本地 WAL
  -> Kafka 恢复：从 WAL 重放
  -> Analytics 消费并写 ClickHouse
```

按下面顺序阅读，可以先理解系统行为，再进入实现细节：

1. [ALS 设计总览](../)：知道组件位置和四段可靠性边界。
2. [协议入口与记录转换](../ingestion/)：理解 Envoy 发来的数据长什么样。
3. [Kafka 可靠写入](../kafka/)：理解正常路径何时算成功。
4. [WAL 与故障恢复](../wal/)：理解 Kafka 故障后的保存与重放。
5. [故障模型与投递语义](../failure-model/)：把重复、丢失和确认窗口放到一张表中。

完成主线后，再按问题进入以下内容：

- 数据为何可能重复：[记录 ID 与幂等](../idempotency/)
- 并发请求如何一起切换 WAL：[并发与状态迁移](../concurrency/)
- WAL 应配置多大：[容量与吞吐规划](../capacity/)
- 如何发现和定位故障：[ALS 可观测性](../observability/)
- 多个故障同时发生时如何推导结果：[设计推演](../reasoning/)
- 如何用代码和运行环境验证结论：[验证与测量](../verification/)

## 最小背景知识

### Envoy ALS

ALS 是 Access Log Service。Envoy 把已经完成的请求整理成访问日志，通过一个长期存在的 gRPC Client Streaming 连接发送给 ALS。

Client Streaming 表示客户端可以连续发送多条 message，服务端在 stream 结束时才返回一次响应。官方协议没有“每个批次对应一个 ACK”的消息。因此 ALS 无法让 Envoy 确认某一批已经进入 Kafka 或 WAL。

### gRPC stream 与 batch

stream 是持续存在的传输通道，batch 是通道中的一次消息内容。一个 Envoy ALS stream 可以存活数小时，其中包含许多 batch。一个 batch 又可以包含多条 HTTP access log。

```text
one stream
  ├─ batch 1: record A, B
  ├─ batch 2: record C
  └─ batch 3: record D, E, F
```

stream 断开和单个 batch 失败不是同一个粒度。ALS 终止 stream 后 Envoy 会尝试重新建立连接，但协议没有承诺重发刚才的 batch。

### Kafka Topic、Partition 和 Offset

- Topic 是一类消息的逻辑名称。
- Partition 是 Topic 中一条有序追加日志。
- Offset 是消息在 Partition 中的位置。
- 一个 Topic 可以有多个 Partition，所以不存在自动的 Topic 全局顺序。

Kafka Key 参与分区选择。ALS 使用 `RequestRecord.id` 作为 Key。分区数不变时，相同 ID 通常落到同一 Partition；扩容 Partition 后映射可能变化。

### Broker、Leader、Follower 和 Replica

Broker 是 Kafka 服务节点。一个 Partition 可以在多个 Broker 上保存 Replica，其中一个 Replica 是 Leader，其余是 Follower。Producer 向 Leader 写入，Follower 从 Leader 复制。

`replication.factor=3` 表示每个 Partition 有三份配置副本，不表示消息保存在集群全部 Broker。

### ACK 与不确定确认

ACK 是 Acknowledgement，即确认。`acks=all` 表示 Kafka Leader 等待当前 ISR 全部完成复制后，再向 Producer 返回成功。

确认响应可能在网络中丢失。Producer 看到超时时，无法判断 Broker 没写，还是 Broker 已写但响应没回来。这种状态在文档中称为 `uncertain`。

### ISR 与 High Watermark

ISR 是当前复制进度满足同步条件的 Replica 集合，Leader 也在其中。High Watermark 表示 ISR 共同确认到的位置，普通 Consumer 只读取该位置之前的消息。

配置副本数较稳定，ISR 会随宕机和复制延迟变化。`min.insync.replicas` 限制 ISR 至少保留多少成员时才能接受 `acks=all` 写入。

### WAL、segment 和 entry

WAL 是只能按顺序追加的持久日志。Ingate WAL 目录由多个 segment 文件组成，每个 segment 包含若干 entry。一个 entry 保存一次降级批次。

```text
WAL directory
  ├─ segment 1
  │    ├─ entry 1 -> RequestRecord A, B
  │    └─ entry 2 -> RequestRecord C
  └─ segment 2
       └─ entry 3 -> RequestRecord D, E
```

segment 用于限制单文件大小和快速删除旧前缀；entry 是业务确认单位。`replay_batch_size` 可以合并多个 entry 读取，但不会拆开一个 entry。

### `write`、Page Cache 与 `fsync`

`write` 通常先把数据交给操作系统 Page Cache。应用进程退出后，内核仍可能把数据刷盘；整机掉电时，尚未稳定化的数据可能丢失。

`fsync` 请求操作系统将文件内容稳定化到存储设备。它提高崩溃恢复能力，也把磁盘同步延迟加入请求耗时。最终保证仍取决于文件系统、Volume 和硬件是否正确实现 flush。

### Durability 与 Availability

- Durability 表示已经确认的数据在故障后仍能保留。
- Availability 表示系统当前仍能接受操作。

`replication.factor=3 + min.insync.replicas=2 + acks=all` 在只剩单副本时拒绝写入。它主动降低写入 Availability，避免向客户端确认一份副本数据。ALS 的 WAL 用于承接这段可用性下降。

### At Most Once、At Least Once 和 Exactly Once

| 语义 | 可能丢失 | 可能重复 | 典型实现 |
| --- | --- | --- | --- |
| At Most Once | 是 | 否 | 先确认或不重试 |
| At Least Once | 尽量避免 | 是 | 成功确认前持续重试 |
| Exactly Once Effect | 由系统边界决定 | 物理消息可重复，业务结果去重 | At Least Once + 稳定 ID + 幂等写入 |

ALS 的 WAL 回放是 At Least Once。Kafka 中允许出现相同 `RequestRecord.id` 的重复消息，Analytics 和 ClickHouse 负责让业务结果尽量保持一次。

“Exactly Once”必须说明边界。Kafka 内部事务、Kafka 到 ClickHouse、Envoy 到 ALS 和最终账单并不属于同一个原子系统，不能用一个词概括全部阶段。

### 幂等

幂等表示同一个操作执行一次和执行多次，最终业务效果相同。例如使用同一个记录 ID 重试插入，存储中仍只产生一条事实。

Kafka 幂等 Producer 使用 PID、epoch 和 sequence 消除同一 Producer 会话中的协议重试；`RequestRecord.id` 处理 ALS 重启、WAL 回放和消费重投。两层幂等覆盖不同范围。

### Backpressure

Backpressure 是下游处理速度低于上游生产速度时，压力向上游传播的现象。表现可以是阻塞、队列增长、延迟上升或主动拒绝。

ALS 对不同数据采用不同策略：

- RequestRecord：Kafka 变慢后切换 WAL，WAL 满后明确拒绝。
- Trace：有界队列满后丢 Span，不能阻塞 RequestRecord。
- 容器日志：Docker non-blocking buffer 满后允许丢日志。

### Counter、Gauge 和 Histogram

- Counter 只增加，适合累计事件；重启后归零。
- Gauge 可以上升和下降，适合当前积压、容量和活跃 stream。
- Histogram 把每次耗时或大小放入累计 bucket，用于计算 p95、p99。

原始 Counter 常用 `rate()` 或 `increase()` 查询。Gauge 通常看当前值或一段时间的最大值。Histogram 分位数需要对 bucket 先求 rate，再按 `le` 聚合。

### Trace、Span、Parent 和 Link

Trace 表示一次工作的完整因果关系，Span 表示其中一个步骤。同步嵌套调用通常使用 Parent/Child。WAL 回放与原批次相隔很久，使用 Link 表示“由历史工作触发”，避免制造一个持续数小时的父子 Trace。

## 从仓库代码进入主流程

### Envoy 批次进入 Recorder

```text
service.Service.StreamAccessLogs
  -> stream.Recv
  -> service.acceptBatch
  -> service.accessLogNodeID
  -> service.parseRequestRecords
  -> biz.Recorder.Write
```

这段代码回答协议输入、字段转换、坏记录处理和批次 Trace。入口位于 `internal/als/service`，持久化选择从 `biz.Recorder.Write` 开始。

### Kafka 直写

```text
biz.Recorder.Write
  -> recorderState.reserveWriteTarget
  -> biz.Recorder.writeKafka
  -> kafka.Client.Publish
  -> kafka.newRecords
  -> kgo.Client.ProduceSync
  -> kgo.Client.Produce
  -> kgo.Client.produce
  -> partition record buffer
  -> sink.produce
  -> broker ProduceRequest
  -> kgo.Client.finishBatch
  -> PublishResult
```

前五层属于 Ingate，`kgo.Client.ProduceSync` 之后属于固定版本 franz-go。追查等待、取消、分区和序列行为时，应直接阅读该版本源码，避免把其他 Kafka 客户端的实现套进来。

### Kafka 失败转入 WAL

```text
kafka.Client.Publish returns error
  -> recorderState.failKafkaWrite
  -> biz.Recorder.writeQueue
  -> context.WithoutCancel
  -> diskqueue.Queue.Write
  -> diskqueue.encodeEntry
  -> capacityPolicy.admits
  -> tidwall/wal.Log.Write
  -> tidwall/wal.Log.writeBatch
  -> os.File.Write
  -> os.File.Sync
```

`failKafkaWrite` 在同一个锁内结束 Kafka 在途计数、建立故障屏障并预留 WAL 写入。后续批次看到屏障后会直接写 Queue，不再逐批等待 Kafka 超时。

### WAL 回放

```text
server.DiskQueueReplayer.replay
  -> biz.Recorder.PrepareReplay
  -> biz.Recorder.ReplayBatch
  -> diskqueue.Queue.Read
  -> kafka.Client.Publish
  -> diskqueue.Queue.Commit
  -> tidwall/wal.Log.TruncateFront
```

`Read` 与 `Commit` 分离是 At Least Once 的核心。Kafka 发布成功、Commit 失败时，磁盘记录仍在队首，下次会再次发布。

### 指标与 Trace

```text
operation
  -> metrics.EventCollector / Recorder counters
  -> prometheus.NewHandler
  -> GET /metrics

operation context
  -> tracer.Start
  -> span.End
  -> telemetry.spanBuffer
  -> BatchSpanProcessor
  -> OTLP exporter
```

指标调用应靠近它所证明的完成点；Span 则包围需要观察耗时和因果关系的操作。看到一个指标时，先找到它的 `Inc/Add/Observe` 位置，再判断它究竟证明到了哪一步。

## 阅读完成后的理解检查

完成上述文档后，应能独立推导以下结果：

- Kafka ACK 丢失后，为什么错误可重试却仍属于 `uncertain`。
- `min.insync.replicas=2` 为什么不等于 `acks=2`。
- ALS 重启后 Kafka Producer ID 为什么变化，业务 ID 为什么不变化。
- Kafka 已确认而 WAL Commit 失败时，为什么选择重复而不是提前删除。
- 一次 `write` 返回和一次 `fsync` 返回分别证明到哪一层。
- 文件 `fsync` 成功后，目录项仍存在哪种掉电边界。
- WAL 有积压时，Kafka 恢复为什么不代表队列一定能够排空。
- Tempo 故障为什么可以丢 Trace，同时不能拖住 Kafka/WAL 写入。
- `rejected / valid` 为什么是可靠接收代理，无法直接当作精确丢失率。
- Request ID、`RequestRecord.id` 和 Kafka PID 分别属于哪个生命周期。

无法解释某一项时，回到对应专题和[设计推演](../reasoning/)，沿代码入口验证，不需要记忆孤立结论。
