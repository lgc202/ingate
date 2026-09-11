---
title: ALS 可观测性
description: 从埋点、采集、传输、存储到告警解释 ALS 的日志、指标和 Trace 实现
---

ALS 的可观测性用于回答三个运维问题：

1. 组件现在还能否接收请求记录；
2. 记录停在 Envoy、Kafka 直写、WAL 回放还是下游消费；
3. 故障发生时，影响了多少记录、持续多久、还剩多少恢复时间。

这里的“可观测性”指 ALS 自身的日志、指标和 Trace。它与 Console 中的流量趋势、请求明细和 Token 用量属于两类数据：

| 数据 | 主要使用者 | 事实来源 | 存储 | 允许影响请求记录投递吗 |
| --- | --- | --- | --- | --- |
| 系统可观测数据 | 开发、值班和平台运维 | ALS 代码与运行环境 | Prometheus、Loki、Tempo | 不允许 |
| 产品分析数据 | 网关用户和管理员 | Envoy `RequestRecord` | Kafka、ClickHouse | 不参与业务转发，但自身需要可靠投递 |

如果 Tempo 不可用，ALS 可以丢失部分 Trace，但不能因此停止写 Kafka 或 WAL。如果 ClickHouse 不可用，产品分析会延迟，这属于 Analytics 数据链路故障，也不能用系统日志代替业务事实。

## 日志、指标和 Trace 的职责边界

初次接触可观测性时，容易把三者理解成同一份信息的不同界面。它们实际保留的信息结构不同。

| 信号 | 数据形态 | 擅长回答 | 不擅长回答 |
| --- | --- | --- | --- |
| 日志 | 带时间和字段的离散事件 | “发生了什么错误，错误上下文是什么” | 大规模聚合、准确计算比例 |
| 指标 | 按时间采样的数值序列 | “故障影响多大、趋势如何、是否越过阈值” | 还原某一次调用的完整步骤 |
| Trace | 一次工作中的 Span 关系 | “时间花在哪一步，父子操作和异步因果如何关联” | 统计所有请求；采样后本就不完整 |

一个典型排障过程为：告警从指标触发，在 Dashboard 判断范围，再用 Trace 找到一次代表性失败，最后用相同 `trace_id` 查询日志中的错误上下文。

```text
Prometheus alert
  -> Grafana metrics: 哪个实例、何时开始、积压多少
  -> Tempo trace: 哪个阶段耗时或失败
  -> Loki logs: 具体错误、恢复日志和进程身份
```

三类信号互相补充，但任何一个都不能单独证明 Envoy 到 ClickHouse 的记录完整性。

## 当前观测拓扑

ALS 没有把三类信号强行走同一套 SDK。指标使用 Prometheus Pull，Trace 使用 OTLP gRPC，日志由容器标准流交给 Docker logging driver。

```text
日志
ALS slog -> stderr -> Docker fluentd driver -> OTel Collector -> Loki -> Grafana

指标
ALS collectors -> /metrics <- Prometheus scrape -> rules -> Grafana / Alertmanager

Trace
ALS OTel SDK -> OTLP gRPC -> OTel Collector -> Tempo -> Grafana
```

这种拆分对应各信号的常用传输方式：Prometheus 主动抓取指标，日志跟随容器运行时采集，Trace 由进程主动导出。Collector 当前只承接日志与 Trace，不转发 ALS 的 Prometheus 指标。

## 统一进程身份

ALS 启动时调用 `telemetry.NewIdentity` 生成：

| 字段 | 示例 | 生命周期 |
| --- | --- | --- |
| `service.namespace` | `ingate` | 代码固定 |
| `service.name` | `ingate-als` | 组件固定 |
| `service.instance.id` | UUID v4 | 每次进程启动重新生成 |
| `service.version` | 构建版本 | 随发布变化 |
| `deployment.environment.name` | `development` | 部署配置 |
| `host.name` | 容器或主机名 | 运行环境 |

日志把这些字段作为结构化属性写入；Trace 把它们作为 OpenTelemetry Resource。`service.instance.id` 表示一次进程生命周期，不是 Envoy Node ID，也不是 ALS WAL 所有者的永久 ID。重启后变化有助于区分“同一部署位置上的新进程”。

