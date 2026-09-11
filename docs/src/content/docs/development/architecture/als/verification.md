---
title: 验证与测量
description: 用单元测试、隔离故障演练和容量测量验证 ALS 的可靠性结论
---

可靠性结论需要对应可重复证据。源码可以证明控制流，单元测试可以控制罕见分支，故障演练可以验证真实进程和基础设施交互，性能测量则回答容量是否满足具体部署。

四类证据解决的问题不同：

| 证据 | 适合验证 | 无法单独证明 |
| --- | --- | --- |
| 源码审查 | 调用顺序、配置和依赖库行为 | 真实磁盘、网络和 Broker 时延 |
| 单元测试 | 并发状态、错误分支、边界值 | 容器重启和真实协议交互 |
| 本地故障演练 | Kafka/WAL/进程/观测组件的组合行为 | 多 Broker、跨可用区和生产吞吐 |
| 性能测量 | 指定机器与负载下的吞吐、延迟和容量 | 其他硬件或流量分布 |

仓库不会写入脱离环境的固定 TPS 数字。任何性能结论都应附带机器、磁盘、Kafka、批次和记录大小，否则无法复现，也不能用于容量承诺。

## 本地检查范围

修改 ALS 后按影响范围执行：

```bash
go test ./internal/als/...
go vet ./internal/als/...
make docs-build
```

涉及并发状态、后台任务停止或 Queue 时，可以补充：

```bash
go test -race ./internal/als/...
```

涉及 Kafka、WAL 持久恢复、探针或观测栈时，运行本地专项演练：

```bash
make als-e2e
```

`make als-e2e` 使用 Docker 创建隔离环境，完成后删除容器和 Volume，并将诊断日志保存到：

```text
_output/als-e2e/<run-id>.log
```

它是本地专项工具，不加入标准 CI。仓库约定本地也不运行 `make verify`；验证命令应按改动范围选择。

## 单元测试覆盖的语义

### Recorder 与并发状态

`internal/als/biz/recorder_test.go` 和 `recorder_state_test.go` 使用可控 Publisher/Queue，覆盖真实基础设施中很难稳定制造的交错：

- Kafka 成功时不写 Queue；
- Kafka 失败后原批次写 Queue；
- 多个 Kafka 写入并发完成时，故障屏障阻止新批次继续直写；
- 已取得准入的 Kafka 写入可以在屏障建立后完成；
- Queue 写入结束前不能恢复 Kafka 直写；
- Kafka 发布成功、Commit 失败时条目不移除；
- 临时错误进入重试，永久错误暂停队首；
- 只有 Topic 合规、在途计数归零、Queue 为空且未暂停时才能恢复。

这类测试验证的是状态机不变量，不要求启动 Kafka。测试失败时应先检查锁内迁移是否仍保持原子关系，不能通过增加等待时间掩盖竞争。

### Queue 持久格式和恢复

`internal/als/data/diskqueue` 的测试覆盖：

- QueueEntry 的确定性编码、版本和 CRC32C；
- Trace Context 写入和回放 Link；
- 单条记录、批次和 segment 大小边界；
- 写入、读取、Commit 及重新打开后的恢复；
- 中段损坏阻止扫描；
- 目录排他锁；
- 配置容量、文件系统空闲空间和恢复预留；
- 单个 entry 超过 `replay_batch_size` 时不被拆开。

这些测试使用真实临时目录和 tidwall/wal，但不能代替不同文件系统和存储设备上的断电实验。

### Kafka Publisher 与 Topic 契约

`internal/als/data/kafka` 和 `internal/als/biz/topic_test.go` 覆盖：

- protobuf Value、稳定 Key 和固定 Header；
- W3C `traceparent` / `tracestate` 注入；
- 逐条成功、失败结果汇总；
- temporary、uncertain、permanent 分类；
- ISR 相关错误计数；
- 生产模式的副本因子和 `min.insync.replicas` 契约；
- Topic 检查失败时保留最近一次有效缓存。

错误分类测试应使用 franz-go 和 Kafka 的真实错误值，避免自定义字符串与上游类型脱节。

### 协议与可观测性

`internal/als/service`、`internal/als/server`、`internal/als/metrics` 和 `internal/pkg/telemetry` 覆盖：

- HTTP access log 转换、无效记录和 TCP 日志处理；
- 每批独立 root Span；
- `/readyz` 的原因优先级和写入目标；
- Replayer 退避、日志抑制和生命周期停止；
- Counter、Gauge、Histogram 暴露；
- Trace Queue 满时 `Span.End` 不阻塞并增加丢弃计数。

单元测试验证“代码在指定输入下如何响应”，E2E 再验证这些行为能否通过容器、网络和后端被观察到。

## `make als-e2e` 的真实执行过程

专项演练定义在 `hack/als-e2e/run.sh`。它使用固定镜像启动：

```text
Envoy 1.39.0
Kafka 4.0.2
OpenTelemetry Collector 0.160.0
Prometheus 3.14.0
Loki 3.7.7
Tempo 2.10.7
```

