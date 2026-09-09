---
title: 可观测性与验证
description: 将 ALS 的指标、就绪探针、Trace 和 SLO 对应到具体执行阶段
---

ALS 产生两类数据：

- `RequestRecord` 是产品数据，最终供 Ingate 用户在分析页面查询。
- 日志、Prometheus 指标和 Trace 是系统观测数据，用于判断 ALS 自身的健康状态。

两类数据不共用可用性边界。OTLP Collector、Tempo 或 Loki 故障可以导致诊断信息丢失，但不应阻塞 `RequestRecord` 进入 Kafka 或 WAL。

## 计数器对应的确切执行点

| 指标 | 增加位置 | 该次增加已经证明 |
| --- | --- | --- |
| `ingate_als_records_received_total` | 一批消息处理结束后，按原始条目数增加 | 已完成这次批处理；若进程在处理期间退出，该批不会增加此计数 |
| `ingate_als_records_valid_total` | `Recorder.Write` 入口 | 记录已通过协议转换和校验 |
| `ingate_als_records_kafka_accepted_total` | Kafka 逐条返回成功后 | Kafka 已确认，可能包含重投 |
| `ingate_als_records_spooled_total` | `Queue.Write` 成功后 | 记录已追加到 WAL |
| `ingate_als_records_replayed_total` | WAL 来源的记录被 Kafka 逐条确认后 | 对应记录已进入 Kafka，WAL 批次可能尚未 Commit |
| `ingate_als_records_committed_total` | `Queue.Commit` 成功后 | 已投递条目从 WAL 删除 |
| `ingate_als_records_rejected_total` | Kafka 未能确认整批，随后 WAL 追加也失败 | 这一批没有取得完整的同步持久化确认，不等于精确丢失条数 |
| `ingate_als_records_discarded_total` | 协议转换拒绝单条日志后 | 外部输入不完整或不支持 |

这些数字不能排成一个简单的守恒等式。例如，Kafka 部分成功后，Recorder 会把原批次完整写 WAL，因此同一批同时增加 `kafka_accepted` 和 `spooled`。若此时 WAL 也失败，`rejected` 会按整批增加，其中仍可能有一部分已经进入 Kafka；Kafka 结果为 `uncertain` 时，也无法证明 Broker 没有写入。WAL 已重放而 Commit 失败时，下次回放还会再次增加 `kafka_accepted` 和 `replayed`。

WAL Gauge 补充当前状态：待回放条目数、记录数、载荷字节、目录物理字节、容量、利用率、最旧条目年龄和文件系统剩余空间。`healthy/warning/critical/blocked` 使用低基数 one-hot Gauge。

Kafka 指标包含发布耗时、固定错误类别、ISR 不足次数和当前可写状态。标签中不放 Gateway、Route、Request ID、Broker 地址、URL path 或错误文本，避免时序数量随业务数据无界增长。

## `/readyz` 表示当前的可靠接收能力

Readiness 的判断顺序来自 `internal/als/server/http.go`：

```go
if status.Topic.Checked && !status.Topic.Compliant {
	return unavailable("topic_noncompliant")
}
if status.ReplayPaused {
	return unavailable("replay_paused")
}
if !status.Queue.Writable {
	return unavailable("wal_unavailable")
}
if !status.Topic.Compliant || status.Spooling {
	return ready("disk_queue")
}
return ready("kafka")
```

上面是对 handler 的等价压缩，用于显示判断顺序。实际 JSON 响应例如：

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

这表示 Kafka 当前不能直写，但 WAL 仍能可靠接收新记录，所以返回 HTTP 200。WAL 不可写时的响应为：

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

此时返回 HTTP 503。即使 Kafka 暂时正常，实例也已失去 Kafka 故障时的持久降级能力，不应继续接收新 stream。

`/livez` 和 `/healthz` 只返回进程存活，不访问 Kafka 或磁盘。

## 长流不共用一个 Trace

Envoy ALS stream 可能持续数小时。如果整条 stream 只创建一个 Span，所有批次会共用一次采样决定，Span 也会持续增长。Service 因此在每次 `Recv` 后创建独立 root Span：

