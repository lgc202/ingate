---
title: 并发与状态迁移
description: 用并发批次和 recorderState 代码解释 Kafka 故障屏障
---

ALS 可以同时接收多条 Envoy stream，多批 Kafka 写入也可能交错完成。Recorder 的并发约束不只是消除 data race，还要保证：一批写 Kafka 失败并开始落 WAL 后，新批次不能继续直写 Kafka，否则新记录会绕过 ALS 本地队列中的旧记录。

![Recorder 在直写、WAL 降级和回放暂停之间的状态迁移](/ingate/images/als/recorder-state.svg)

## 一个具体的竞争场景

假设 A 和 B 两个批次已经并发写 Kafka：

```text
A 取得 Kafka 准入 ── Kafka 超时 ── 写 WAL
B 取得 Kafka 准入 ──────── Kafka 成功
C 在 A 失败后到达 ───────────── 必须写 WAL
```

B 在故障屏障建立前已经取得准入，Recorder 不会取消它，也无法撤销 Kafka 可能已经接收的数据。C 在屏障后到达，必须与 A 一样进入 WAL。只有 A 的 WAL 追加、B 的 Kafka 写入和队列回放全部结束后，才能恢复直写。

这里保证的是单个 ALS 实例的准入顺序和 WAL 队首提交顺序，不是全局事件顺序。Kafka 会按记录 ID 选择分区，多实例 ALS、不同分区以及 Analytics 并发处理都可能改变最终可见顺序；业务查询必须使用 `started_at` 等显式时间字段排序。

## 准入点一次选定写入目标

`Recorder.Write` 调用 `reserveWriteTarget` 完成目标选择和在途计数：

```go
func (s *recorderState) reserveWriteTarget(topicCompliant bool) writeTarget {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.spooling || !topicCompliant {
		s.spooling = true
		s.queueWrites++
		return queueTarget
	}

	s.kafkaWrites++
	return kafkaTarget
}
```

`spooling` 是故障屏障。屏障存在或 Topic 不合规时，新批次不会先等待一次 Kafka 超时，而是直接记一个 `queueWrites` 并写 WAL。准入决定在锁内完成，Kafka 和磁盘 I/O 都在锁外执行。

## Kafka 失败同时建立屏障和预留 WAL 写入

```go
func (s *recorderState) failKafkaWrite() bool {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.kafkaWrites--
	s.kafkaOK = false
	switched := !s.spooling
	s.spooling = true
	s.queueWrites++
	return switched
}
```

失败分支在同一个临界区内完成四件事：结束 Kafka 在途操作、标记 Kafka 不可写、建立 `spooling`、为本批预留一个 WAL 在途写入。这使失败批次自己与它之后到达的批次都位于同一道屏障之后。

`failKafkaWrite` 返回的 `switched` 只用于把“首次进入降级”日志记一次，不参与后续投递决策。成功路径由更窄的 `completeKafkaWrite` 结束在途计数，两种迁移不再用布尔参数揉进同一个函数。

## 恢复直写的全部条件

```go
if !s.spooling || !topicCompliant || s.kafkaWrites > 0 || s.queueWrites > 0 ||
	s.replayPaused || !queueEmpty() {
	return false
}

s.spooling = false
return true
```

每个条件都对应一个具体的越过风险：

| 恢复条件 | 忽略后的结果 |
| --- | --- |
| Topic 合规 | 直写会立即被可预期地拒绝 |
| `kafkaWrites == 0` | 屏障前的 Kafka 写入结果还没有收敛 |
| `queueWrites == 0` | 已取得 WAL 准入的批次可能尚未落盘 |
| `replayPaused == false` | 队首的永久错误会被隐藏 |
| `queueEmpty()` | 新记录会越过旧积压 |

`queueEmpty` 在状态锁内调用，但只读取 Queue 发布的不可变原子快照，不会进行磁盘 I/O。

## Mutex 保护联合不变量

`spooling`、`kafkaWrites`、`queueWrites` 和 `replayPaused` 需要在一次迁移中联合读写。多个原子变量只能保证每个字段单独安全，无法保证组合快照来自同一个真实时刻。

`sync.RWMutex` 也没有明显收益。这些临界区只做布尔值和计数器更新，写操作占比高，读只来自探针和指标。一把普通 `sync.Mutex` 让不变量更容易阅读和测试。

Recorder 状态锁和 Queue 锁有不同责任：

| 锁 | 保护内容 | 不在锁内执行 |
| --- | --- | --- |
| `recorderState.mu` | 写入准入、故障屏障、在途计数 | Kafka 和磁盘 I/O |
| `Queue.mu` | WAL 的 Write、Read、Commit 和物理占用快照 | Kafka 调用 |

## 回放保持串行

DiskQueueReplayer 只有一个循环推进队首。回放成功后立即处理下一批；队列为空时等待下一个轮询周期；临时错误指数退避并添加随机抖动；永久错误暂停当前进程。

并发回放需要额外的确认区间、乱序完成和空洞跟踪。当前没有吞吐证据支持引入这些复杂度，因此保持串行，使 Commit 位置天然单调前进。

## 代码审查时检查的不变量

- `kafkaWrites` 和 `queueWrites` 不能为负数。
- `spooling=true` 后，新批次不能取得 Kafka 准入。
- Kafka 失败批次必须在建立屏障的同一次迁移中预留 WAL 写入。
- 队列未空或仍有在途写入时，不能恢复 Kafka 直写。
- `replayPaused=true` 时不能自动清除故障屏障。
- Recorder 状态锁和 Queue 锁都不能包住 Kafka 网络调用。

## 源码入口

- `internal/als/biz/recorder.go`：写入与回放主流程
- `internal/als/biz/recorder_state.go`：准入、屏障和恢复迁移
- `internal/als/data/diskqueue/queue.go`：队首顺序和积压快照
- `internal/als/server/disk_queue_replayer.go`：串行调度和退避
