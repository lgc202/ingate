---
title: WAL 与故障恢复
description: 从文件布局、fsync、截断协议和崩溃点解释 ALS 的本地故障队列
---

ALS 的 WAL 是 Kafka 暂时不可用时的本地持久队列。正常批次直接写 Kafka；一旦 Kafka 写入失败、Topic 不合规或队列已有积压，新批次就先追加到 WAL。Kafka 恢复后，后台任务从队首读取，成功发布到 Kafka，再删除已经确认的前缀。

```text
正常：RequestRecord -> Kafka

降级：RequestRecord -> WAL

恢复：WAL.Read -> Kafka.Publish -> WAL.Commit
```

WAL 解决的是**单个 ALS 实例在 Kafka 故障期间的有限缓冲**。它不复制到其他节点，不是 Kafka 的替代品，也不把 Envoy ALS 协议变成零丢失协议。

## WAL 与“预写”的含义

WAL 是 Write-Ahead Log，直译为“预写日志”。“预写”描述的是动作顺序：在执行一个可能失败、稍后才能确认的目标操作之前，先把待完成的意图写入可恢复日志。

对 ALS 回放而言，这个顺序是：

```text
1. 记录仍在 WAL
2. 发布到 Kafka
3. Kafka 全部确认
4. 从 WAL 删除
```

如果第 2 或第 3 步失败，记录仍在磁盘；如果第 4 步失败，记录会再次发布，形成可去重的重复。反过来，若先删 WAL 再发 Kafka，进程在两步之间退出就会确定丢失。

当前正常写入采用 Kafka-first。“预写”主要体现在**已经进入 WAL 的回放协议**：Kafka 直写失败后，ALS 先把原批次保存到 WAL，再等待后台重投。

## 普通文件追加缺少的能力

把 protobuf 调用 `os.WriteFile` 写到磁盘，只完成了“出现一个文件”。一个可恢复队列还需要回答：

- 新批次追加到哪里，如何避免覆盖旧数据；
- 进程崩溃后如何找到队首和队尾；
- 一个文件太大时如何分段；
- Kafka 确认后如何只删除连续前缀；
- 截断进行到一半时如何恢复；
- 如何识别半条记录、损坏记录和未知格式；
- 多个进程误用同一目录时由谁写；
- 容量满时是否删旧数据，还是拒绝新数据。

Ingate 使用 `tidwall/wal` 处理连续索引、分段日志、文件同步和队首截断；项目自己定义 `QueueEntry`、CRC、容量策略、锁、回放顺序和错误处理。使用开源库减少了底层文件协议代码，但并没有把可靠性设计交给库自动完成。

## 三层数据格式

理解磁盘内容时，需要把三层分开：

```text
RequestRecord protobuf
        ↓ repeated bytes
QueueEntry protobuf（Ingate 定义）
        ↓ length prefix + payload
tidwall/wal Binary entry（开源库定义）
        ↓ 文件系统分配块
WAL segment file（操作系统管理）
```

### 第一层：`RequestRecord`

每条请求记录独立编码为 protobuf bytes。单条编码结果不能超过 64 KiB。记录 ID 已经在进入 Queue 前生成，后续回放不会重新生成。

### 第二层：`QueueEntry`

一次 `Recorder.Write` 降级形成一个 `QueueEntry`：

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

| 字段 | 为什么存在 |
| --- | --- |
| `format_version` | 新版本遇到无法理解的旧格式时拒绝启动，避免错误解码 |
| `enqueued_at_unix_nano` | 恢复最旧积压年龄，支撑新鲜度 SLO 和告警 |
| `records` | 保存已经通过协议校验的完整批次和稳定 ID |
| `traceparent`、`tracestate` | 回放时用 Span Link 关联最初接收批次 |
| `crc32c` | 发现条目内容意外损坏 |

CRC32C 的计算过程为：复制条目、把 CRC 字段清零、执行确定性 protobuf 编码、再计算 Castagnoli 多项式校验和。

```go
checksumEntry := proto.Clone(entry).(*QueueEntry)
checksumEntry.Crc32C = 0
value, err := proto.MarshalOptions{Deterministic: true}.Marshal(checksumEntry)
checksum := crc32.Checksum(value, castagnoliTable)
```

