---
title: 记录 ID 与幂等
description: 通过 ID 的生成和消费端代码说明 ALS 能消除哪些重复
---

ALS 链路中同时存在三种 ID。它们各自处理不同边界，不能互相替代。

| ID | 生成方 | 有效范围 | 用途 |
| --- | --- | --- | --- |
| `RequestRecord.id` | Ingate ALS | 记录对象生成后的整条异步链路 | Kafka key、WAL 回放和 ClickHouse 幂等 token |
| `request_id` | Envoy 或上游 | 一次请求及其调用链 | 关联代理、应用和用户日志 |
| Kafka Producer ID 与序列号 | Kafka 协议 | 一个 Producer 会话 | 消除同一会话内的重试追加 |

![RequestRecord ID 在 ALS、Kafka、Analytics 和 ClickHouse 之间的传递](/ingate/images/als/idempotency.svg)

## `RequestRecord.id` 在解析成功时生成

`internal/als/service/request_record.go` 为每条通过校验的 Envoy 日志创建 UUID v4：

```go
record := &alsv1.RequestRecord{
	Id:        uuid.NewString(),
	RequestId: request.GetRequestId(),
	// 其他字段来自 Envoy 访问日志。
}
```

这个位置将“记录的生命周期”定义为：ALS 已经把一条外部日志转成 Ingate 记录。后续的 Kafka 重试、WAL 回放和 Analytics 重消费传递同一个对象，因此 ID 保持不变。

UUID v4 不需要共享时钟、节点编号、数据库序列或持久化计数器。ALS 重启只会创建新的随机 ID，不会与已在 WAL 中的记录复用一个本地序列。对请求分析链路来说，这比引入 Snowflake 节点租约或集中序列更简单，也避免新的可用性依赖。

Kafka 消息使用记录 ID 作为 key：

```go
&kgo.Record{
	Key:   []byte(record.GetId()),
	Value: value,
}
```

因此重放不会只在 value 中保留 ID，Kafka 外层协议也使用同一个值。Analytics 会校验 `message.Key == record.id`，防止封装层和载荷指向不同记录。

## `request_id` 只用于关联

Envoy Request ID 会原样保存到 `request_id`，但不用作主键。原因来自其边界：

- 客户端可以不发、重复使用或伪造该值；
- 一次请求的重试或内部调用可以产生多条合法观测记录；
- 多个代理节点可能同时观测同一条调用链。

`id` 标识一条 Ingate 分析记录，`request_id` 则用于将它与其他系统的请求日志关联。

## 备选 ID 方案的取舍

| 方案 | 优点 | 在当前链路中的问题 |
| --- | --- | --- |
| Envoy `request_id` | 现成，便于跨系统查询 | 可缺失、伪造和复用，不能稳定表示一条分析事实 |
| 内容哈希 | 同一输入可生成同一 ID | 包含时间等动态字段时难以复现；排除这些字段又可能合并两次合法请求 |
| Snowflake 或集中序列 | 有序，可从 ID 读出部分信息 | 需要稳定节点号、租约或集中存储，新增一个生成 ID 的故障点 |
| UUID v7 | 时间有序，对 B-tree 写入更友好 | 当前 ClickHouse 查询按 `started_at` 和 ID 分页，没有依赖 ID 时序的证据 |
| UUID v4 | 无状态、无共享依赖 | 本身不带时间顺序，查询必须单独使用 `started_at` |

当前选择 UUID v4，因为 ALS 只需要在记录对象创建后稳定识别它，没有用 ID 表达业务顺序的需求。若后续的真实压测证明 UUID v4 导致存储局部性问题，UUID v7 是不引入协调依赖的可选替换；在此之前不为理论排序收益增加协议含义。

## 重复窗口和对应的去重层

