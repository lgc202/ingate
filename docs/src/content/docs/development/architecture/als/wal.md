---
title: WAL 与故障恢复
description: 通过实际持久格式和 Read-Publish-Commit 代码解释 ALS 的本地故障队列
---

ALS 的 WAL 是 Kafka 故障期间的本地持久队列。正常批次直接写 Kafka，不会先写一次磁盘。进入降级状态后，新批次先追加到 WAL；Kafka 恢复后，回放器按队首顺序投递。

“预写”指这个顺序：先保存尚未完成的 Kafka 投递意图，再投递，最后删除已确认条目。直接把 protobuf 写成普通文件，仍然需要自行解决追加原子性、索引、分段、确认位置和截断。Ingate 用 `tidwall/wal` 处理这些底层问题，业务条目格式和恢复策略仍由项目自己控制。

![ALS WAL 条目的编码、校验、追加和确认结构](/ingate/images/als/wal-entry.svg)

## 一个落盘批次对应一个 WAL 条目

持久格式定义在 `internal/als/data/diskqueue/entry.proto`：

```proto
message QueueEntry {
  uint32 format_version = 1;
  int64 enqueued_at_unix_nano = 2;
  repeated bytes records = 3;
  string traceparent = 4;
  string tracestate = 5;
  fixed32 crc32c = 6;
}
```

| 字段 | 解决的问题 |
| --- | --- |
| `format_version` | 防止新代码把无法理解的旧数据当成正常记录 |
| `enqueued_at_unix_nano` | 计算最旧积压已经等待多久 |
| `records` | 保留已校验的 `RequestRecord` 及稳定 ID |
| `traceparent` / `tracestate` | 让后续回放 Trace 链接到当初的接收批次 |
| `crc32c` | 检测条目内容的意外损坏 |

CRC32C 的输入是“先将 `crc32c` 清零，再确定性编码”后的整个 `QueueEntry`：

```go
checksumEntry := proto.Clone(entry).(*QueueEntry)
checksumEntry.Crc32C = 0
value, err := proto.MarshalOptions{Deterministic: true}.Marshal(checksumEntry)
if err != nil {
	return 0, fmt.Errorf("marshal queue entry checksum input: %w", err)
}
return crc32.Checksum(value, castagnoliTable), nil
```

校验和版本号能发现损坏或不兼容数据，不能防止有权修改目录的攻击者篡改数据。目录权限和宿主机安全仍是信任边界。

## 追加成功的含义

队列用下面的选项打开 WAL：

```go
queueLog, err := wal.Open(path, &wal.Options{
	NoSync:           !config.GetSync(),
	SegmentSize:      int(config.GetSegmentBytes()),
	LogFormat:        wal.Binary,
	SegmentCacheSize: 2,
	AllowEmpty:       true,
	DirPerms:         directoryMode,
	FilePerms:        fileMode,
})
```

生产模式强制 `sync: true`，因此 `NoSync=false`。`Queue.Write` 只在 `q.log.Write` 返回成功后更新内存中的积压快照：

```go
if err := q.log.Write(last+1, value); err != nil {
	return fmt.Errorf("append disk queue: %w", err)
}
q.pending.Store(&nextPending)
```

这个顺序保证返回成功的批次已经进入 WAL 的持久边界。它能覆盖进程崩溃和多数主机重启，无法覆盖 Volume 丢失、文件系统损坏或硬件错误报告 flush 成功。WAL 是单节点故障缓冲，不是复制存储。

Recorder 在调用 Queue 时会移除 stream 的取消信号：

```go
err := r.queue.Write(context.WithoutCancel(ctx), records)
```

这批记录已经被 gRPC `Recv` 完整交给 ALS。若 Envoy 随后断开 stream，直接把该取消传给磁盘追加会在最需要降级保护时放弃记录。`context.WithoutCancel` 保留 Trace 和日志使用的上下文值，同时移除取消和 deadline。这次本地追加因此没有请求级超时；若底层文件系统长时间卡住，它会拖慢 stream 收尾和进程关闭，最终只能由外部进程管理器终止。这是选择“已接收记录优先落盘”带来的代价。