确定性编码很重要。同一个 protobuf 对象若可能编码成不同字节序列，读取时重新计算 CRC 就可能误报。CRC 能检测随机损坏和大部分位翻转，不能防止有权限修改文件的人同时修改数据与 CRC；它不是签名或 MAC。

### 第三层：tidwall/wal Binary entry

`tidwall/wal` 的 Binary 格式不在每个条目里重复保存序号。每个 segment 的文件名给出首个序号，后续条目按文件内顺序隐式递增。单条物理编码是：

```text
[unsigned-varint payload_length][QueueEntry protobuf bytes]
```

例如 payload 长度为 300 字节，长度前缀可能占 2 字节，磁盘逻辑内容共 302 字节。文件系统通常按 4 KiB 等分配块记账，所以目录物理占用可能大于逻辑 payload。

segment 文件名是 20 位十进制首序号：

```text
00000000000000000001
00000000000000000427
00000000000000001015
```

固定宽度让文件名字典序与序号顺序一致。

## `Queue.Write` 的实际执行过程

主流程位于 `internal/als/data/diskqueue/queue.go`：

```text
检查 Context 和非空批次
  -> 获取 Queue.mu
  -> 编码 RequestRecord 与 QueueEntry
  -> 读取当前 LastIndex
  -> 探测目录物理占用、最大文件和文件系统空闲空间
  -> 计算新条目的最坏分配增长
  -> 容量准入
  -> wal.Write(last+1, value)
  -> 发布新的 pending 原子快照
```

只有 `wal.Write` 成功后，内存中的积压统计才会更新：

```go
if err := q.log.Write(last+1, value); err != nil {
	return fmt.Errorf("append disk queue: %w", err)
}
q.pending.Store(&nextPending)
```

内存快照晚于磁盘写入，因此进程如果恰好在两句之间退出，重启扫描会从磁盘重新得到正确积压。反过来先更新内存再写磁盘，会短暂报告一条实际不存在的持久记录。

## `write`、页缓存和 `fsync` 的区别

这是 WAL 最常见的深挖点。

### `write(2)` 通常只到内核页缓存

应用调用 `os.File.Write` 后，操作系统通常先把数据复制到内核 Page Cache，并标记为 dirty。调用返回只表示内核接受了数据，存储设备可能还没有持久化。

```text
Go heap -> write -> kernel page cache -> filesystem -> device cache -> physical media
```

如果只是进程崩溃，页缓存仍在内核里，数据通常还会继续刷盘；如果主机掉电或内核崩溃，尚未稳定化的 dirty page 可能丢失。

### `fsync` 请求稳定化文件内容

生产模式要求 `sync: true`，因此打开 tidwall/wal 时使用 `NoSync=false`。该库在 `Write` 返回前对当前 segment 调用 `File.Sync()`：

```go
if _, err := l.sfile.Write(bytes); err != nil {
	return err
}
if !l.opts.NoSync {
	if err := l.sfile.Sync(); err != nil {
		return err
	}
}
```

这使一次 WAL 追加的完成时间包含磁盘同步延迟。SSD 抖动、宿主机 I/O 竞争和网络 Volume 卡顿会直接反映为批次处理长尾。

`File.Sync` 仍然依赖文件系统、虚拟化层和硬件正确实现 flush。设备如果错误地提前报告写入完成，应用层无法仅靠 `fsync` 修复。WAL 的承诺应表述为“在受支持存储正确实现同步语义的前提下持久化”，不能表述成绝对不会丢。

### 文件同步不完全等于目录同步

文件内容和目录项是两个持久化对象。新建文件或 `rename` 之后，严格的掉电一致性通常还需要同步父目录，确保文件名到 inode 的映射也稳定化。

当前固定版本的 tidwall/wal 会同步临时文件，再执行 `rename`，但没有显式对父目录执行 `fsync`。因此：

- 进程崩溃与普通重启是主要覆盖场景；
- 突然掉电时的最终保证还取决于文件系统对 rename 和日志模式的实现；
- 不能把当前实现宣称为经过断电认证的存储引擎。

这是已知边界，不应通过文档措辞隐藏。若将来要求严格的掉电一致性，需要补充目录同步、故障注入和真实文件系统断电测试，或选用提供明确持久性契约的存储实现。