所有二进制在临时目录构建，WAL、Kafka 和观测数据使用本次 Docker Project 独立 Volume。演练结束后，无论成功还是失败，都会先保存 `compose ps` 与完整容器日志，再清理环境。

### 场景一：Kafka 正常直写

步骤：

```text
发送请求 -> Envoy 生成 HTTP access log -> ALS -> Kafka
```

验证：

- Kafka 能按 Request ID 查询到记录；
- `/readyz` 返回 `write_target=kafka`；
- WAL `pending_records=0`。

这证明正常路径没有先写本地磁盘。

### 场景二：Kafka 故障、强制退出和恢复

步骤：

```text
停止 Kafka
  -> 重启 ALS，使 Topic 初始状态为 unknown
  -> 写入两批记录
  -> 确认 write_target=disk_queue 且存在积压
  -> SIGKILL ALS
  -> 启动 ALS
  -> 确认积压仍存在
  -> 恢复 Kafka
  -> 等待 TopicMonitor 和真实 Replayer
```

验证最终两条记录进入 Kafka，WAL 归零，写入目标恢复为 Kafka。使用 `SIGKILL` 可以绕过优雅关闭，证明恢复依赖磁盘内容，而不是关闭钩子中的内存状态。

### 场景三：不确定确认产生同 ID 重复

演练中的 `lostAckPublisher` 先让真实 Kafka Client 成功发布，再向 Recorder 返回人为注入的 `uncertain` 结果。Recorder 把原记录写 WAL，随后执行真实回放。

验证 Kafka 中至少出现两条满足以下条件的 message：

```text
message.key == record.id
decoded_record.id == record.id
```

这个场景不模拟某个特定网卡故障，而是精确控制“远端已执行、调用方未得到成功”这一语义。它证明外层重投保留稳定 ID。

### 场景四：WAL 满而业务流量继续

专项配置使用 1 MiB Queue 和 64 KiB segment，快速写入带 payload 的批次直至容量策略拒绝。

验证：

- 至少已有多个 entry 成功落盘；
- `/readyz` 返回 `reason=wal_unavailable`、`write_target=none`；
- 经过 Envoy 的 HTTP 请求仍然成功。

这证明 ALS 故障影响请求记录，不在 Envoy 同步转发结果中。

### 场景五：指标、日志和 Trace 关联

发送一条带本次运行唯一 marker 的无效记录，然后：

1. 在 Prometheus 查询 `up{job="ingate-als"}`；
2. 等待 `ALSRecordsDiscarded` 进入 pending 或 firing；
3. 在 Loki 按 marker 找到日志并提取 32 位 `trace_id`；
4. 使用该 Trace ID 查询 Tempo。

这验证埋点、规则、日志采集、Trace 导出和两种后端都可用。它不验证持续过载后的丢弃量，也不代表 Loki/Tempo 已具备生产高可用。

### 场景六：中段 entry 损坏

演练通过 tidwall/wal 公共 API 复制 Queue，在中间 QueueEntry 的正文翻转一个字节，再启动独立 ALS 进程。

验证进程非零退出，并且日志包含具体序号、CRC 或反序列化错误。它没有直接修改第三方 segment 私有 framing，验证的是 Ingate QueueEntry 自身的校验边界。

## 当前故障演练未覆盖的范围

| 未覆盖内容 | 原因 | 补充验证方式 |
| --- | --- | --- |
| 三 Broker 的真实 ISR 收缩和 Leader 切换 | 本地 E2E 使用单 Broker 开发契约 | 独立三 Broker 环境，逐台停止并查看 ISR |
| Broker 已落盘但 TCP ACK 丢失 | 稳定制造真实网络时间点成本高 | Toxiproxy/netem + Broker 日志；当前用语义注入验证 |
| 主机突然断电 | 容器 SIGKILL 不会清空宿主页缓存 | 裸机或虚拟机断电实验，检查文件系统与硬件 flush |
| Volume 整体丢失 | WAL 定义为单节点缓冲 | 从部署级备份与节点故障流程验证 |
| 长时间 Collector/Loki/Tempo 故障 | 当前演练只验证正常关联 | 分别停止组件直到队列满，检查丢弃和业务隔离 |
| Analytics 到 ClickHouse 的端到端新鲜度 | ALS 演练到 Kafka 为止 | Analytics 专项消费、重投和入库演练 |
| 生产吞吐与恢复时间 | 与机器和记录分布强相关 | 按后文的三阶段测量执行 |

明确未覆盖项比把本地 E2E 描述成生产证明更重要。每次增加可靠性承诺，都应先为相应故障层补充可执行验证。

## 三阶段性能测量

容量测量至少分成正常、降级和恢复三段。把三段混在一起，只能得到一个无法解释的平均值。

### 测量前固定环境

记录以下条件：

| 类别 | 必须记录的值 |
| --- | --- |
| ALS | commit、Go 版本、CPU/内存限制、实例数 |
| 输入 | 每秒请求数、并发 stream、每批记录数分布、记录字节 p50/p95/p99 |
| Kafka | 版本、Broker 数、Partition 数、副本因子、最小 ISR、磁盘、网络 RTT |
| WAL | 文件系统、Volume 类型、`sync`、segment、capacity、min free、replay batch |
| 观测 | Trace 采样率、Collector 是否启用、Prometheus 抓取周期 |