| 重复窗口 | 两次投递的 `RequestRecord.id` | 处理者 |
| --- | --- | --- |
| franz-go 在同一 Producer 会话重试 | 不需要业务 ID 参与 | Kafka 的 Producer ID 和序列号 |
| Kafka 已写入，ALS 收到不确定错误后把整批写 WAL | 相同 | Analytics 和 ClickHouse |
| Kafka 已确认，WAL Commit 失败后重放 | 相同 | Analytics 和 ClickHouse |
| Analytics 入库后、提交 offset 前退出 | 相同 | Analytics 和 ClickHouse |
| Envoy 重发日志，ALS 重新解析 | 不同 | 当前无法可靠识别 |

Kafka 幂等 Producer 只能消除同一 Producer 会话内的正常重试追加。ALS 允许结果不明的在途写超时，以便及时切换到 WAL；此后的回放属于应用层重新发布，已经超出原序列窗口。ALS 重启、Kafka 确认丢失、WAL Commit 失败和消费端重投同样需要稳定的 `RequestRecord.id`。

## Analytics 的两层去重

消费批次先在内存中检查同 ID 内容是否一致：

```go
if previous, exists := seen[record.GetId()]; exists {
	if proto.Equal(previous, record) {
		decoded.duplicateCount++
	} else {
		decoded.invalidCount++
	}
	continue
}
seen[record.GetId()] = record
```

同 ID、同内容是重投，本批只保留一份。同 ID、不同内容是主键冲突，不会被当成正常重复。

跨批次重复由 ClickHouse 处理。每条记录单独使用 ID 作为幂等 token：

```go
clickhousego.Context(ctx,
	clickhousego.WithAsync(true),
	clickhousego.WithSettings(clickhousego.Settings{
		"async_insert_deduplicate":                           1,
		"insert_deduplicate":                                 1,
		"insert_deduplication_token":                         eventID,
		"deduplicate_blocks_in_dependent_materialized_views": 1,
	}),
)
```

Analytics 先写 ClickHouse，整批持久化成功后再提交 Kafka offset。进程在两步之间退出时，消费组会重投，但 token 不变。源表还使用 `ReplacingMergeTree`，作为后台合并时的最后一层保护。

这个顺序在 `RequestConsumer` 中是显式的：

```go
if err := c.recorder.Save(runCtx, decoded.records); err != nil {
	return fmt.Errorf("record requests: %w", err)
}
if err := c.client.CommitUncommittedOffsets(runCtx); err != nil {
	return fmt.Errorf("commit Kafka offsets: %w", err)
}
```

ClickHouse 的去重窗口有限，适用于在线重试和常见故障恢复。将很久以前的备份重新灌入时，应按时间范围重建明细和聚合，不能假设旧 token 永久有效。当前 Compose 使用 ClickHouse 26.7；依赖物化视图去重时不应降级到 26.1 之前。

## ID 不能证明无丢失

`RequestRecord.id` 只在 ALS 成功解析后生成。Envoy 在发送前丢掉的日志没有 ID，自然也无法通过检查 ID 发现。Envoy 在新的 gRPC message 中重发同一请求时，ALS 会生成新 ID，因为官方 ALS 协议没有提供可持久、可确认的日志序列号。

完整性需要同时查看 Envoy 发送侧丢弃、ALS 的 `valid/rejected/discarded`、WAL 积压和 Analytics 消费进度。ID 是幂等工具，不是完整性证明。

## 源码入口

- `internal/als/service/request_record.go`：记录 ID 生成
- `internal/als/data/kafka/publisher.go`：Kafka key 和消息编码
- `internal/analytics/server/request_record.go`：批内去重和冲突检查
- `internal/analytics/server/request_consumer.go`：先入库、后提交 offset
- `internal/analytics/data/clickhouse/request.go`：每条记录的 ClickHouse 幂等 token

## 上游资料

- [Google UUID 包](https://pkg.go.dev/github.com/google/uuid)
- [ClickHouse 26.1：异步插入与物化视图去重](https://clickhouse.com/blog/clickhouse-release-26-01)