## 分段和轮转

WAL 不把所有条目放在一个无限增长文件中。当前配置通过 `segment_bytes` 设置目标 segment 大小。

tidwall/wal 写入时会：

1. 校验新序号必须紧跟当前 `LastIndex`；
2. 把长度前缀和 payload 追加到当前 segment 内存索引与文件；
3. segment 达到目标大小时，同步并关闭旧文件；
4. 创建以下一个序号开头的新 segment；
5. `NoSync=false` 时同步当前尾文件后返回。

`segment_bytes` 是轮转目标，不是绝对文件上限。最后一条记录可能把 segment 推过目标。Ingate 额外要求一个编码后的 QueueEntry 不超过 `segment_bytes`，因此越过量存在上界，也便于容量策略预留截断副本空间。

segment 带来的主要收益是：

- 确认大量旧记录时可以直接删除完整旧 segment；
- 启动和缓存不必把所有历史内容长期保留在同一对象中；
- 容量规划可以围绕有限大小的截断副本计算。

segment 过小会频繁创建、同步、关闭和删除文件；过大则会放大中段截断时的临时复制空间和延迟。该参数需要结合批次大小、故障时长和磁盘实测选择。

## `Read -> Publish -> Commit` 的三段确认

回放器只推进连续队首：

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

`Read` 不改变磁盘内容，只返回记录、最后序号和逻辑字节数。`Publish` 全部成功后，`Commit` 才调用 `TruncateFront(lastSequence+1)` 删除连续前缀。

| 顺序 | 故障时结果 |
| --- | --- |
| `Read -> Delete -> Publish` | 删除后、发布前退出会确定丢失 |
| `Read -> Publish -> Commit` | 发布后、Commit 前退出会重复 |

当前选择第二种，所以 WAL 回放是 At Least Once。重复保留原 `RequestRecord.id`，由 Analytics 和 ClickHouse 处理。

一个 QueueEntry 不会为满足 `replay_batch_size` 被拆开。若当前条目包含 800 条记录、回放限制为 500，本次仍返回完整 800 条。拆开条目需要为条目内部建立确认游标，会扩大持久格式和恢复协议；当前没有对应需求。

## `TruncateFront` 如何删除队首

删除完整旧 segment 很直接。复杂情况是确认位置落在某个 segment 中间：文件前半段已确认，后半段仍需保留。

tidwall/wal 使用 `.START` 协议：

```text
原文件 00000000000000000100
条目    [100][101][102][103][104]
提交到 102，下一条为 103

1. 将 [103][104] 写入 00000000000000000103.START.TEMP
2. Sync 临时文件
3. Rename 为 00000000000000000103.START
4. 删除旧前缀 segment
5. Rename 为 00000000000000000103
```

`atomicWrite` 先写 `.TEMP`、同步文件、关闭，再 rename 为 `.START`。`.START` 表示“新的队首内容已经准备好，但旧文件清理可能尚未完成”。

进程重启时，`wal.Open` 扫描目录：

- 发现 `.START`，删除它之前的旧 segment，再将 `.START` 改为正式文件名；
- `.START` 与 `.END` 同时存在则视为损坏；
- 普通 segment 会按文件名中的首序号排序并逐条解析。

这套协议允许截断包含多个步骤，同时让每个中间状态都能被下一次启动识别。由于前面提到的目录 `fsync` 边界，突然掉电的保证仍依赖具体文件系统。

## 崩溃发生在不同位置会怎样

| 崩溃点 | 磁盘或 Kafka 可能状态 | 重启后的处理 | 语义 |
| --- | --- | --- | --- |
| QueueEntry 编码前 | WAL 无本批 | 无法从 ALS 恢复 | 可能丢失 |
| segment `Write` 返回前 | 没有条目、完整条目或不完整尾部 | 完整条目可扫描；不完整尾部使启动失败 | 不静默跳过 |
| `File.Sync` 成功后、`Queue.Write` 返回前 | WAL 已有完整条目 | 启动扫描恢复 | 可能与上游重发重复 |
| WAL Read 后、Kafka Publish 前 | WAL 条目仍在 | 再次从队首发布 | 不丢 |
| Kafka 已追加、ACK 丢失 | Kafka 可能已有，WAL 仍在 | 后续重放同 ID | 可能重复 |
| Kafka 成功、Commit 前 | Kafka 已有，WAL 仍在 | 重启后重放同 ID | 可能重复 |
| `.START` 已同步、旧 segment 删除中 | 新队首副本与部分旧文件并存 | `wal.Open` 完成清理 | 可恢复的截断中间态 |
| Commit 成功后 | 已确认前缀删除 | 不再回放 | 完成 |

