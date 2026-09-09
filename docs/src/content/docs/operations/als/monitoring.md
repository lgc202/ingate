---
title: ALS SLO 与告警处置
description: ALS 可靠接收、新鲜度目标以及每条告警的确认、恢复和验证步骤
---

ALS Dashboard 位于 Grafana 的 **Ingate / Ingate ALS**。它只使用实例等低基数标签，不按 Gateway、Route、Request ID 等产品维度拆分系统指标。

开始值班前先阅读 [ALS 运行模型](../)。配置项和生产基线见 [ALS 配置](../configuration/)，磁盘队列操作见[恢复与迁移](../recovery/)。

## 服务目标

| 目标 | SLI | 30 天目标 |
| --- | --- | --- |
| 可靠接收 | `1 - rejected / valid`；写入 Kafka 或持久化 WAL 均视为成功接收 | 99.99% |
| Kafka 新鲜度 | 每分钟检查最老 WAL 记录是否不超过 5 分钟；空队列视为满足，ALS 抓取失败视为不满足 | 99% |

可靠接收告警使用两组多窗口预算消耗规则：5 分钟与 1 小时同时超过 14.4 倍预算时报告 Critical；30 分钟与 6 小时同时超过 6 倍预算时报告 Warning。短窗口负责尽快发现突发故障，长窗口过滤瞬时噪声。零流量时记录规则得到 1；可靠接收规则没有像新鲜度规则一样把抓取失败显式算作坏样本，缺失序列会成为无数据。SLO 按实例记录，不代表 Envoy 到 ClickHouse 的全链路完整率。

Kafka 新鲜度只衡量记录进入 Kafka 前的 WAL 延迟。WAL 为空时，Analytics 或 ClickHouse 仍可能滞后；产品页面的端到端可见性需要结合 Analytics 消费延迟和 ClickHouse 写入状态判断。

## 常用查询

```text
# ALS 已经收到并通过校验，但 Kafka 与 WAL 都未接收
increase(ingate_als_records_rejected_total[5m])

# Kafka 故障期间的积压与最老等待时间
ingate_als_disk_queue_records
ingate_als_disk_queue_oldest_entry_age_seconds

# 回放进度
rate(ingate_als_records_replayed_total[5m])
rate(ingate_als_records_committed_total[5m])

# 粗略排空时间；没有 Commit 速率时下限钳制为 0.001 条/秒
ingate_als_disk_queue_records
  / clamp_min(rate(ingate_als_records_committed_total[5m]), 0.001)
```

`ingate_als_records_received_total` 在批次处理结束时按 Envoy 消息中的记录数增加，包含随后被丢弃的记录。`valid` 只统计成功解析并进入 Recorder 的记录。二者可以用来排查协议质量，但批次处理异常中断和采样时点会让短窗口不严格守恒。

就绪原因码按固定顺序选择：`topic_noncompliant` 优先于 `replay_paused`，后者优先于 `wal_unavailable`。多个故障并存时，先处理返回的原因，再检查同实例的全部告警。

Compose 使用 `/readyz` 作为 ALS healthcheck。连续失败会把容器标为 unhealthy，但 `restart: unless-stopped` 不会仅因为 healthcheck 失败自动重启；Envoy 的 `depends_on` 只影响初次启动顺序。运行中的 Envoy 和已有 ALS stream 仍需通过指标与告警判断。

## 通用确认顺序

1. 在 Dashboard 选择告警中的 `instance`，确认接收速率、WAL 积压、最老记录年龄和 Kafka 状态。
2. 在 Alertmanager 查看同实例的关联告警，再通过 `docker compose logs als kafka` 检查首次错误；不要只依赖最后一条重试日志。
3. 先恢复 Kafka 或磁盘，再观察 `committed` 速率和预计排空时间。不要直接删除 WAL。

## ALSAvailabilityBudgetBurnCritical

- **影响：** 可靠接收错误预算正在快速消耗，继续持续会违反 99.99% 月度目标。
- **确认：** 检查 5 分钟和 1 小时错误率，并确认 `ALSRecordsRejected`、`ALSDiskQueueBlocked` 是否同时触发。
- **常见原因：** Kafka 不可写且 WAL 已满、文件系统不可写，或 ALS 实例持续重启。
- **恢复：** 优先恢复 WAL 写入能力，再恢复 Kafka；若多实例部署，只摘除无法可靠接收的实例。
- **验证：** 两个窗口的错误率均回落，`rejected` 不再增长，WAL 能持续提交。

## ALSAvailabilityBudgetBurnWarning

- **影响：** 可靠接收在较长时间内持续退化，尚未形成快速故障但会耗尽月度预算。
- **确认：** 检查 30 分钟和 6 小时错误率，按实例比较是否集中在单机。
- **常见原因：** Kafka 间歇超时、磁盘空间反复触线、实例资源长期不足。
- **恢复：** 修复持续抖动的依赖或实例；若 WAL 接近容量边界，先恢复 Kafka 消费积压再调整容量。
- **验证：** 两个窗口都低于 6 倍预算消耗速率，且不再产生新的 Critical 告警。

## ALSRecordsRejected

