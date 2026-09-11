---
title: 设计推演
description: 从复合故障和替代方案推导 ALS 的行为、边界与演进条件
---

单独记住 `acks=all`、WAL 或幂等 Producer 的定义，还不足以判断组合系统的行为。本页从具体事件开始，沿确认边界逐步推导结果。每个结论都能回到当前代码、配置或本地故障演练验证。

## Kafka 已写入，但 ACK 丢失

初始条件：Topic 合规，Recorder 选择 Kafka 直写。

```text
1. Producer 向 Leader 发送批次
2. Leader 与当前 ISR 完成复制
3. Leader 返回成功响应
4. 网络在响应到达 ALS 前断开
5. Producer 到达 delivery timeout
```

第 5 步只能证明 ALS 没收到确认，无法证明 Kafka 没追加。`classifyPublishError` 因此先判断 `mayHavePublished`，将网络错误和请求超时归为 `uncertain`。

Recorder 随后建立 `spooling` 屏障，并把原批次写入 WAL。Kafka 恢复后，WAL 重放相同 `RequestRecord.id`。Kafka 中可能出现两份物理消息，Analytics 按 ID 处理重复。

这条推导依赖四个机制：

| 机制 | 负责的边界 |
| --- | --- |
| `acks=all` | Kafka 成功确认前要求 ISR 完成复制 |
| `AllowIdempotentProduceCancellation` | 不确定请求能够结束并交给 WAL |
| 原批次写 WAL | 不根据模糊结果猜测失败子集 |
| 稳定记录 ID | 让外层重投可被识别 |

去掉任意一层都会改变语义。禁用取消会让 WAL 接管时间失去上界；只写失败子集可能产生缺口；回放生成新 ID 会让重复无法识别。

## ISR 从三个缩小到一个

初始配置：

```text
replication.factor = 3
min.insync.replicas = 2
acks = all
```

运行时过程：

```text
ISR={B1,B2,B3} -> B3 落后 -> ISR={B1,B2} -> B2 故障 -> ISR={B1}
```

前两个状态仍能写入。只剩 B1 后，Leader 拒绝 `acks=all` 写入，因为 ISR 数量低于 2。ALS 把批次转入 WAL，并增加 ISR failure 指标。

降低 `min.insync.replicas` 或改成 Leader ACK 可以恢复 Kafka 写入，但会允许只剩一份在线副本时返回成功。当前设计保留可靠性契约，让本地 WAL 承担可用性下降。

TopicMonitor 只检查副本因子和 `min.insync.replicas` 配置，不持续缓存运行时 ISR。ISR 变化由真实 Produce 错误反映。否则每个写入批次都需要额外查询元数据，查询结果到使用时仍可能过期。

## 多个 Kafka 写入并发时出现一次失败

假设 A 和 B 已取得 Kafka 准入，C 尚未到达：

```text
A: reserve Kafka -> timeout -> reserve WAL -> append WAL
B: reserve Kafka --------------------------> success
C:                arrives -> reserve WAL -> append WAL
```

A 失败时，`failKafkaWrite` 在同一个锁内：

1. 减少 `kafkaWrites`；
2. 标记 Kafka 不可写；
3. 建立 `spooling` 屏障；
4. 为 A 预留一次 `queueWrites`。

B 在屏障前已经取得准入，可能成功，Recorder 不会试图取消或撤销它。C 在屏障后到达，直接写 WAL。恢复 Kafka 直写需要满足：Topic 合规、所有在途 Kafka/WAL 写入结束、回放未暂停、Queue 已空。

这里维持的是单实例写入目标切换，不是请求全局顺序。A、B、C 的 Kafka 可见顺序仍受 Partition、并发完成和回放影响。

## Kafka 故障期间 ALS 被强制终止

初始条件：新记录已经成功追加 WAL，`spooling=true`。

```text
1. Queue.Write 完成 File.Sync
2. ALS 收到 SIGKILL，没有执行优雅关闭
3. 进程重新启动
4. Queue 打开所有 segment
5. 逐条校验格式、CRC 和 RequestRecord
6. 从磁盘重建 pending 统计
```