记录大小必须包含正常 API、流式 AI、错误响应等实际分布。只用一条最小 GET 记录会低估 protobuf、Kafka Batch 和 WAL 容量。

### 阶段一：Kafka 正常

Kafka 可用且 WAL 为空时持续施压，记录：

```text
input_rate        = rate(records_valid_total)
kafka_accept_rate = rate(records_kafka_accepted_total)
publish_p95/p99   = histogram_quantile(...kafka_publish_seconds_bucket)
batch_p95         = histogram_quantile(...batch_records_bucket)
CPU / RSS / heap / goroutines
Kafka request latency, batch size and compression ratio
```

目标是找出持续吞吐、发布长尾和第一个饱和资源。输入速率高于接受速率且 franz-go 缓冲接近上限时，增加请求只会放大等待和超时。

### 阶段二：Kafka 完全不可写

停止 Kafka 或隔离网络，保持同样输入，记录：

```text
spool_rate         = rate(records_spooled_total)
physical_growth    = delta(disk_queue_disk_bytes) / delta(records_spooled_total)
WAL append latency = batch processing latency after entering disk path
filesystem free bytes
Envoy logs_dropped and ALS active streams
```

根据实测每条物理增长 `b` 与高峰输入 `λ` 计算：

```text
T_buffer ≈ writable_capacity / (λ * b)
```

若 `sync=true` 下 batch processing p99 明显上升，需要检查每批记录数和 `fsync` 长尾。不能用 `sync=false` 的结果推导持久模式容量。

### 阶段三：Kafka 恢复且新流量继续

恢复 Kafka，同时保持输入，记录：

```text
arrival_rate   = rate(records_spooled_total)
commit_rate    = rate(records_committed_total)
net_drain      = commit_rate - arrival_rate
oldest_age     = disk_queue_oldest_entry_age_seconds
pending        = disk_queue_records
```

只有 `net_drain > 0` 时存在有限恢复时间：

```text
T_recovery ≈ pending / net_drain
```

同时观察 oldest age 是否下降。短窗口内 Commit 可能呈批量跳变，只看瞬时 rate 容易误判，应使用足够覆盖多个回放批次的窗口。

## 结果记录格式

一次可比较的结果至少包含：

```text
环境标识与 commit
测试开始/结束时间
输入模型与记录大小分位数
Kafka 和 WAL 配置

正常阶段：最大稳定 input rate、publish p50/p95/p99、CPU、RSS
降级阶段：spool rate、每条物理增长、append p95/p99、估算缓冲时间
恢复阶段：arrival rate、commit rate、net drain、实际排空时间

错误数：temporary / uncertain / permanent / rejected / discarded
诊断文件：Prometheus snapshot、Kafka topic describe、容器日志
```

“最大稳定”需要事先定义，例如连续 30 分钟无 rejected、WAL 不增长、p99 未越过目标、CPU 与内存仍有余量。没有稳定条件的峰值只能表示短时冲高。

仓库当前没有提交一组机器相关基线，也不应把开发机结果写成默认生产能力。正式容量结论应在目标部署规格上执行上述测量，并将原始环境和查询一并保留。

## 从结果回到设计

| 观察结果 | 优先检查 | 不应立即采取的动作 |
| --- | --- | --- |
| Kafka 正常时发布 p99 高 | Broker RTT、ISR、批次、压缩和缓冲 | 直接关闭 `acks=all` |
| WAL append p99 高 | `fsync`、批次大小、磁盘争用 | 改成 `sync=false` 后宣称问题解决 |
| 恢复时 `net_drain <= 0` | Partition、Broker、回放 batch、单 Replayer | 直接并发 Commit 同一 Queue |
| WAL 物理增长远大于 payload | batch 过小、block 取整、segment 轮转 | 只按 protobuf 大小配容量 |
| Trace drop 增长 | 采样率、Exporter、Collector 队列和 Tempo | 扩大成无界队列 |
| `rejected` 增长 | Kafka 与 WAL 同时失败的时间点 | 把整批 rejected 当成精确丢失量 |

调整配置后应重复同一负载模型，比较发布长尾、物理增长、净排空和拒绝记录。只比较平均耗时，无法判断尾延迟和故障容量是否改善。

## 证据入口

- `hack/als-e2e/run.sh`：隔离故障演练的实际步骤
- `hack/als-e2e/kafka.go`：真实 Kafka 查询和不确定确认注入
- `hack/als-e2e/check.go`：Readiness、Prometheus、Loki 与 Tempo 验证
- `hack/als-e2e/server.go`：中段损坏构造
- `internal/als/biz/*_test.go`：Recorder 与状态迁移
- `internal/als/data/diskqueue/*_test.go`：持久格式、容量和恢复
- `internal/als/data/kafka/*_test.go`：消息协议与错误分类
- `internal/als/server/*_test.go`：回放、Topic 检查和探针
- `internal/pkg/telemetry/*_test.go`：Trace 缓冲和日志身份
