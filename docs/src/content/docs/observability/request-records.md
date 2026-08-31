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

## 保留时间

请求明细默认保留 30 天，由 Analytics 的 ClickHouse retention 配置控制。长期趋势使用独立聚合表，不依赖无限期保存明细。