当前 Prometheus 的 `instance` 标签来自抓取地址，例如 `controller:18092`，不等于日志和 Trace 中的启动 UUID。这是现有实现的一个关联缺口：同一抓取地址重启后，指标时间序列仍沿用原 `instance`，日志和 Trace 则能区分两次进程生命周期。排障时需要结合重启时间和 `service.instance.id`，不能假设两个字段可以直接相等匹配。

## 日志链路如何工作

### 1. ALS 进程写结构化日志

`internal/pkg/telemetry/logging.go` 使用 Go `log/slog`，外层接 Kratos 的 slog Handler：

```go
logger := slog.New(kratoslog.NewHandler(
	kratoslog.WithWriter(os.Stderr),
	kratoslog.WithFormat(kratoslog.FormatJSON),
	kratoslog.WithLevel(...),
	kratoslog.WithExtractor(kratosotel.TraceAttrs),
))
```

Compose 配置使用 JSON。每条日志包含时间、级别、消息、进程身份；当前 Context 中存在有效 Span 时，Kratos OpenTelemetry extractor 还会加入 `trace_id` 和 `span_id`。

ALS 只在责任边界记录一次错误。例如 Kafka Publisher 返回分类后的错误，由 Recorder 记录“已切换到 disk queue”；底层不会再重复打一条同内容 ERROR。正常健康检查和轮询不写 INFO，避免日志量被探针淹没。

日志中不记录完整请求、URL query、凭据和 Kafka 消息 Value。结构化日志的价值在于字段可查询，不代表可以把业务数据全部写进去。

### 2. Docker 把标准流交给 Fluent Forward

Observability Overlay 为 ALS 配置 `fluentd` logging driver：

```yaml
logging:
  driver: fluentd
  options:
    fluentd-address: "127.0.0.1:24224"
    fluentd-async: "true"
    fluentd-buffer-limit: "1024"
    mode: non-blocking
    max-buffer-size: 4m
    tag: ingate.als
```

这里有两个目标：Collector 短暂停止时允许缓冲；持续故障时限制 Docker 占用，避免日志反压阻塞 ALS 主流程。`mode: non-blocking` 意味着缓冲耗尽后允许丢日志。

因此，`logger.InfoContext` 返回不等于日志已经进入 Loki。日志可靠性低于 RequestRecord：

```text
slog 调用成功
  != Docker 已接收
  != Collector 已接收
  != Loki 已持久化
```

当前 ALS 没有拿到 Docker driver 的丢弃回调，也没有“容器日志丢弃总数”这一业务指标。Collector 完全不可达且缓冲耗尽时，日志可能丢失而 ALS 自身无法精确计数。这是为了隔离观测故障而接受的边界。

### 3. Collector 处理并发送 Loki

Collector 的日志 Pipeline 为：

```text
fluent_forward receiver
  -> memory_limiter
  -> resource/als
  -> batch
  -> otlp_http/loki exporter
```

- `memory_limiter` 在 Collector 内存压力过高时拒绝新数据，防止 Collector 拖垮主机。
- `resource/als` 补齐稳定的 `service.namespace` 和 `service.name`。
- `batch` 按 512 条触发或最多等待 2 秒，减少出口请求数并提高压缩效率。`send_batch_size` 是触发阈值，不是硬性的最大批次上限。
- Loki Exporter 使用 2 个消费者、容量 2048 的有界发送队列，对可重试错误最多重试 5 分钟。

发送队列的单位由 Collector Exporter 实现决定，不能直接把 `queue_size: 2048` 解释成“恰好 2048 条日志”。运维应以 `otelcol_exporter_queue_size`、`queue_capacity`、enqueue failed 和 send failed 指标判断出口状态。

### 4. Loki 如何存储和查询

当前 Compose 中 Loki 使用单实例、本地文件系统、7 天保留。低基数 Resource 字段用于索引，请求 ID 和错误详情留在日志正文中查询。

低基数表示可选值数量有限，例如服务名通常只有几个。把 `request_id`、URL path 或错误全文做成标签，会为每个不同值创建新 Stream，增加索引、内存和查询成本。因此这些字段可以存在日志正文中，但不应成为 Loki 或 Prometheus 的无界标签。

当前 Loki 配置适合本地开发和专项验证，不具备生产级多副本、对象存储和跨节点容灾。

## 指标链路如何工作

### 1. 埋点分为事件和状态

ALS 使用两个 Collector：

- `EventCollector` 在操作发生时更新 Counter、Gauge 或 Histogram；
- `StatusCollector` 在 Prometheus 抓取时读取 Recorder 与 Queue 的当前快照。