pending 是运行时原子快照，不是独立持久数据。重启后以 WAL segment 为准重新扫描，避免维护第二份可能与日志不一致的游标文件。

如果进程在 `File.Sync` 成功后、内存快照更新前退出，记录仍能通过扫描恢复。如果在 entry 完整落盘前退出，可能没有该条目，也可能留下无法解析的尾部；当前实现遇到损坏会拒绝启动。

WAL 能覆盖这段进程生命周期，但无法恢复 Envoy 尚未发送的 batch，也无法在 Volume 丢失后从其他节点取回数据。

## Kafka 发布成功，WAL Commit 失败

回放顺序为：

```text
Read -> Publish -> Commit
```

Kafka 成功后，`Queue.Commit` 可能因磁盘 I/O、权限或文件系统空间问题失败。此时 Kafka 已有消息，WAL 仍保留原 entry。

可选动作有两个：

| 动作 | 风险 |
| --- | --- |
| 将 entry 当作完成并跳过 | 无法证明本地确认位置已持久化，可能丢失 |
| 保留 entry 并再次发布 | Kafka 中出现相同 ID 重复 |

当前选择第二项。重复能通过业务 ID 识别；删除尚未确认的本地数据会制造不可恢复缺口。这是 At Least Once 的典型取舍。

## Kafka 恢复，但 WAL 一直增长

Kafka 能成功接受回放，不代表积压一定下降。设：

- `λ` 为每秒新入队记录数；
- `μ` 为每秒 Commit 的回放记录数。

```text
net_drain = μ - λ
```

| 条件 | WAL 变化 |
| --- | --- |
| `μ > λ` | 积压最终排空 |
| `μ = λ` | 积压大致不变，最旧年龄可能继续增长 |
| `μ < λ` | 积压继续增长 |

`pending / μ` 忽略新流量，会给出偏乐观恢复时间。Dashboard 使用 `pending / (committed_rate - spooled_rate)` 估算净排空时间，并在分母接近零时显示很大的值。

提高回放并发并非自动正确。当前 Queue 只保存连续队首确认位置；并发发布会产生乱序完成和确认空洞，需要新的持久状态。修改前应先用实际负载证明单 Replayer 无法达到 `μ > λ_peak`。

## WAL 达到容量边界

WAL 准入同时检查配置容量和文件系统空闲空间，并为中段截断保留临时副本空间。

```text
Kafka unavailable
  -> new batch chooses WAL
  -> capacityPolicy rejects growth
  -> Queue.Write returns ErrQueueFull
  -> records_rejected_total increases
  -> /readyz returns 503, reason=wal_unavailable
```

已有 WAL 记录不会被删除。覆盖最旧数据虽然能继续接收新数据，却会静默改变历史完整性，最旧缺口也难以从 ALS 指标精确恢复。

Envoy 的业务转发仍然可以成功，因为 ALS 是异步 access log 路径。受到影响的是请求分析记录。ALS 终止 gRPC stream 会促使 Envoy 重连，但官方协议没有逐批 ACK，不能承诺当前失败批次一定重发。

## WAL 截断进行到一半时退出

假设 segment 保存 `[100..104]`，当前确认到 102。tidwall/wal 需要保留 `[103..104]`：

```text
write 000...103.START.TEMP
  -> file Sync
  -> rename to 000...103.START
  -> delete old segment
  -> rename to 000...103
```

`.START` 是可识别的中间状态。下次 `wal.Open` 看到它后，会删除更早的旧 segment，再完成 rename。临时文件内容先同步，减少“旧数据已删，新队首却没有完整内容”的风险。

当前上游实现没有显式同步父目录。突然掉电时，rename 和目录项的持久性仍依赖文件系统。这也是“进程崩溃可恢复”和“经过断电认证”不能混为一个承诺的原因。

## OTel Collector 和 Tempo 同时不可用

Trace 经过两层有界缓冲：ALS 进程内 BatchSpanProcessor，以及 Collector Exporter 的 sending queue。

Collector 已不可达时，ALS OTLP Exporter 会在自身超时与重试范围内失败。进程内队列逐渐占满后，新结束的 Span 被丢弃，`ingate_telemetry_spans_dropped_total{reason="queue_full"}` 增长。