Envoy ALS 没有逐批 ACK，ALS 在生成 QueueEntry 前退出时，不能依赖 Envoy 一定重发该批。WAL 的可靠边界从 `Queue.Write` 成功开始，不覆盖尚未到达或尚未完整解析的日志。

## 尾部损坏与拒绝启动

Binary entry 依赖长度前缀确定 payload 边界。若崩溃留下：

```text
[length=500][只有 120 bytes...]
```

`tidwall/wal` 无法知道缺失部分是否本应存在，也无法把后面的随机字节安全解释为新条目。它会返回 `ErrCorrupt`。

即使底层 framing 完整，Ingate 还会检查：

- `format_version` 是否受支持；
- 入队时间是否有效；
- 条目是否至少有一条记录；
- CRC32C 是否匹配；
- 每条 `RequestRecord` 是否可解码且不超过大小限制。

任一未确认条目失败都会阻止 ALS 启动。自动截尾或跳过看似方便，但会在没有备份、审计和人工确认时直接做出“这些记录可以丢”的决定。当前做法选择显式失败，恢复步骤由运维文档和备份策略决定。

## 三把锁的保护范围

当前路径存在三层互斥，职责不同：

| 锁 | 范围 | 保护内容 |
| --- | --- | --- |
| 目录 `flock` | 进程之间 | 防止两个 ALS 进程同时打开同一 WAL 目录 |
| `Queue.mu` | Queue 实例内部 | 串行化 Write、Read、Commit 和物理容量探测 |
| tidwall/wal 内部 `RWMutex` | 库内部 | 保护 segment、索引、缓存和文件句柄 |

`Queue.mu` 不是底层库锁的无意义重复。它把“容量检查 -> 物理写入 -> pending 快照更新”组成一个业务临界区，并保证 Read/Commit 的队首假设不会被并发改写破坏。目录锁处理的是另一类错误：两个进程误配到同一目录。

目录权限为 `0700`，锁文件和 WAL 文件为 `0600`。`flock` 是本机文件锁，不应被当作跨节点分布式锁；每个 ALS 实例仍需独立持久目录。

## 磁盘追加脱离请求取消

Recorder 调用 Queue 时使用：

```go
r.queue.Write(context.WithoutCancel(ctx), records)
```

这批记录已经被 gRPC `Recv` 完整交给 ALS。Envoy stream 随后断开时，如果把取消直接传给 WAL，ALS 会在 Kafka 已失败的情况下再次主动放弃记录。`context.WithoutCancel` 保留 Trace 与日志所需的上下文值，但移除取消和 deadline。

代价也必须明确：文件系统长期卡住时，本次追加会拖慢 stream 收尾和进程关闭。当前优先保证已接收记录的落盘机会，最终停机上界由外部进程管理器控制。若存储经常无界卡顿，应修复 Volume 或隔离 I/O，而不是给 `fsync` 随意增加一个超时并假装底层写入已经停止。

## 按物理字节执行容量准入

protobuf payload 大小不等于 WAL 目录实际占用。物理占用还包括：

- QueueEntry 字段和 protobuf 长度；
- tidwall/wal 的 varint 长度前缀；
- 文件系统按 block 分配产生的取整；
- 当前 segment 的空洞与元数据；
- `TruncateFront` 临时复制的 `.START` 文件。

追加准入可概括为：

```text
disk_bytes + growth <= capacity_bytes - recovery_bytes
free_bytes - growth >= min_free_bytes + recovery_bytes
```

其中 `recovery_bytes` 取“两个配置 segment”与“现存最大旧文件”的较大值，再按文件系统 block 向上取整。配置缩小后，旧 segment 可能仍大于新值，所以不能只看当前 `segment_bytes`。