这个区分来自数据性质。Kafka 失败发生过多少次是历史事件，必须当场累计；WAL 现在有多少条是当前状态，可以从 Queue 快照读取。

```text
业务动作发生
  -> EventCollector.Inc / Observe

Prometheus 发起 GET /metrics
  -> StatusCollector.Collect
  -> Recorder.Status + Recorder.Counters
  -> 输出当前样本
```

### 2. Counter、Gauge、Histogram 怎么选

| 类型 | 数值行为 | ALS 示例 | 常用 PromQL |
| --- | --- | --- | --- |
| Counter | 只增加，进程重启归零 | `records_rejected_total` | `rate`、`increase` |
| Gauge | 可以增加、减少或直接设置 | `disk_queue_records`、`streams_active` | 原值、`max_over_time` |
| Histogram | 将每次观测累计到一组桶 | `kafka_publish_seconds` | `histogram_quantile` |

Counter 原始值通常没有直接业务意义。实例重启会归零，Prometheus 的 `rate()` 会识别正常 Counter reset，并估算窗口内每秒增长速度。

Histogram 会导出 `_bucket`、`_sum`、`_count` 多组时间序列。经典 Histogram 的 bucket 是累计的，例如耗时 0.2 秒的观测会同时进入 `le="0.25"`、`le="0.5"` 和更大的桶。p95 查询需要先对 bucket 求 rate，再按 `le` 聚合：

```text
histogram_quantile(
  0.95,
  sum by (le) (
    rate(ingate_als_kafka_publish_seconds_bucket[5m])
  )
)
```

Histogram 可以跨实例聚合，但精度取决于 bucket 边界。当前使用 client_golang 默认耗时桶；若将来有明确的延迟 SLO，应让桶边界覆盖 SLO 阈值，而不是只追求曲线看起来平滑。

### 3. 独立 Registry 的作用

`internal/pkg/prometheus.NewHandler` 不使用默认全局 Registry：

```go
registry := prometheus.NewRegistry()
registry.MustRegister(
	collectors.NewGoCollector(),
	collectors.NewProcessCollector(...),
)
registry.MustRegister(componentCollectors...)
```

独立 Registry 只暴露显式传入的 ALS Collector，并附带 Go runtime 和进程指标。这样可以避免依赖库在全局 Registry 中注册未知指标，也让每个组件选择自己的观测面。

`/metrics` 使用 OpenMetrics 格式。它是只读快照端点，不访问 Kafka；但 `StatusCollector` 会调用 `Queue.Status()` 探测 WAL 目录物理占用和文件系统剩余空间，所以抓取延迟仍可能受本地文件系统影响。

### 4. Prometheus 怎样抓取和保存

Prometheus 每 15 秒请求：

```text
http://controller:18092/metrics
```

ALS 与 Controller 共享网络命名空间，所以 Compose 用 `controller` 服务名访问 ALS 的 HTTP 端口。Prometheus 为每个样本自动加上：

```text
job="ingate-als"
instance="controller:18092"
```

抓取成功由 `up{job="ingate-als"}=1` 表示。若 `/metrics` 超时、连接失败或格式非法，`up=0`。业务指标暂时缺失不应简单解释为零；它也可能意味着采集失败。

Prometheus 本地 TSDB 保留 35 天，用于计算 30 天 SLO 并留出采集间隙。当前同样是单实例本地存储，没有远端写、多副本查询或长期归档。

### 5. 指标对应哪个代码完成点

| 指标 | 增加或采集位置 | 它证明了什么 | 它没有证明什么 |
| --- | --- | --- | --- |
| `records_received_total` | 一批处理尝试结束后 | Envoy message 已被 ALS 处理到返回点 | 每条都有效或已持久化 |
| `records_valid_total` | `Recorder.Write` 入口 | 记录通过转换和校验 | 后续 Kafka/WAL 成功 |
| `records_kafka_accepted_total` | Kafka 逐条成功结果 | Kafka 返回确认，可能是重复 | Analytics 已消费或入库 |
| `records_spooled_total` | Queue Write 成功 | 原批次已进入 WAL 同步边界 | 已进入 Kafka |
| `records_replayed_total` | WAL 来源记录被 Kafka 确认 | Kafka 接收了回放记录 | WAL Commit 已成功 |
| `records_committed_total` | Queue Commit 成功 | 已确认前缀从 WAL 删除 | Kafka 中没有重复 |
| `records_rejected_total` | Kafka 未确认整批且 WAL 也失败 | 整批没有取得完整同步确认 | 这一批全部实际丢失 |
| `records_discarded_total` | 协议转换拒绝单条日志 | 输入不完整或类型不支持 | Recorder 可靠性退化 |