```text
Trace backend failure
  -> export slows/fails
  -> bounded queue fills
  -> drop spans
  -> Kafka/WAL code continues
```

这项隔离使诊断信息可以受损，但 RequestRecord 投递不被拖住。若 Trace 使用无界内存队列，长时间故障最终可能耗尽 ALS 内存；若在 `span.End` 阻塞，则可观测后端会进入请求记录的关键路径。

自定义 drop Counter 只统计进程内 queue full。Exporter 已取走 Span、随后最终发送失败时不会增加这个 Counter。完整判断需要联合 Collector 的 receiver accepted/refused、exporter queue、enqueue failed 和 send failed 指标。

## Envoy Request ID 直接作为记录主键

Envoy 或上游提供的 `request_id` 便于跨系统关联，但不满足记录主键约束：

- 客户端可能不提供；
- 外部调用方可以重复或伪造；
- 一次请求经过多层代理时可能产生多条合法观测记录；
- Envoy 重发语义没有给出稳定日志序号。

ALS 在成功解析每条 access log 后生成独立 UUID v4，作为 `RequestRecord.id`。WAL、Kafka 和 Analytics 传递同一对象，所以应用层重投 ID 不变。`request_id` 继续作为查询关联字段，不承担唯一性。

内容哈希也不是直接替代方案。把动态时间、耗时等字段纳入哈希，会让同一业务请求的不同观测不相等；排除这些字段，又可能合并两次内容相同但都合法的请求。

## 将 ALS 数据改为严格计费事实

当前 `RequestRecord` 用于请求分析、排障、趋势和允许小误差的 Token 用量展示。AI ExtProc 与 Redis 在同步路径执行额度判断，ALS 不是额度余额的事实来源。

若需求变成逐笔计费、余额结算或审计事实，现有链路需要改变：

1. 在同步业务事务中生成稳定计量事件，避免 Envoy 发送前丢失没有 ID 的记录；
2. 使用 transactional outbox 或等价持久机制，将业务结果与计量事件写入同一原子边界；
3. 将本地单副本 WAL 替换为复制存储，或明确节点丢失的补偿来源；
4. 建立不可变账本、版本化价格、冲正事件和周期对账；
5. 用消费幂等保证重复事件不重复扣费；
6. 监控从事件产生到最终账本的连续序号或对账差异。

只提高 Kafka 重试次数、改用 UUID v7 或把 ALS WAL 配大，都无法补上 Envoy 到 ALS 之前的事实生成缺口。

## 多个 ALS 实例的边界

每个 ALS 实例必须拥有独立 WAL 目录。目录 `flock` 只阻止同一文件系统上的重复打开，不提供跨节点所有权转移。

多实例共同向 Kafka 发布时：

- 每个实例有自己的 franz-go Producer 会话和 PID；
- 每条新解析记录由本实例生成 UUID；
- Kafka 按记录 ID 分区；
- 实例之间没有全局写入顺序；
- 一个节点丢失只影响该节点尚未重放的本地 WAL。

若需要实例故障后由其他节点接管 WAL，必须增加复制、租约、所有权转移和双写防护。这已经超出“本地故障缓冲”的边界，不能仅通过挂载同一个共享目录实现。

## 设计变更时的推导顺序

修改可靠链路前，按以下顺序记录影响：

1. 输入事实在哪个时刻产生，之前是否可能丢失；
2. 当前操作的成功确认来自哪个组件；
3. 超时能否判断远端没有执行；
4. 重试是否复用同一个业务 ID；
5. 重复由哪一层消除，去重窗口有多长；
6. 本地状态在进程、主机和 Volume 故障后能否恢复；
7. 下游变慢时选择阻塞、缓冲、拒绝还是丢弃；
8. 指标在代码的哪个完成点更新，能够证明到哪一步；
9. 哪些结论已有自动化故障验证，哪些仍依赖部署环境实测。

这套顺序适用于 Producer 参数、WAL 存储、回放并发和观测出口调整。它把配置变化转成确认、重复、丢失和恢复行为，避免只比较参数名称。
