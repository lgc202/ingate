---
title: 协议入口与记录转换
description: Envoy ALS stream 的响应语义、批次处理和 RequestRecord 字段来源
---

ALS 的可靠性从协议入口开始。这里需要区分两件事：Envoy 是否成功完成业务请求，以及 Ingate ALS 是否已经收到对应日志。前者发生在数据面，后者发生在异步日志 stream；两者没有事务关系。

## gRPC 方法没有逐批响应

Envoy 调用官方 `StreamAccessLogs` 客户端流式 RPC。服务端持续 `Recv`，只在 stream 结束时发送一个空响应：

```go
for {
	message, err := stream.Recv()
	if errors.Is(err, io.EOF) {
		return stream.SendAndClose(new(accesslogservice.StreamAccessLogsResponse))
	}
	if err != nil {
		return err
	}

	// 节选仅保留 stream 的响应时机，批次处理代码见后文。
}
```

这段代码位于 `internal/als/service/service.go`。协议中没有“第 N 批已持久化”的响应，因此 ALS 无法让 Envoy 逐批删除一个可重放发送日志。Envoy 发送缓冲溢出或进程退出时，日志可能在到达 Ingate 前丢失。WAL 只能保护 `Recv` 已经返回后的数据。

## Envoy 先在内存中组成批次

Controller 写入 Envoy 的配置固定为 1 秒或 64 KiB 触发一次 flush，以先到者为准：

```go
const (
	alsBufferSizeBytes = 64 * 1024
	alsFlushInterval   = time.Second
)

configuration.CommonConfig.BufferFlushInterval = durationpb.New(alsFlushInterval)
configuration.CommonConfig.BufferSizeBytes = wrapperspb.UInt32(alsBufferSizeBytes)
```

`buffer_size_bytes` 是软上限，单条较大的日志仍可能让批次越过该值。这里的聚合只减少 gRPC 调用和 protobuf 外壳开销，不是持久队列。一个正常请求结束后，日志通常还会在 Envoy 内存中停留不超过一个 flush 周期；ALS 不可用、gRPC 写缓冲达到高水位或 Envoy 退出时，这批数据仍可能丢失。

Envoy 的发送侧指标需要按它们真正确认到的位置解释：

| Envoy 指标 | 已经证明 | 没有证明 |
| --- | --- | --- |
| `logs_written` | 日志进入 logger 且当时未被丢弃 | 已发送到 ALS |
| `grpc_entries_flushed` | 条目已写入 gRPC send buffer | ALS 已收到、Kafka 或 WAL 已持久化 |
| `grpc_entries_flush_failed` | 本次 stream 创建或写缓冲提交失败 | 条目最终一定丢失，下一次 flush 仍可能成功 |
| `logs_dropped` | Envoy 因网络或应用侧积压丢弃了条目 | ALS 能够补回这条记录 |

`StreamAccessLogs` 的流量控制会把 ALS 处理变慢逐步传回 Envoy，但不会暂停业务请求。积压最终耗尽日志缓冲时，Envoy 选择丢日志而不是阻塞代理流量。这也是端到端完整率不能只看 ALS 指标的原因。

## Node ID 绑定在 stream 上

Envoy 只保证首条消息带 `identifier`。ALS 从首条消息取得 Node ID，后续消息沿用它；同一 stream 中途换 Node ID 会被拒绝：

```go
if identifier == nil {
	if current == "" {
		return "", status.Error(codes.InvalidArgument, "envoy node identity is required")
	}
	return current, nil
}

nodeID := identifier.GetNode().GetId()
if identifier.GetLogName() != requestrecord.StreamName ||
	nodeID == "" ||
	(current != "" && nodeID != current) {
	return "", status.Error(codes.InvalidArgument, "envoy access log stream identity is invalid")
}
```

Node ID 用于定位发送实例，不参与记录 ID 生成。Envoy 或 ALS 重启导致的 Node ID 变化，不会影响 WAL 中已有记录的主键。

## 单条坏记录不会丢掉整个批次

一个 Envoy message 可以携带多条 HTTP 日志。ALS 逐条转换，保留成功项，同时统计第一条错误和丢弃数量：

```go
for _, entry := range entries {
	record, err := parseRequestRecord(nodeID, entry)
	if err != nil {
		discarded++
		if firstErr == nil {
			firstErr = err
		}
		continue
	}
	records = append(records, record)
}
```

错误记录已经不可能通过重连变成合法数据。若服务端因为一条坏记录关闭整个 stream，Envoy 只会再次发送后续日志，不能修复该条内容，还可能制造重连噪声。因此无效项计入 `records_discarded_total`，同批有效项继续进入 Recorder。

Kafka 与 WAL 都失败时处理不同。此时有效记录没有可靠去向，`Recorder.Write` 返回错误，协议层以稳定的 `Unavailable` 关闭 stream：

```go
if err := s.recorder.Write(ctx, records); err != nil {
	return "", status.Error(codes.Unavailable, "request record storage is unavailable")
}
```

响应不会包含 Kafka 地址、WAL 路径或内部错误。具体原因在 ALS 日志、指标和 `/readyz` 中观察。