预留恢复空间的原因是：队列越接近满载，越需要通过 Commit 和截断释放空间；但中段截断恰好可能先复制一个 segment。若把磁盘最后一字节都交给新记录，恢复动作反而可能没有空间执行。

WAL 满时不会删除最旧记录。删除旧数据会破坏队首语义并静默制造分析缺口。当前策略拒绝新批次、返回非就绪并触发告警，等待 Kafka 恢复或人工扩容。

容量推导、净回放速率和压测方法见[容量与吞吐规划](./capacity/)。

## WAL 比直接写数据库快吗

没有脱离条件的答案。

顺序追加 WAL 通常具备这些优势：

- 写入模式接近连续 append，随机 I/O 少；
- 单条语义简单，不需要维护多索引和查询结构；
- 批次可以共用一次 `fsync`；
- 本地磁盘没有数据库网络 RTT。

数据库可能提供复制、事务、查询、压缩和成熟恢复能力，代价是协议往返、日志与数据页写放大、索引维护和远端依赖。ALS 使用 WAL，是为了在 Kafka 故障时保留一个不依赖网络的有限持久缓冲；具体性能仍取决于存储和负载条件。

生产判断需要实测：每次 QueueEntry 的记录数、`fsync` p95/p99、磁盘类型、文件系统、宿主机 I/O 竞争、segment 轮转和截断耗时。`sync=false` 的高吞吐结果不能代表生产持久模式。

## 当前保证与不保证

当前保证：

- `Queue.Write` 成功后，条目进入 tidwall/wal 的同步完成边界；
- 重启扫描恢复所有可解码的未确认条目和积压统计；
- 只有 Kafka 全部确认后才删除连续队首；
- Commit 失败最多引入同 ID 重复，不提前删除数据；
- 损坏和未知格式阻止启动，不静默跳过；
- 容量不足拒绝新记录，不覆盖旧记录。

当前不保证：

- Envoy 到 ALS 的零丢失；
- ALS 主机或 Volume 丢失后的恢复；
- 跨节点复制和自动主备切换；
- 对恶意文件篡改的检测；
- 所有文件系统上的严格断电一致性；
- 自动修复损坏 segment；
- 全局请求顺序或端到端 Exactly Once。

## 常见深挖点

### 一批记录对应一个 WAL 条目

一次 Envoy 批次共享一次落盘确认和 Trace Context，能减少 `fsync` 次数。代价是大批次放大单次延迟、内存和重复范围，因此上游批次大小与 `segment_bytes` 必须共同约束。

### Commit 失败与 Kafka 发布结果的关系

两者是不同持久系统。Kafka 可能已经确认，只是本地截断失败。重新发布会重复；跳过重发又可能在无法确认 Commit 状态时丢失。At Least Once 选择前者。

### 持久 WAL 与内存队列的差异

内存队列只能吸收短暂背压，进程退出和主机重启后积压消失。WAL 的用途正是跨进程生命周期恢复。

### 多实例不能共享当前 WAL

本地 segment、队首索引和 `flock` 都按单机所有权设计。共享存储需要分布式租约、所有权转移和并发确认协议，会把一个故障缓冲变成新的分布式日志系统。

### 损坏条目需要人工决策

跳过等价于删除尚未确认的数据。没有人工审批、备份和审计时，自动继续会让数据缺口难以发现。

## 源码入口

- `internal/als/data/diskqueue/entry.proto`：QueueEntry 持久协议
- `internal/als/data/diskqueue/entry.go`：编码、CRC 和 Trace Context
- `internal/als/data/diskqueue/queue.go`：追加、读取、提交和启动扫描
- `internal/als/data/diskqueue/capacity.go`：物理容量与恢复预留
- `internal/als/data/diskqueue/lock.go`：目录权限与进程排他锁
- `internal/als/server/disk_queue_replayer.go`：串行回放和退避

## 上游资料

- [tidwall/wal 固定版本源码](https://github.com/tidwall/wal/blob/4b09f9519cba/wal.go)
- [Go `os.File.Sync` 文档](https://pkg.go.dev/os#File.Sync)
- [Linux `fsync(2)` 手册](https://man7.org/linux/man-pages/man2/fsync.2.html)
- [Linux `rename(2)` 手册](https://man7.org/linux/man-pages/man2/rename.2.html)
