---
title: 请求记录
description: 按单次请求查看匹配、响应和最终转发服务
---

请求记录保存单次调用经过的 Gateway、Route 和 Service，以及最终响应状态和耗时。

## 记录内容

当前只持久化排障和聚合所需的请求元数据：

- 开始时间、HTTP Method、Host 和 Path
- 响应状态、总耗时和首字节时间
- Gateway、Route 和 Service ID
- 最终端点、转发尝试次数
- Caller 归属
- AI 请求的客户端模型名、实际模型、Token 与线路尝试

请求 Header、查询参数和正文不会持久化。这样可以降低敏感数据暴露与存储体量，但也意味着请求记录不是完整流量回放系统。

## 筛选与详情

列表适合按时间范围、响应分类、Method、Gateway、Route、Service、Caller、Host 和路径前缀定位请求。页面默认查询最近 1 小时；开始时间和结束时间都是必填项，单次最多查询 90 天。默认每页 10 条，可切换 20 或 50 条。

详情页展示单次请求的处理链路和最终选择，不向普通用户展示内部 UUID、xDS 名称或 Envoy 实现字段。内部请求 ID 只在需要精确关联日志时使用，不作为默认搜索入口。

请求列表按开始时间倒序排列。打开详情时会同时使用记录 ID 和开始时间定位 ClickHouse 分区，避免跨全部保留数据扫描；这两个参数由 Console 自动处理。

## 数据延迟

记录通过异步链路写入：

```text
Envoy ALS → Ingate ALS → Kafka → Analytics → ClickHouse
```

因此请求成功后，列表出现记录可能有短暂延迟。Kafka 或 ClickHouse 故障不影响同步转发，但会增加延迟；ALS 会在 Kafka 不可用时先写本地 WAL，恢复后重放。

## ALS 可靠性模式

ALS 不会自动创建 Kafka Topic。部署前应创建请求记录 Topic，并根据环境选择可靠性模式：

- `DEVELOPMENT` 接受副本数 1、`min.insync.replicas` 1，适合本地单 Broker。
- `PRODUCTION` 要求每个分区至少 3 个副本、`min.insync.replicas` 至少为 2，并要求 WAL 开启 `sync`。

两种模式都使用幂等 Producer 和 `acks=all`。ALS 启动时检查 Topic，之后每分钟刷新一次缓存；生产模式发现 Topic 明确不满足契约时停止直写，并让 `/readyz` 返回 503。Kafka 暂时不可达但 WAL 仍可写时，ALS 保持就绪并进入降级采集，避免可观测链路故障影响业务流量。

## WAL 持久化

Kafka 写入失败后，一个有效 Envoy 批次会作为一个版本化 WAL 条目原子追加。条目保存入队时间、原始 RequestRecord protobuf、W3C Trace Context 和 CRC32C；任意记录内容损坏、未知格式版本或旧的逐记录格式都会阻止 ALS 把数据当作有效批次继续读取。

生产模式只有追加和 `fsync` 都成功才认为批次已可靠接收。进程在落盘后、返回结果前异常退出时，同一批次可能在重启后再次投递，但 RequestRecord ID 不会重新生成，消费端可以据此幂等入库。

一个 ALS 实例独占一个 WAL 目录，第二个实例无法同时打开。目录权限固定为 `0700`，锁文件和 WAL 文件固定为 `0600`，避免同一主机上的其他用户读取请求元数据。

WAL 容量按目录中文件的实际磁盘块统计，包括分段、截断临时文件、锁和存储元数据，而不是只计算 RequestRecord 的 protobuf 大小。`capacity_bytes` 限制 WAL 目录总占用，`min_free_bytes` 为同一文件系统的其他工作负载保留安全余量。tidwall/wal 允许最后一批越过分段目标，因此 ALS 将单批限制在一个分段内，并预留两个分段大小的最坏复制空间，保证队列满后仍能通过截断队首释放容量。开发模式未配置容量时使用 1 GiB，生产模式必须显式配置容量和安全余量。

物理占用达到 80% 时进入 `warning`，达到 90% 时进入 `critical`；目录容量或文件系统余量无法容纳下一次可靠追加时进入 `blocked`。这些状态由 `/readyz` 和 Prometheus 指标暴露。`blocked` 不会删除旧记录或转为内存缓存，当前 ALS 流会以 `Unavailable` 结束；回放任务仍可读取并确认旧记录，释放空间后恢复接收。Envoy 的 gRPC access logger 与业务转发异步，因此 ALS 拒绝记录不会改变已经完成的代理响应。

## 保留时间

请求明细默认保留 30 天，由 Analytics 的 ClickHouse retention 配置控制。长期趋势使用独立聚合表，不依赖无限期保存明细。