## 只接受已经结束的 HTTP 请求

`validateAccessLogEntry` 要求日志包含 common、request、response 和 start time，并只接受 `NotSet` 或 `DownstreamEnd`：

```go
logType := common.GetAccessLogType()
switch logType {
case accesslogdata.AccessLogType_NotSet, accesslogdata.AccessLogType_DownstreamEnd:
default:
	return fmt.Errorf("HTTP access log type %s is not a completed request", logType)
}
```

周期日志描述的是仍在执行的请求。若周期日志和结束日志都进入 Analytics，同一次调用会被统计多次，流式 AI 请求的 Token 也可能只记录到中间值。

TCP 日志不会进入存储。Ingate 当前的数据面只处理 HTTP 流量，意外收到 TCP batch 时只增加丢弃计数，不关闭 stream。

## RequestRecord 的字段来源

转换发生在 `internal/als/service/request_record.go`。下面是决定身份与敏感数据边界的部分：

```go
record := &alsv1.RequestRecord{
	Id:        uuid.NewString(),
	RequestId: request.GetRequestId(),
	StartedAt: timestamppb.New(common.GetStartTime().AsTime()),

	Method: request.GetRequestMethod().String(),
	Host:   requestHost(host),
	Path:   requestPath(path),

	GatewayId:   gatewayID,
	RouteId:     routeID,
	UpstreamId:  common.GetUpstreamCluster(),
	EnvoyNodeId: nodeID,
}
```

| 字段 | 来源 | 转换规则 |
| --- | --- | --- |
| `id` | ALS | 每条成功转换的记录生成 UUID v4 |
| `request_id` | Envoy | 原样保留，只用于跨系统关联 |
| `gateway_id`、`route_id` | Controller 生成的 route name | 解析 `ingate-route/<gateway-id>/<route-id>/...` |
| `upstream_id` | Envoy upstream cluster | 保存最终选中的 Service 内部 ID |
| `host` | AI 元数据或 HTTP authority | 只保留派生后的主机名，移除端口 |
| `path` | AI 元数据或 HTTP path | 只保留派生后的路径，在第一个 `?` 处截断 |
| `caller_id`、`access_key_id` | Authz dynamic metadata | 只保存资源 ID，不保存密钥 |
| `ai_model_call` | AI ExtProc dynamic metadata | 只有模型或 Token 字段存在时创建 |

ALS 不保存完整或原始 Header、查询参数、请求体和响应体。`host`、`path`、`request_id` 等字段来自 Envoy 已解析的请求属性，经过上表所述裁剪后单独保存。字段转换完成后还会执行领域校验，并拒绝编码后超过 64 KiB 的单条记录。

## AI Token 的数值约束

Envoy dynamic metadata 使用 protobuf `Struct`，数值实际由 `double` 承载。ALS 只接受可以无损表示的非负整数：

```go
number := numberValue.NumberValue
if number < 0 || number > maxExactMetadataInteger || math.Trunc(number) != number {
	return nil
}
return new(uint64(number))
```

`maxExactMetadataInteger` 是 `2^53 - 1`。更大的整数无法保证在 IEEE 754 double 中保持逐位精确；负数和小数也不符合 Token 计数语义。无效 Token 字段会被忽略，不会把错误数值写进分析表。字段级忽略和整条记录拒绝的边界如下：

| 输入问题 | 处理方式 | 原因 |
| --- | --- | --- |
| Token 缺失、类型不是 number、为负数、小数或超过 `2^53 - 1` | 忽略该 Token 字段 | 单个可选统计值不应拖累整条请求记录 |
| `total_tokens` 小于已有的输入/输出 Token 下界 | 拒绝整条记录 | 同一模型调用内部自相矛盾 |
| Gateway、Route、Service、Caller 或 Access Key ID 非法 | 拒绝整条记录 | 资源关联不能写入不可查询的标识 |
| Gateway/Route、Caller/Access Key 或 AI 上游关联不完整 | 拒绝整条记录 | 半条关联会破坏后续聚合语义 |
| 模型名或上游协议非法 | 拒绝整条记录 | 下游不能可靠解释该模型调用 |
| 编码后超过 64 KiB | 拒绝整条记录 | Kafka 与 WAL 都要求明确的单条上限 |

## 源码入口

- `internal/als/service/service.go`：stream、Node ID 和批次处理
- `internal/als/service/request_record.go`：字段转换和敏感数据裁剪
- `api/als/v1/request_record.proto`：跨进程记录协议
- `internal/pkg/requestrecord/validation.go`：跨 ALS 与 Analytics 复用的字段校验

## 上游资料

- [Envoy gRPC ALS 协议](https://www.envoyproxy.io/docs/envoy/latest/api-v3/service/accesslog/v3/als.proto.html)
- [Envoy gRPC access logger 缓冲配置](https://www.envoyproxy.io/docs/envoy/latest/api-v3/extensions/access_loggers/grpc/v3/als.proto.html)
- [Envoy gRPC access log 指标](https://www.envoyproxy.io/docs/envoy/latest/configuration/observability/access_log/stats)