这些 Counter 不满足简单守恒式。Kafka 部分成功后，原批次仍会整体入 WAL，同一记录可能同时计入 `kafka_accepted` 和 `spooled`；Commit 失败后再次回放，又会再次增加 `replayed`。`rejected` 按整批增加，但 Kafka 可能已确认其中一部分或返回不确定结果。

WAL Gauge 补充当前状态：entries、records、payload bytes、目录物理字节、容量、利用率、最旧条目年龄、文件系统剩余空间和是否可写。`healthy/warning/critical/blocked` 使用低基数 one-hot Gauge，任一时刻对应状态值为 1，其余为 0。

## Trace 链路如何工作

### 1. Trace、Span 和 Context

Trace 表示一次工作及其因果关系；Span 表示其中一个有起止时间的操作。Trace ID 标识整条 Trace，Span ID 标识一个步骤。父子关系表示同步调用继承，Link 表示异步工作由历史事件触发，但不要求父 Span 仍存活。

ALS 当前主要 Span 为：

```text
als.receive_batch                  root, Consumer
  ├─ als.kafka.publish            child, Producer
  └─ als.wal.append               child

als.wal.replay                    新 root
  └─ als.kafka.publish            child, Producer
      links -> 历史 als.receive_batch
```

Span 中只保存稳定的操作属性，如 Kafka messaging system、Topic、操作类型和批次记录数。Request ID、URL 和完整记录不进入 Span，避免高基数、敏感信息和 Trace 体积膨胀。

### 2. 每个批次使用独立 root Trace

Envoy ALS 是长连接 stream，可能持续数小时。如果整个 stream 只有一个 Span：

- 一次采样决定会控制数小时的全部批次；
- Span 生命周期过长；
- 事件和属性容易无限增长；
- 某个批次的延迟难以独立观察。

Service 因此每次 `Recv` 后创建新 root：

```go
ctx, span := s.tracer.Start(
	stream.Context(),
	"als.receive_batch",
	oteltrace.WithNewRoot(),
	oteltrace.WithSpanKind(oteltrace.SpanKindConsumer),
)
```

Kafka 发布和 WAL 追加继承这个 Context，形成普通父子 Span。它故意不继承 gRPC stream 的远端 Trace，因为 Envoy ALS 消息没有为每个批次提供一个可依赖的上游业务 Trace Context。

### 3. 采样发生在哪里

TracerProvider 使用：

```go
sdktrace.ParentBased(
	sdktrace.TraceIDRatioBased(sampleRatio),
)
```

无父 root Span 按 `sample_ratio` 做概率采样；有父 Span 时继承父采样决定。Compose 当前设置 `1.0`，便于本地故障验证；普通本地配置是 `0.1`。

采样为 10% 不表示每连续十批恰好留一批，而是每个 Trace ID 独立做近似概率选择。短时间、低流量样本可能明显偏离 10%。错误 Trace 当前没有 Tail Sampling 特权，未采样的失败不会因为后来出错自动补回。

### 4. WAL 保存 Trace Context

WAL 回放可能比原接收晚几分钟甚至几小时。若让回放 Span 继续做原 Span 的 child，Trace 会横跨整个积压时间，父子时序也变得难以解释。

QueueEntry 保存 W3C `traceparent` 和 `tracestate`。读取时恢复为远端 SpanContext，并给新的 `als.wal.replay` root 添加 Link：

```go
if entry.spanContext.IsValid() && spanLinkCount < maxReplayLinks {
	oteltrace.SpanFromContext(ctx).AddLink(
		oteltrace.Link{SpanContext: entry.spanContext},
	)
}
```

Link 表达“这次回放由那次历史接收产生”。单次回放最多 128 个 Link，防止一个回放 Span 的诊断数据随批次数量无限增长。超过上限只损失关联，不影响 WAL 记录读取和提交。

Trace Context 不是业务 ID，也不参与去重。未采样时 Context 可能仍传播，但本地不会记录对应 Span；RequestRecord 的完整性仍由稳定记录 ID 和投递状态判断。