```go
ctx, span := s.tracer.Start(
	stream.Context(),
	"als.receive_batch",
	oteltrace.WithNewRoot(),
	oteltrace.WithSpanKind(oteltrace.SpanKindConsumer),
)
```

同一接收批次中的 Kafka 发布和 WAL 追加继承这个上下文。WAL 还会把 W3C `traceparent` 与 `tracestate` 保存到条目中。

回放可能发生在原 Trace 结束数小时后，因此 Replayer 创建新 root Span。Queue 解码条目后，用 Span Link 关联原批次：

```go
if entry.spanContext.IsValid() && spanLinkCount < maxReplayLinks {
	oteltrace.SpanFromContext(ctx).AddLink(
		oteltrace.Link{SpanContext: entry.spanContext},
	)
	spanLinkCount++
}
```

这种关系表示“新任务由历史批次引起”，没有伪造一个长时间不结束的父子 Trace。单次回放最多保留 128 个 Link；超出上限只损失诊断关联，不会改变记录投递。

OTLP 导出使用有界队列。Collector 或 Tempo 不可用时允许丢 Span。`ingate_telemetry_spans_dropped_total{reason="queue_full"}` 记录本地队列已满时拒绝的 Span；远端持续失败会使队列逐渐占满，最终反映在这个计数中。

## 当前 SLO 是同步确认代理指标

同步确认率 SLI 的实际记录规则为：

```text
clamp(
  1 - sum by (instance) (rate(ingate_als_records_rejected_total[30d]))
    / clamp_min(
        sum by (instance) (rate(ingate_als_records_valid_total[30d])),
        1e-9
      ),
  0,
  1
)
```

30 天目标是 99.99%。Kafka 确认或 WAL 追加成功都算取得同步持久化确认。`discarded` 不进入分母，因为它表示协议输入质量，而非 Recorder 对有效记录的保存能力。

这个 SLI 是“没有出现未确认批次”的运行代理指标，不是精确的端到端完整率。它会把 Kafka 已接收一部分但 WAL 随后失败的整批计为拒绝，也无法观察进程在计数完成前退出、Envoy 发送前丢弃或 Kafka 不确定结果中的实际写入数。判断数据完整性仍需联合 Envoy 发送侧指标、Kafka 消费进度和 Analytics 入库进度。

Kafka 新鲜度每分钟记录“最旧 WAL 记录年龄小于 5 分钟”是否成立，30 天目标是 99%。空队列视为满足，Prometheus 抓取不到 ALS 时视为不满足。这个 SLI 只到 Kafka；Analytics 消费滞后和 ClickHouse 写入故障需要结合后续组件的指标。

告警窗口、PromQL 和处置顺序见 [ALS SLO 与告警处置](../../../../operations/als/monitoring/)。

## 本地故障验证

`make als-e2e` 是本地专项验证，不放入标准 CI。它覆盖：

- Kafka 正常时直写，WAL 保持为空；
- Kafka 故障后落 WAL，ALS 被强制终止并重启后恢复积压；
- Kafka 确认丢失后可能出现两份消息，两份保持同一记录 ID；
- WAL 满后 ALS 拒绝记录，Envoy 业务转发仍然成功；
- Prometheus、Loki 和 Tempo 可以查询同一故障链路；
- 中间 WAL 条目损坏时启动失败，不会静默跳过。

单元测试则覆盖状态迁移、容量边界、格式校验、回放退避和探针响应。测试用于验证故障语义，不为单纯提高覆盖率增加无意义用例。

## 源码与配置入口

- `internal/als/metrics`：ALS 计数器、Gauge 和 Histogram
- `internal/als/server/http.go`：探针决策和 JSON 响应
- `internal/als/service/service.go`：每批 root Span
- `internal/als/data/diskqueue/entry.go`：Trace Context 持久化
- `internal/pkg/telemetry`：有界 OTLP 导出与丢弃计数
- `deploy/docker/observability/rules/als.yaml`：SLO 记录规则和告警
- `deploy/docker/observability/grafana/dashboards/als.json`：ALS Dashboard
- `hack/als-e2e`：本地故障验证