## Read、Publish 和 Commit 的顺序

回放器每次只处理连续的队首区间。下面的节选保留了 `ReplayBatch` 中决定数据语义的三步：

```go
batch, err := r.queue.Read(ctx, limit)

result := r.publisher.Publish(ctx, batch.Records)
if result.Err != nil {
	return ReplayRetry, fmt.Errorf("write queued records: %w", result.Err)
}

if err := r.queue.Commit(ctx, batch); err != nil {
	return ReplayRetry, fmt.Errorf("commit disk queue: %w", err)
}
```

`Read` 只返回数据和最后序列号，不删除条目。`Publish` 失败后，原条目仍在队首。只有 Kafka 全部确认后才执行 `Commit`，`Commit` 通过 `TruncateFront` 删除连续前缀。

一个 WAL 条目不会为迎合 `replay_batch_size` 而拆开。如果已存条目含 800 条记录，回放上限是 500，本次仍返回完整的 800 条。这避免一个业务条目被拆成多个独立确认位置。

![Kafka 确认丢失与 WAL Commit 失败产生重复的时序](/ingate/images/als/lost-ack.svg)

| 退出位置 | 重启后的结果 |
| --- | --- |
| WAL 追加前 | ALS 尚未持久化本批，Envoy 协议也不保证逐批重发 |
| WAL 同步后、`Write` 返回前 | 重启扫描可找回条目，可能与上游重发重复 |
| Kafka 写入前 | 条目留在队首，重启后首次投递 |
| Kafka 成功后、Commit 前 | 条目再次投递，记录 ID 不变 |
| Commit 成功后 | 条目已删除，不再回放 |

这个确认顺序选择了“故障时可能重复”，以避免“先删队列、后发 Kafka”造成确定丢失。

## 启动扫描和损坏处理

`NewQueue` 打开目录后扫描所有未确认条目，每条都执行以下检查：

- `format_version` 等于当前版本；
- 入队时间有效；
- 至少包含一条记录；
- CRC32C 匹配；
- 每条 `RequestRecord` 能解码且不超过 64 KiB。

任意条目失败都会阻止 ALS 启动。程序不会跳过、截断或删除损坏数据，因为这些动作都会代替运维人员做数据丢失决策。

目录同时持有排他锁，权限为 `0700`，锁文件和 WAL 文件为 `0600`。每个 ALS 实例必须使用独立持久目录；这把锁不提供网络文件系统的分布式互斥。

## 容量按物理占用计算

`capacity_bytes` 限制的是目录已分配的物理字节，不是 protobuf 载荷之和。追加前的准入条件可概括为：

```text
disk_bytes + growth <= capacity_bytes - recovery_bytes
free_bytes - growth >= min_free_bytes + recovery_bytes
```

`recovery_bytes` 取“两个配置分段”和“现存最大旧分段”的较大值。`tidwall/wal` 截断队首时可能先写一份分段副本；不预留这部分空间，队列越接近容量上限，越可能无法通过截断释放空间。

物理占用达到可写上限的 80% 和 90% 时，状态分别进入 `warning` 和 `critical`。无法容纳下一个最小分配块时进入 `blocked`。队列不会删旧数据换空间；回放仍可以推进，空间释放后新追加自动恢复。

## 源码入口

- `internal/als/data/diskqueue/queue.go`：追加、读取、确认和启动扫描
- `internal/als/data/diskqueue/entry.go`：条目编码、版本和 CRC32C
- `internal/als/data/diskqueue/capacity.go`：物理容量准入
- `internal/als/server/disk_queue_replayer.go`：串行回放和退避

## 上游资料

- [tidwall/wal 源码与使用说明](https://github.com/tidwall/wal)
- [Go `File.Sync` 文档](https://pkg.go.dev/os#File.Sync)