### 5. 有界的进程内 Trace 导出

Span 在 `End()` 后才进入导出队列。如果 Collector 或 Tempo 变慢，而应用继续产生 Span，无界队列会持续占用内存，最终影响 ALS 主流程。

当前 `spanBuffer` 在官方 `BatchSpanProcessor` 外增加容量令牌：

```text
Span.End
  -> 尝试获取 pending token
  -> 有容量：交给 BatchSpanProcessor
  -> 无容量：丢弃并增加 spans_dropped_total{reason="queue_full"}
```

容量覆盖“等待导出”和“正在导出”的已采样 Span。默认最大 2048，单次最多导出 256，未满时最多等待 2 秒，单次出口预算 5 秒。队列满时 `OnEnd` 不阻塞请求记录处理。

`ingate_telemetry_spans_dropped_total{reason="queue_full"}` 只统计进程内容量不足时的主动丢弃。Exporter 最终返回错误导致的一批失败，目前不会增加这个自定义 Counter；需要结合进程错误输出和 Collector 接收指标判断。这一点不能从指标名字推导成“所有未到 Tempo 的 Span 总数”。

### 6. Collector 与 Tempo 的第二层缓冲

Trace Pipeline 为：

```text
OTLP gRPC receiver
  -> memory_limiter
  -> batch
  -> otlp_grpc/tempo exporter
       -> bounded sending_queue
       -> retry_on_failure(max 5m)
  -> Tempo local storage
```

这里存在两层隔离：ALS 进程内队列保护 ALS 内存；Collector 出口队列吸收 Tempo 短暂故障。两者容量单位和丢弃指标不同，不能只监控 ALS 自定义 Counter。

Collector 暴露自己的 Prometheus 指标。关键组包括：

- `otelcol_receiver_accepted_spans` / `accepted_log_records`：进入 Collector 的量；
- `otelcol_receiver_refused_spans` / `refused_log_records`：接收侧拒绝量；
- `otelcol_exporter_queue_size` / `queue_capacity`：出口积压和容量；
- `otelcol_exporter_enqueue_failed_spans` / `enqueue_failed_log_records`：无法进入出口队列；
- `otelcol_exporter_send_failed_spans` / `send_failed_log_records`：发送失败，可能仍会重试；
- `otelcol_exporter_sent_spans` / `sent_log_records`：成功交给后端。

当前 Prometheus 已抓取 Collector 的 `:8888`，但项目还没有为这些指标配置完整告警。这属于现有可观测栈的待补边界。

Tempo 在 Compose 中是单实例本地存储，保留 7 天。它用于本地开发和故障演示，不代表生产 HA Trace 后端。

## 日志与 Trace 怎样互相跳转

当代码使用 `InfoContext`、`WarnContext` 或 `ErrorContext` 且 Context 中有有效 Span 时，日志 JSON 包含 `trace_id`。Grafana Loki Data Source 配置 Derived Field：

```yaml
derivedFields:
  - name: TraceID
    matcherRegex: '"trace_id":"([0-9a-f]{32})"'
    datasourceUid: tempo
```

在 Loki 中查看日志时，Grafana 从正文提取 Trace ID，并生成跳到 Tempo 的链接。Tempo 反向查日志时按 `service.name` 和 Trace ID 查询 Loki，时间范围向前后各扩两分钟。

这一关联要求：

- 日志在带 Span 的 Context 中写出；
- 该 Trace 被采样并成功进入 Tempo；
- 相应日志没有在 Docker、Collector 或 Loki 入口丢失；
- `trace_id` 保留为 32 位小写十六进制 JSON 字段。

因此“按钮没有跳转结果”可能是采样、任一出口丢弃、时间范围或字段格式问题，不一定表示业务操作没有发生。

当前指标没有 Prometheus Exemplar，无法从某个 Histogram bucket 直接跳到代表性 Trace。排障仍需先按时间和实例缩小范围，再查询 Trace 或日志。

## 健康检查与可观测指标的区别

ALS 暴露三个探针：

| 端点 | 回答的问题 | 是否检查 Kafka/WAL |
| --- | --- | --- |
| `/livez` | 进程是否活着并能响应 HTTP | 否 |
| `/healthz` | 通用存活兼容端点 | 否 |
| `/readyz` | 当前是否具备可靠接收新记录的路径 | 读取缓存状态和 WAL 状态 |

