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
- Kafka 没有确认整批且 WAL 追加失败时，ALS 终止当前流并增加拒绝计数；
- 单条格式错误不会拖累同批的其他记录，它会被丢弃并计数。

这条链路采用至少一次投递。重复允许出现，未取得同步持久化确认的批次会被监控。拒绝计数不是精确丢失量：Kafka 部分成功或确认结果不确定、随后 WAL 又失败时，整批都会计为拒绝。它适合请求分析、容量观察和允许小误差的用量统计，不应单独充当需要逐笔守恒的财务账本。

## 依赖故障行为

| 状态 | 新记录去向 | `/readyz` | 业务流量 |
| --- | --- | --- | --- |
| Kafka 正常、WAL 可写 | Kafka | `200` | 不受影响 |
| Kafka 暂时不可达、WAL 可写 | WAL | `200`，`write_target=disk_queue` | 不受影响 |
| Topic 明确不符合契约、WAL 可写 | WAL | `503 topic_noncompliant` | Envoy 转发不受影响；持续故障会增加积压 |
| WAL 不可写、没有积压且未处于降级状态、Kafka 正常 | 已有 stream 仍可直写 Kafka | `503 wal_unavailable` | 当前记录可能进入 Kafka，但实例已失去故障兜底能力 |
| WAL 不可写、已有积压或正处于降级状态，Kafka 即使已恢复 | 继续尝试 WAL，失败后拒绝 | `503 wal_unavailable` | 新记录不能越过本地积压直写 Kafka |
| WAL 不可写、没有积压且未处于降级状态，Kafka 本次写入失败 | 降级写 WAL 失败后拒绝 | `503 wal_unavailable` | Envoy 转发不受影响，观测数据存在缺口风险 |
| 队首遇到永久 Kafka 错误 | 新记录仍进 WAL，回放暂停 | `503 replay_paused` | Envoy 转发不受影响，积压只增不减 |

即使 Kafka 正常，WAL 不可写也会让实例退出就绪状态。原因很直接：此时 Kafka 一旦失败，实例没有可靠的降级去向。`/readyz` 只是调度信号，gRPC server 不会主动拒绝绕过就绪检查建立的连接。`write_target=none` 表示调度系统不应再把新 ALS stream 送给该实例，不是 Recorder 已有 stream 的强制路由结果；已有 stream 会按实时状态直写 Kafka、追加 WAL 或返回 `Unavailable`。

## 直接查看当前写入目标

在 ALS 实例所在网络执行：

```bash
curl -sS -i http://als:18092/readyz
```

Kafka 暂时不可用、WAL 仍可写时，返回 HTTP 200：

```json
{
  "status": "ready",
  "write_target": "disk_queue",
  "queue_state": "healthy",
  "queue_writable": true,
  "pending_records": 1240,
  "pending_bytes": 816920
}
```

这表示记录正在积压，但 ALS 仍有可写的持久去向。值班人员需要恢复 Kafka，并观察 `pending_records` 和最旧条目年龄，无需重启 ALS，也不能删除 WAL。

WAL 不可写时，返回 HTTP 503：

```json
{
  "status": "unavailable",
  "reason": "wal_unavailable",
  "write_target": "none",
  "queue_state": "blocked",
  "queue_writable": false,
  "pending_records": 1240,
  "pending_bytes": 816920
}
```

这表示实例已失去 Kafka 故障时的持久降级能力。先查看 WAL 所在文件系统的剩余空间和 I/O 错误，再按[ALS SLO 与告警处置](./monitoring/)中的 `ALSDiskQueueBlocked` 流程处理。

## 运行前检查

- 每个 ALS 实例使用独立、持久化的 WAL 目录；多个进程不能共享同一目录。
- 生产模式的 Kafka Topic 至少有 3 个副本，`min.insync.replicas` 至少为 2。
- WAL 所在文件系统必须有独立容量预算和告警，不要与无上限增长的日志共用空间。
- 上线前检查 `/readyz`、Dashboard 和告警规则；仅检查进程存活不足以判断采集能力。
- 备份与迁移 WAL 前先停止对应 ALS 实例，保留整个目录，不要只复制单个分段。

配置字段见[ALS 配置](./configuration/)，日常监控与故障处理见[SLO 与告警处置](./monitoring/)，磁盘队列操作见[恢复与迁移](./recovery/)。实现原理见[ALS 设计总览](../../development/architecture/als/)。
