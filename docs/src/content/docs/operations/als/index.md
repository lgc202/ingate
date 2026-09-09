---
title: ALS 运行模型
description: ALS 的可靠性边界、依赖关系、健康检查和故障时的运行方式
---

ALS 接收 Envoy 发送的 HTTP access log，转换为 Ingate 请求记录，再投递到 Kafka。Kafka 暂时不可写时，记录先进入降级 WAL，也就是本地持久队列，恢复后按队首顺序重放。

![ALS 运行链路，包含 Envoy、Kafka、磁盘队列、Analytics 和 ClickHouse](/ingate/images/als/architecture.svg)

## 先明确可靠性边界

Envoy 的 gRPC ALS 是异步日志出口，不在业务请求的响应路径中。Envoy 会把日志写入发送缓冲区，但协议没有逐批确认；网络中断、Envoy 进程退出或其发送缓冲区溢出时，记录仍可能在到达 Ingate ALS 之前丢失。

Ingate ALS 的可靠接收保证从“组件已经收到并通过校验的一批记录”开始：

- Kafka 确认成功，批次接收成功；
- Kafka 未确认，但 WAL 追加并同步成功，批次也接收成功；
- Kafka 和 WAL 都不能接收时，ALS 终止当前流并增加拒绝计数；
- 单条格式错误不会拖累同批的其他记录，它会被丢弃并计数。

这条链路采用至少一次投递。重复允许出现，确定丢失应被监控。它适合请求分析、容量观察和允许小误差的用量统计，不应单独充当需要逐笔守恒的财务账本。

## 依赖故障行为

| 状态 | 新记录去向 | `/readyz` | 业务流量 |
| --- | --- | --- | --- |
| Kafka 正常、WAL 可写 | Kafka | `200` | 不受影响 |
| Kafka 暂时不可达、WAL 可写 | WAL | `200`，`write_target=disk_queue` | 不受影响 |
| Topic 明确不符合契约、WAL 可写 | WAL | `503 topic_noncompliant` | Envoy 转发不受影响；持续故障会增加积压 |
| WAL 不可写或容量阻塞、Kafka 正常 | 仍可直写 Kafka | `503 wal_unavailable` | 当前记录可进入 Kafka，但实例已失去故障兜底能力 |
| WAL 不可写或容量阻塞、Kafka 失败 | 拒绝 | `503 wal_unavailable` | Envoy 转发不受影响，观测数据形成缺口 |
| 队首遇到永久 Kafka 错误 | 新记录仍进 WAL，回放暂停 | `503 replay_paused` | Envoy 转发不受影响，积压只增不减 |

即使 Kafka 正常，WAL 不可写也会让实例退出就绪状态。原因很直接：此时 Kafka 一旦失败，实例没有可靠的降级去向。`/readyz` 只是调度信号，gRPC server 不会主动拒绝绕过就绪检查建立的连接。`write_target=none` 表示调度系统不应再把新 ALS stream 送给该实例；已有 stream 会按 Recorder 的实时状态继续直写 Kafka、追加 WAL 或返回 `Unavailable`。

## 运行前检查

- 每个 ALS 实例使用独立、持久化的 WAL 目录；多个进程不能共享同一目录。
- 生产模式的 Kafka Topic 至少有 3 个副本，`min.insync.replicas` 至少为 2。
- WAL 所在文件系统必须有独立容量预算和告警，不要与无上限增长的日志共用空间。
- 上线前检查 `/readyz`、Dashboard 和告警规则；仅检查进程存活不足以判断采集能力。
- 备份与迁移 WAL 前先停止对应 ALS 实例，保留整个目录，不要只复制单个分段。

配置字段见[ALS 配置](./configuration/)，日常监控与故障处理见[SLO 与告警处置](./monitoring/)，磁盘队列操作见[恢复与迁移](./recovery/)。实现原理见[ALS 设计总览](../../development/architecture/als/)。
