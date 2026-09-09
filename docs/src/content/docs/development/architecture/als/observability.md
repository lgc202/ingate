---
title: 可观测性与验证
description: ALS 的日志、指标、Trace、SLO 以及端到端故障验证方法
---

ALS 有两类观测数据。请求记录是提供给 Ingate 用户的产品数据；日志、Prometheus 指标和 Trace 用来判断 ALS 自己是否工作正常。两类数据走不同链路，系统观测后端故障不能反过来阻塞请求记录投递。

## 处理阶段与指标

| 指标 | 含义 |
| --- | --- |
| `ingate_als_records_valid_total` | 已通过协议校验并进入可靠投递流程 |
| `ingate_als_records_kafka_accepted_total` | Kafka 已确认，包括回放及可能重复的发布 |
| `ingate_als_records_spooled_total` | 已成功追加到 WAL |
| `ingate_als_records_replayed_total` | WAL 记录已成功发布到 Kafka，可能仍未 Commit |
| `ingate_als_records_committed_total` | 已从 WAL 确认删除 |
| `ingate_als_records_rejected_total` | Kafka 与 WAL 都未能接收 |
| `ingate_als_records_discarded_total` | 协议边界发现的不完整或不支持记录 |

这些计数不能简单相减得到严格守恒式。一次 Kafka 部分成功后整批进入 WAL，会同时增加 Kafka accepted 和 spooled；Commit 失败后的重放也会增加 accepted 与 replayed。指标名称故意描述阶段，不把重复发布包装成“成功记录总数”。

WAL 还暴露条目数、记录数、载荷字节、目录物理字节、容量、利用率、最老条目年龄、文件系统剩余空间和最小保留空间。队列状态是 `healthy/warning/critical/blocked` 的低基数 one-hot Gauge。

Kafka 指标包含发布耗时、按固定类别划分的失败、ISR 不足次数和当前可写状态。标签不放 Gateway、Route、Request ID、Broker 地址、路径或错误文本，避免时序基数随业务数据增长。

## 探针语义

`/livez` 和 `/healthz` 只说明进程还活着，不访问 Kafka 或磁盘。

`/readyz` 检查当前实例能否保持可靠降级能力：

- Kafka 可写且 WAL 可写：`200`，目标为 Kafka；
- Kafka 暂时不可用但 WAL 可写：`200`，目标为 disk queue；
- Topic 已知不合规、回放暂停或 WAL 不可写：`503`，返回稳定原因码。

响应不会携带 Broker、WAL 路径或内部错误。详细原因留在受控日志和 Dashboard。

## 跨越 WAL 的 Trace

主要 Span 为：

- `als.receive_batch`
- `als.kafka.publish`
- `als.wal.append`
- `als.wal.replay`

ALS stream 可能持续很久，因此每个接收批次创建新的 root Span，避免整条连接共享一次采样决定或形成超大 Trace。WAL 条目保存原批次的 W3C Trace Context；回放可能在数小时后发生，它会创建新的 Trace，并通过 Span Link 指向原接收上下文，而不是伪造一个长期不结束的父子关系。

单次回放最多添加 128 个 Link。超过限制只影响诊断信息，不影响记录读取和投递。OTLP 导出使用有界队列；Collector 或 Tempo 不可用时允许丢 Trace。

## SLO 定义

可靠接收 SLI：

```text
1 - rate(records_rejected_total) / rate(records_valid_total)
```

30 天目标为 99.99%。Kafka 或 WAL 任一成功都算接收；协议边界丢弃不进入分母，因为它反映上游协议质量，另有独立告警。

Kafka 新鲜度 SLI 每分钟判断最老 WAL 记录是否不超过 5 分钟，30 天目标为 99%。空队列算满足，Prometheus 无法抓取 ALS 时算不满足。它直接描述用户多久能在分析页面看到请求，比用含重复项的 Kafka 写入计数推测延迟更可靠。

告警窗口、恢复步骤和查询入口见[ALS SLO 与告警处置](../../../../operations/als/monitoring/)。

## 端到端验证

`make als-e2e` 是本地故障验证入口，不放入标准 CI。它覆盖的行为包括：

- Kafka 正常时直接写入，WAL 保持为空；
- Kafka 故障后落 WAL，ALS 被强制终止并重启，积压能够恢复；
- 丢失 Kafka ACK 时允许重复，但两份消息的记录 ID 相同；
- WAL 满时 ALS 拒绝记录，而 Envoy 业务转发仍可完成；
- Prometheus、Loki 和 Tempo 能查询到同一故障链路；
- 中间条目损坏时启动失败，不静默跳过。

单元测试还应覆盖状态迁移、容量边界、格式校验、回放退避和探针响应。测试重点是故障语义，不是为了覆盖率给每个简单 getter 写用例。

## 排查顺序

先看 `records_rejected_total` 是否增长，再看 WAL 是否可写和最老积压年龄。Kafka 恢复后确认 `replayed` 与 `committed` 是否增长。若只看 `kafka_writable=0`，容易把正常的 WAL 降级误判为立即丢数据；若只看进程存活，又会漏掉 WAL 已阻塞的实例。
