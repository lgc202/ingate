---
title: 容量与吞吐规划
description: 用实际请求速率、记录大小和回放速度估算 ALS 的磁盘余量与恢复时间
---

ALS 的容量问题有两个不同答案：Kafka 故障时 WAL 能接收多久，Kafka 恢复后积压能多久排空。前者取决于磁盘可写预算，后者取决于回放吞吐是否持续高于新记录产生速度。

## 先测四个量

| 符号 | 含义 | 建议数据来源 |
| --- | --- | --- |
| `λ` | 每秒新增合法记录数 | `rate(ingate_als_records_valid_total[5m])` 的高峰值 |
| `b` | 每条记录带来的平均物理磁盘增长 | 故障演练期间 WAL `disk_bytes` 增量除以 `spooled` 增量 |
| `μ` | Kafka 恢复时每秒成功 Commit 的回放记录数 | 无新流量压测时 `rate(ingate_als_records_committed_total[5m])` |
| `T` | 计划容忍的 Kafka 最长不可写时间 | 运维目标，不从代码默认值推断 |

`disk_queue_payload_bytes / disk_queue_records` 只能得到 protobuf 载荷均值，不能直接代替 `b`。WAL 还包含 QueueEntry 外壳、长度前缀、Trace Context、文件系统分配块和暂存的截断副本。生产容量应使用磁盘物理增量测量。

## 计算当前还能写多少

设：

- `C` 为 `capacity_bytes`；
- `D` 为当前 `disk_queue_disk_bytes`；
- `F` 为当前 `disk_queue_filesystem_free_bytes`；
- `M` 为 `min_free_bytes`；
- `R` 为截断恢复预留空间。

当前代码中的恢复预留为：

```text
R = max(roundUp(2 * segment_bytes, filesystem_block_size), largest_existing_segment)
```

新数据可使用的物理预算是两项约束的较小值：

```text
B = max(0, min(C - R - D, F - M - R))
```

第一项防止 WAL 自己越过配置容量，第二项防止它吃掉同一文件系统上为其他工作负载保留的空间。只配一个很大的 `capacity_bytes` 而不配 `min_free_bytes`，不能防止整个 Volume 被写满。

`ingate_als_disk_queue_utilization_ratio` 展示的是原始 `D / C`。队列的 `warning`、`critical` 和 `blocked` 状态还会扣除 `R`，并检查文件系统的剩余余量，所以利用率看起来没有到 90% 时也可能进入 Critical 或 Blocked。运维判断应以状态和 `disk_queue_writable` 为准，原始比率只用于观察趋势。

## 估算故障缓冲时间

Kafka 完全不可写且请求速率稳定时，理论缓冲时间为：

```text
T_buffer ≈ B / (λ * b)
```

反过来，若目标是支撑 `T` 秒故障，至少需要：

```text
B_required ≈ λ_peak * b_pessimistic * T
```

这里应使用高峰 `λ` 和偏保守的 `b`，并额外留出告警响应、流量增长和测量误差余量。不要把默认 1 GiB 当成生产建议；它只是开发模式在未配置容量时的启动默认值。生产模式要求显式设置 `capacity_bytes`、正数 `min_free_bytes` 和 `sync: true`。

## 估算恢复时间

Kafka 恢复后，新请求仍会先进入 WAL，直到旧积压排空。若当前流量继续以 `λ` 进入，回放器以 `μ` Commit：

```text
net_drain = μ - λ
T_recovery ≈ pending_records / net_drain
```

只有 `μ > λ` 时队列才会收敛。若两者相等，最旧记录年龄不会下降；若 `μ < λ`，Kafka 已恢复也无法排空积压。Dashboard 的恢复时间若只用 `pending / μ`，在持续有新流量时会偏乐观。

实际的 `μ` 受以下路径共同限制：

- Kafka 的分区数、Broker 能力、ISR 状态和网络 RTT；
- `replay_batch_size` 及每条记录大小；
- franz-go 的分区批处理和 Zstd 压缩；
- 单个串行 Replayer；
- 每次成功发布后的 WAL 队首截断和文件同步。

当前没有并发回放。增加并发会引入乱序完成、确认空洞和更复杂的 Commit 位置；应先证明串行回放无法满足 `μ > λ_peak`，再考虑修改。

## 三层批次不要混为一谈

```text
Envoy batch       ALS QueueEntry       Kafka produce batch
1 秒或约 64 KiB -> 一次 Recv 对应一次 -> franz-go 按分区重新聚合
                   WAL 追加
```

Envoy 的 `buffer_size_bytes` 是软上限。Kafka 正常时，ALS 把每条记录交给 franz-go，客户端再按目标分区组批和压缩。Kafka 故障时，一次 `Recorder.Write` 形成一个 WAL 条目，并在生产模式执行一次同步追加。提高 Envoy 批次通常能减少 WAL fsync 次数，但会增加 Envoy 内存中的未发送窗口；这两个参数不能只看吞吐单独调整。

`replay_batch_size` 限制一次回放期望读取的记录数，不会拆开一个已落盘的 QueueEntry。若某条 QueueEntry 自身超过该值，Read 仍返回完整条目。因此单次回放的峰值内存和 Kafka 请求量还要受 Envoy 实际批次分布约束。

## 压测应分别覆盖正常、降级和恢复

一轮容量验证至少记录三段数据：

1. Kafka 正常：观察入口吞吐、Kafka 发布延迟和 CPU，确认 ALS 不限制业务请求日志处理。
2. Kafka 停止：观察 WAL 追加延迟、每条记录的物理增长 `b`、fsync 尾延迟和 Envoy 丢弃指标。
3. Kafka 恢复：在持续写入新请求的同时测量 `λ`、`μ`、WAL 物理字节和最旧条目年龄，确认队列能够净排空。

结果应保留负载模型、记录大小分布、Kafka 分区数、Broker 数、磁盘类型、`sync`、`segment_bytes` 和 `replay_batch_size`。脱离这些条件的单一“每秒多少条”没有复用价值。

## 配置和指标入口

- `configs/ingate-als.yaml`：本地默认配置
- `internal/als/data/diskqueue/capacity.go`：物理容量和恢复预留算法
- `internal/als/data/diskqueue/queue.go`：条目大小限制、追加、读取和 Commit
- `internal/als/server/disk_queue_replayer.go`：串行回放与退避
- `internal/als/metrics/prometheus.go`：容量、积压和处理计数指标