Readiness 判断顺序为：

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

Kafka 不可写但 WAL 可写时，`/readyz` 返回 HTTP 200，并标记 `write_target=disk_queue`。这是因为实例仍能取得本地持久确认。WAL 不可写时返回 503，即使 Kafka 暂时正常：该实例已经失去 Kafka 下一次故障时的保护路径。

Readiness 是当前布尔决策，指标保留时间趋势。`ready=200` 不能说明过去一小时没有拒绝记录；`ready=503` 也不能说明业务请求转发已失败，因为 ALS 不在 Envoy 同步转发路径中。

Compose healthcheck 会把连续失败的 ALS 容器标记为 unhealthy，但 `restart: unless-stopped` 不会仅因 healthcheck 失败自动重启。探针、容器状态和重启策略是三个不同机制。

## SLI、SLO 和 Error Budget

- **SLI** 是实际测量方式，例如同步确认率。
- **SLO** 是目标，例如 30 天同步确认率达到 99.99%。
- **Error Budget** 是目标允许的失败比例。99.99% SLO 对应 0.01%，即 `0.0001`。

### 同步确认率

当前 SLI 为：

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

有效记录写入 Kafka 或 WAL 任一持久边界，都不会增加 `rejected`。Kafka 未确认整批且 WAL 也失败时增加。`clamp_min` 防止零流量除零，外层 `clamp` 把异常采样产生的结果限制在 `[0,1]`。

这是一项**可靠接收代理指标**，不是精确丢失率：

- Kafka 部分成功、随后 WAL 失败时，整批计入 rejected，但其中可能已有消息；
- Kafka 不确定结果可能实际已经追加；
- ALS 在计数前崩溃无法体现在 Counter 中；
- Envoy 发送前丢弃不进入 ALS 指标；
- Analytics 和 ClickHouse 故障不在该 SLI 范围内。

### Kafka 新鲜度

每分钟观察最旧 WAL 条目年龄是否小于 300 秒，并要求 `up=1`。空队列时年龄为 0，视为满足：

```text
(
  ingate_als_disk_queue_oldest_entry_age_seconds < bool 300
    and on (instance)
  up{job="ingate-als"} == 1
)
or on (instance)
up{job="ingate-als"} * 0
```

后半段保证抓取失败产生值 0，而不是整条序列消失。30 天对这些一分钟好坏样本取平均，目标为 99%。

该 SLI 只衡量记录从 ALS 进入 Kafka 前的延迟。WAL 为空不代表 Analytics 没有消费滞后，也不代表 ClickHouse 查询已经可见。

### 多窗口 Burn Rate 告警

Burn Rate 表示消耗 Error Budget 的速度：

```text
burn_rate = observed_error_ratio / allowed_error_ratio
```

99.99% SLO 的预算为 `0.0001`：

```text
14.4 倍阈值 = 0.0001 * 14.4 = 0.00144
 6.0 倍阈值 = 0.0001 *  6.0 = 0.00060
```

Critical 要求 5 分钟和 1 小时窗口同时超过 14.4 倍；Warning 要求 30 分钟和 6 小时同时超过 6 倍。短窗口快速发现突发，长窗口过滤一次性尖峰。两个窗口同时成立，减少单窗口告警在速度和稳定性之间的冲突。

## 从告警表达式到通知

```text
Prometheus 每 15s 抓取
  -> 每 15s/30s/1m 评估 rule group
  -> alert expression 持续满足 for 条件
  -> Prometheus 发送到 Alertmanager
  -> Alertmanager 分组、抑制、路由到 receiver
```

`for: 2m` 表示表达式连续满足两分钟后才进入 firing，不表示 Prometheus 每两分钟检查一次。规则组自己的 `interval` 决定评估频率。

当前 Alertmanager 只有名为 `default` 的空 receiver，没有邮件、Webhook 或即时通信渠道。这意味着告警状态可以在 Prometheus/Alertmanager 中查看，但不会自动通知外部人员。部署到需要值班响应的环境时，必须配置真实 receiver、路由、分组和静默策略。

告警含义和逐项处置见 [ALS SLO 与告警处置](../../../../operations/als/monitoring/)。架构文档解释信号如何产生，运维文档负责告诉值班人员看到告警后怎么做。

## 如何判断数据停在哪一段