- **影响：** 已通过协议校验的请求记录未进入 Kafka 或 WAL，形成确定的数据缺口。
- **确认：** 检查 `rejected` 增量、`disk_queue_writable`、`kafka_writable` 和同一时刻的 ALS 日志。
- **常见原因：** Kafka 故障期间 WAL 达到容量限制、文件系统空间不足或 WAL I/O 失败。
- **恢复：** 恢复 WAL 所在文件系统写入能力或 Kafka；保留现有 WAL，避免扩大数据缺口。
- **验证：** 新请求不再增加 `rejected`，`readyz` 恢复，Kafka 或 WAL 的成功计数继续增长。

## ALSRecordsDiscarded

- **影响：** 非 HTTP、字段不完整或格式不受支持的访问日志未进入可靠投递链路。
- **确认：** 查看 `discarded` 增量和 ALS 警告日志中的数量、Envoy 节点 ID；日志不会包含完整请求内容。
- **常见原因：** Envoy ALS 配置错误、协议版本不匹配，或数据面意外发送 TCP 日志。
- **恢复：** 修正 Envoy access log 配置并确认控制面发布版本；不要在 ALS 中放宽必要字段校验。
- **验证：** 发送一批正常 HTTP 请求，`received` 与 `valid` 同步增长且 `discarded` 保持不变。

## ALSDiskQueueBlocked

- **影响：** ALS 已失去 Kafka 故障时的持久化兜底能力；后续 Kafka 失败可能立即产生拒绝。
- **确认：** 检查 WAL 状态、利用率、文件系统剩余字节和安全余量，再查看是否存在 I/O 错误。
- **常见原因：** WAL 达到配置容量、宿主机磁盘接近耗尽、目录只读或底层存储故障。
- **恢复：** 先恢复 Kafka 让回放释放队首；必要时扩容同一 Volume 或迁移到健康磁盘。不得手工删除 WAL 分段。
- **验证：** `disk_queue_writable` 回到 1，`readyz` 恢复，积压记录按顺序提交并最终归零。

## ALSKafkaUnavailable

- **影响：** 新记录转入 WAL，持续时间过长会消耗本地容量并降低 Kafka 新鲜度。
- **确认：** 检查 Broker 健康、Topic 契约、发布失败分类和 WAL 增长速度。
- **常见原因：** Broker 不可达、认证或 TLS 错误、Topic 不合规、请求超时。
- **恢复：** 恢复 Broker 与网络；若 Topic 契约不合规，修复副本数和 `min.insync.replicas` 后等待最多一分钟的合规检查刷新。若 `ingate_als_replay_paused` 已为 1，还要在修复后重启 ALS；合规检查恢复不会自动清除暂停状态。
- **验证：** `kafka_writable` 回到 1，`replayed` 与 `committed` 持续增长，预计排空时间下降。

## ALSKafkaISRInsufficient

- **影响：** Kafka 无法满足 all-ISR 确认，ALS 会将整批记录写入 WAL 并可能产生重复投递。
- **确认：** 查看 Topic 的 ISR、Broker 存活状态以及 `kafka_isr_failures_total` 增量。
- **常见原因：** 副本 Broker 离线、复制延迟过大，或 `min.insync.replicas` 与实际副本状态不匹配。
- **恢复：** 恢复缺失副本并等待 ISR 回归；不要通过降低确认等级绕过可靠性契约。ISR 是运行时集合，不由 Topic 合规检查器缓存。
- **验证：** Topic ISR 满足契约，ISR 错误不再增长，Kafka 直写和 WAL 回放恢复。

## ALSReplayPaused

- **影响：** 队首遇到永久错误后自动回放停止，后续记录不能越过队首提交。
- **确认：** 检查首次永久发布错误、Topic 配置和队首记录编码版本。
- **常见原因：** Topic 或认证配置不可恢复、消息大小限制不兼容、队首数据格式无法发布。
- **恢复：** 修复 Kafka 或配置后重启 ALS 重新评估队首；若确认 WAL 数据损坏，先保全 Volume，再按[恢复与迁移](../recovery/)处理。当前没有受支持的就地修复命令。
- **验证：** `replay_paused` 回到 0，`committed` 增长，队列条目数持续下降。

## ALSReplayStalled

- **影响：** Kafka 已可写但 WAL 长时间没有提交，积压和最老记录年龄继续增长。
- **确认：** 对比 `replayed`、`committed`、回放退避和 Kafka 发布延迟，检查是否只有本地 Commit 失败。
- **常见原因：** WAL 读取或截断 I/O 错误、Kafka 间歇失败、回放循环异常退出。
- **恢复：** 修复磁盘或 Kafka 后重启受影响实例；不要启动第二个进程并发打开同一 WAL。
- **验证：** `committed` 重新增长，预计排空时间持续下降，告警恢复后积压最终归零。

## ALSTraceDropped

- **影响：** 业务投递不受影响，但部分 ALS 故障链路无法在 Tempo 中追踪。
- **确认：** 检查进程内丢弃计数、Collector 指标、OTLP 连通性和 Collector 出口队列。
- **常见原因：** Collector 停止、Tempo 写入缓慢、Trace 采样量超过进程内有界队列处理能力。
- **恢复：** 恢复 Collector 或 Tempo；持续过载时先降低 `sample_ratio`，再评估 Collector 容量。
- **验证：** 丢弃计数不再增长，新 Trace 可从 Tempo 跳转到相同 `trace_id` 的 Loki 日志。
