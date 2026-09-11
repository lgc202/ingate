---
title: 故障模型与投递语义
description: 沿 Envoy、ALS、Kafka、WAL 和 Analytics 的确认点说明哪里可能丢失、重复或延迟
---

ALS 不是一条端到端事务。Envoy 不等待逐批确认，Kafka 的成功响应也不能覆盖后续 ClickHouse 入库。判断可靠性时，应先确定数据已经越过哪个确认点。

## 七个确认点

```text
请求结束
  -> E1: 进入 Envoy logger
  -> E2: 写入 gRPC send buffer
  -> E3: ALS Recv 返回并完成记录校验
  -> E4a: Kafka 返回成功
     或 E4b: WAL 追加并 Sync 成功
  -> E5: WAL 回放后 Kafka 返回成功
  -> E6: Analytics 写入 ClickHouse 成功
  -> E7: Analytics 提交 Kafka offset
```

这些点表达的事实不同：

| 确认点 | 此时能够确认 | 此时仍不能确认 |
| --- | --- | --- |
| E1 | Envoy logger 当时接受了日志 | 已 flush、ALS 已收到 |
| E2 | 日志已交给 gRPC send buffer | ALS 已 `Recv`、服务端已持久化 |
| E3 | ALS 已拥有一条带稳定 ID 的合法记录 | Kafka 或磁盘已经保存 |
| E4a | Kafka 已按当前 ISR 契约确认消息 | Analytics 已消费和入库 |
| E4b | 当前 ALS 实例的本地 WAL 已同步条目 | 其他节点持有副本、Kafka 已恢复 |
| E5 | WAL 中的记录已进入 Kafka | WAL Commit 已成功 |
| E6 | ClickHouse 写入调用成功 | Kafka offset 已提交 |
| E7 | 当前消费组不会因这次处理再次读取该 offset | ClickHouse 永远不存在历史重复 |

在 Kafka 已确认的 E4a 之后，Kafka 到 Analytics 使用至少一次消费；在 E4b 之后，WAL 到 Kafka 使用至少一次回放。这些保证以 Kafka 保留数据、本地 Volume 可恢复为前提，不能向前覆盖 E3 之前的 Envoy 缓冲，也不能把本地 WAL 变成跨节点复制日志。

## 主要故障的结果

| 故障位置 | 可能结果 | 系统如何处理 | 主要观测信号 |
| --- | --- | --- | --- |
| Envoy flush 前退出 | 日志丢失 | 无法恢复，记录尚未获得 Ingate ID | Envoy 重启；ALS 无对应记录 |
| gRPC 写缓冲积压 | 延迟，缓冲耗尽后可能丢失 | Envoy 保持业务转发，日志侧施加流量控制并允许丢弃 | Envoy `logs_dropped`、`grpc_entries_flush_failed` |
| ALS 在 E3 与 E4 之间退出 | 本批可能丢失 | 官方 ALS 没有逐批 ACK 或服务端 inbox，不能承诺 Envoy 重发 | ALS 重启；入口计数可能尚未增加 |
| Kafka 明确失败，WAL 可写 | 延迟 | 原批次同步追加 WAL，后续新批次也直接入队 | `spooling=1`、WAL 积压增加 |
| Kafka 已写入但 ACK 丢失 | 重复 | 超时后原批次写 WAL；回放保留相同记录 ID | `uncertain` 发布失败、同 ID 重复 |
| Kafka 失败且 WAL 已满或不可写 | 未取得完整确认 | ALS 增加 rejected 并以 `Unavailable` 结束 stream | `/readyz` 503、`records_rejected_total` |
| WAL 所在 Volume 丢失 | 未回放记录丢失 | 当前没有跨节点副本；从备份恢复也可能超出去重窗口 | Volume 和主机告警 |
| Kafka 成功、WAL Commit 失败 | 重复 | 条目保持队首，下次再次发布相同 ID | replay 增加但 committed 不增加 |
| ClickHouse 成功、offset 提交前退出 | 重复消费 | Analytics 重读消息并使用相同 ID 入库 | Consumer lag、重复计数 |
| 队首消息是永久错误 | 后续积压停住 | 回放暂停，不越过坏条目 | `replay_paused=1` |

## Exactly once 在这里不成立

Exactly once 需要所有参与方共享同一个可恢复提交协议。当前链路跨越 Envoy 内存缓冲、无逐批响应的 gRPC stream、本地文件、Kafka 和 ClickHouse，任何一个稳定 ID 都不能把这些系统合成一笔原子事务。

Ingate 采用的是更具体的组合：

- Envoy 到 ALS 为尽力发送，业务流量优先；
- 合法记录在 ALS 内取得 Kafka 确认或同步 WAL 确认；
- WAL 到 Kafka 为至少一次；
- Kafka 到 ClickHouse 为至少一次，并在已知重试窗口内按记录 ID 去重。

这套语义适合请求排障、趋势分析和允许小误差的用量统计。若数据被定义为逐笔收费事实，需要在请求执行侧产生不可变计量事件，写入复制存储或事务 outbox，并增加账单对账和差错修复；不能把 ALS 的本地 WAL 当成账本。

## 用故障实验验证语义

测试不应只验证“最终有数据”，还要验证故障发生在哪个确认点：

| 实验 | 应验证的结果 |
| --- | --- |
| Kafka 停止后继续请求 | 记录进入 WAL，业务请求仍成功 |
| WAL 有积压时重启 ALS | 启动扫描恢复相同 ID，随后回放并 Commit |
| 注入 ACK 丢失 | Kafka 最终出现至少两条同 ID 消息，内容一致 |
| Kafka 故障并填满 WAL | ALS `/readyz` 返回 503，Envoy 转发仍可用 |
| Kafka 恢复后在 Publish 与 Commit 之间退出 | 重启后再次投递相同 ID |
| 损坏中间 WAL 条目 | ALS 启动失败，不跳过条目继续回放 |

当前 `make als-e2e` 覆盖其中的正常写入、Kafka 故障恢复、ACK 丢失、WAL 满和条目损坏。Publish 与 Commit 之间的进程退出仍主要由单元测试中的可控依赖验证。

## 上游资料

- [Envoy gRPC ALS 协议](https://www.envoyproxy.io/docs/envoy/latest/api-v3/service/accesslog/v3/als.proto.html)
- [Envoy gRPC access log 指标](https://www.envoyproxy.io/docs/envoy/latest/configuration/observability/access_log/stats)
- [Apache Kafka 的持久性说明](https://kafka.apache.org/20/design/design/#availability-and-durability-guarantees)
- [franz-go v1.21.0 Producer 配置源码](https://github.com/twmb/franz-go/blob/v1.21.0/pkg/kgo/config.go)