| 观察 | 更可能的状态 | 下一步证据 |
| --- | --- | --- |
| Envoy `logs_dropped` 增长，ALS `received` 不增长 | Envoy 发送缓冲或 ALS 连接前丢弃 | Envoy ALS stats、连接日志 |
| `valid` 增长，`kafka_accepted` 停止，`spooled` 增长 | Kafka 直写失败，WAL 正常接管 | Kafka failure class、ISR、WAL 年龄 |
| `rejected` 增长 | Kafka 和 WAL 均未完整确认 | Queue writable、磁盘、首次 Kafka 错误 |
| WAL records 增长，Kafka writable=0 | Kafka 仍不可用 | Broker、网络、认证、Topic 契约 |
| Kafka writable=1，WAL records 不下降 | 回放净速率不足或 Commit 失败 | committed/spooled rate、replay backoff、磁盘日志 |
| WAL 已空，产品数据仍延迟 | 故障在 Analytics 或 ClickHouse | Consumer lag、Analytics 写入指标 |
| Trace drop 增长，业务计数正常 | 观测出口过载 | Collector queue/refused/send failed |

第一行需要特别注意：当前 Prometheus 没有抓取 Envoy 的 `logs_dropped`、`grpc_entries_flush_failed` 等 ALS 发送侧指标。因此现有 Dashboard 无法独立证明 Envoy 到 ALS 的完整性。这是当前端到端观测的主要盲区。

## 故障隔离与允许丢弃的位置

| 故障 | 是否阻塞 RequestRecord | 允许丢什么 | 有哪些信号 |
| --- | --- | --- | --- |
| Loki 停止 | 否 | 缓冲耗尽后的系统日志 | Collector 队列与发送失败；ALS 无精确 driver drop 计数 |
| Tempo 停止 | 否 | 缓冲耗尽或导出失败的 Span | ALS queue-full Counter、Collector 指标 |
| Prometheus 停止 | 否 | 停止期间未抓取的指标样本 | 外部探测；本地 Counter 仍在内存累计 |
| OTel Collector 停止 | 否 | 超过有界缓冲的日志与 Trace | ALS Trace drop、Docker/Collector 状态 |
| Kafka 停止 | 否，WAL 可写时 | 暂不丢，记录延迟 | spooled、WAL backlog、Kafka failure |
| WAL 也不可写 | ALS 终止 stream | 有效记录存在缺口风险 | rejected、readyz=503、critical alert |

“观测故障不能阻塞业务”不是免费保证。它通过有界队列和非阻塞写换来，代价是持续故障时允许丢系统日志或 Span。RequestRecord 使用另一套 Kafka/WAL 可靠性边界，因为它是产品分析数据。

## 当前实现的边界

现有能力已经能诊断 ALS 自身的直写、降级和回放，但还不能称为完整的生产可观测系统。明确缺口包括：

1. **Envoy 发送侧未纳入 Prometheus。** 无法在同一面板观察 `logs_dropped`、flush failure 和 ALS gRPC 连接状态。
2. **下游端到端新鲜度未闭合。** ALS SLO 到 Kafka 为止，缺少 Kafka Consumer Lag、Analytics 批次写入和 ClickHouse 可见时间的统一视图。
3. **Collector 与后端告警不完整。** 已抓取 Collector 指标，但没有覆盖接收拒绝、出口队列接近满、最终发送失败；Loki、Tempo、Prometheus 自身也缺少可用性告警。
4. **日志丢弃无法精确计数。** Docker non-blocking buffer 丢弃没有反馈到 ALS 指标。
5. **实例身份未统一。** Prometheus 抓取地址与日志/Trace 启动 UUID 不能直接关联。
6. **指标没有 Exemplar。** 无法从发布耗时 Histogram 直接进入代表性 Trace。
7. **Alertmanager 没有实际通知渠道。** 空 receiver 只适合本地验证。
8. **观测后端均为本地单实例。** Loki、Tempo、Prometheus 没有高可用和远端持久存储。

这些缺口不改变 ALS 当前 Kafka/WAL 的数据语义，但会限制故障发现速度、端到端定位和生产容灾。后续设计系统级可观测性时，应优先补发送侧、消费侧和 Collector 自监控，而不是继续增加更多 ALS 内部指标。

## 本地验证范围

`make als-e2e` 是本地专项验证，不加入标准 CI。一次完整故障演练应观察三类证据：

| 场景 | 数据行为 | 应出现的指标 | 应能找到的诊断信息 |
| --- | --- | --- | --- |
| Kafka 正常 | 直写，WAL 为空 | accepted 增长 | receive -> kafka Trace |
| Kafka 停止 | 新批进入 WAL | spooled、backlog、oldest age 增长 | 切换日志，wal.append Span |
| Kafka 恢复 | Read-Publish-Commit | replayed、committed 增长 | replay root 与历史 Link |
| ACK 丢失 | 允许同 ID 重复 | uncertain failure | Kafka/WAL 两条相关路径 |
| WAL 满 | stream 终止，业务转发仍成功 | rejected 增长，ready=503 | disk queue blocked 错误 |
| Collector 停止 | 数据投递不受影响 | Trace drop 或 Collector down | 恢复后新 Trace 可查询 |
| WAL 中段损坏 | ALS 启动失败 | 无新业务样本 | 启动错误明确指出序号 |

测试不只检查“页面上有一条线”。需要把指标增量、日志字段、Span 关系和 Kafka/WAL 最终状态放在同一个时间窗口内核对。

## 常见深挖点

### 指标继续使用 Prometheus Client

当前部署已经采用 Prometheus Pull，Go client 提供成熟的 Counter、Gauge、Histogram 和 Registry。再经 OTLP 转一遍会增加 Collector 依赖和语义转换，却没有当前需求证明收益。Trace 使用 OTLP，不要求三类信号必须使用同一 SDK。

### Trace 与 RequestRecord 使用不同可靠性等级

Trace 是采样后的诊断数据，持续后端故障时保住全部 Trace 会让观测系统反过来耗尽 ALS 内存。RequestRecord 是产品分析输入，使用 Kafka/WAL 取得更强的持久化边界。

### Request ID 不适合作为 Prometheus 标签

每个请求几乎都是新值，会为每个 ID 创建独立时间序列，导致高基数。单请求查询应使用日志、Trace 或 ClickHouse，而不是指标标签。

### 日志和 Trace 保存的信息不同

日志记录离散事件；Trace 保存一次批次在 Kafka、WAL 等步骤的时长和关系。只靠日志需要人工按时间拼接，且很难准确得到每一步耗时。

### `/readyz` 与告警的时间尺度不同

Readiness 只描述当前是否可接收，不能表达过去发生的拒绝、积压增长速度和 SLO 预算。告警建立在时间序列上，可以要求条件持续一段时间并区分影响等级。

## 源码与配置入口

- `internal/pkg/telemetry/identity.go`：统一服务身份与 OpenTelemetry Resource
- `internal/pkg/telemetry/logging.go`：Kratos slog、结构化字段和 Trace 属性提取
- `internal/pkg/telemetry/tracing.go`：采样、TracerProvider 和 OTLP Exporter
- `internal/pkg/telemetry/buffer.go`：非阻塞 Span 容量边界
- `internal/als/metrics/events.go`：事件型 Counter、Gauge 和 Histogram
- `internal/als/metrics/prometheus.go`：Recorder 与 WAL 状态采集
- `internal/als/server/http.go`：探针和 `/metrics`
- `internal/als/service/service.go`：每批 root Span
- `internal/als/data/kafka/publisher.go`：Kafka Producer Span 与 Trace header
- `internal/als/data/diskqueue/entry.go`：WAL Trace Context 持久化
- `deploy/docker/observability/otel-collector.yaml`：日志与 Trace Pipeline
- `deploy/docker/observability/prometheus.yaml`：抓取配置
- `deploy/docker/observability/rules/als.yaml`：SLI、SLO 与告警规则
- `deploy/docker/observability/grafana/dashboards/als.json`：ALS Dashboard
- `deploy/docker/observability/grafana/provisioning/datasources/datasources.yaml`：日志与 Trace 跳转

## 上游资料

- [Prometheus 指标类型](https://prometheus.io/docs/concepts/metric_types/)
- [Prometheus 埋点实践](https://prometheus.io/docs/practices/instrumentation/)
- [Prometheus Histogram 与 Summary](https://prometheus.io/docs/practices/histograms/)
- [OpenTelemetry Trace 概念](https://opentelemetry.io/docs/concepts/signals/traces/)
- [OpenTelemetry Collector 内部指标](https://opentelemetry.io/docs/collector/internal-telemetry/)
- [OpenTelemetry Collector Batch Processor](https://github.com/open-telemetry/opentelemetry-collector/tree/main/processor/batchprocessor)
