---
title: ALS 设计总览
description: Envoy 请求记录从协议入口到 Kafka、WAL 和 Analytics 的完整设计
---

Ingate ALS 解决的是一个窄问题：把 Envoy 产生的请求记录送进异步分析链路，并在 Kafka 短暂故障时保住已经接收的数据。它不参与代理转发，也不承担计费账本或通用消息队列的职责。

![ALS 从 Envoy 到 ClickHouse 的组件与数据边界](/ingate/images/als/architecture.svg)

## 数据路径

```text
Envoy -> gRPC ALS -> parse and validate -> Recorder
                                           |-- Kafka
                                           `-- disk queue -> replay -> Kafka
                                                                     |
                                                                 Analytics
                                                                     |
                                                                 ClickHouse
```

入口使用 Envoy 官方 `service.accesslog.v3.AccessLogService`。一个长连接承载多个批次，只有首个消息保证带 Node 标识。Ingate 只接受完成的 HTTP 日志；周期日志还没描述完一次请求，如果和结束日志同时入库，会重复统计请求和 Token。

每条有效日志转换为 `RequestRecord`。转换时去掉查询参数，不保存 Header 和正文。这样能限制敏感数据面与单条消息大小，但它也意味着请求记录不能用于流量重放。

## Kafka 优先的取舍

正常情况下直接写 Kafka，省去本地写放大和回放延迟。Kafka 写入失败、Topic 不合规或队列已有积压时，后续批次进入降级 WAL。队列排空前不恢复直写，否则新数据会越过旧数据，延迟分布和时间顺序会变得更难解释。

WAL 是故障缓冲，不是 Kafka 的替代品。它只有单机副本，容量有限，也不提供跨节点复制。长期故障应通过恢复 Kafka、扩容和告警处理，不能靠无限增大本地队列掩盖。

## 实际保证

| 区间 | 语义 | 说明 |
| --- | --- | --- |
| 业务请求完成到 Envoy 发送缓冲 | 尽力而为 | Envoy ALS 不等待服务端逐批确认，缓冲溢出或进程退出可能丢失 |
| ALS 收到有效批次到 Kafka/WAL | 可靠接收 | Kafka 已确认，或同步 WAL 追加成功 |
| WAL 到 Kafka | 至少一次 | Kafka 成功而本地 Commit 失败时会重放 |
| Kafka 到 ClickHouse | 至少一次 + 有界去重 | Analytics 先入库再提交 offset。在线重试按稳定 ID 去重 |

因此，“ALS 不丢数据”不是准确表述。更准确的说法是：它缩小了已进入采集服务后的确定丢失窗口，并让拒绝、积压和新鲜度可观测；协议上游和单机磁盘失效仍在保证之外。

AI Token 额度的同步判断由 AI ExtProc 和 Redis 完成，ALS 记录用于事后分析，不是额度状态的事实来源。少量采集缺口会让报表产生误差，但不会直接改变当次请求是否放行。若将来要按逐笔用量结算，应单独设计计量账本：在结算边界产生稳定事件，经持久 outbox 或等价机制写入复制日志，并与模型厂商账单对账。不能只给现有 ALS 再贴上“零丢失”标签。

## 失败优先级

![ALS 批次在 Kafka、WAL 和拒绝之间的处理流程](/ingate/images/als/failure-flow.svg)

一批 Kafka 写入可能部分成功。只要其中一条返回错误，Recorder 会把原批次完整写入 WAL。这里选择重复而不是缺口：已经成功的部分可能再次出现，但整批都有稳定 ID，Analytics 可以去重。若只把失败子集写入 WAL，错误分类或客户端结果存在歧义时反而可能漏掉已写入但未确认的记录。

首次直写若返回永久错误，Recorder 仍先把整批保存到 WAL。回放器读到同一队首并再次确认错误不可恢复后，才把回放状态设为 paused。这样既不在一次分类结果后丢弃批次，也不会让永久错误无限占用 Kafka 请求。

## 代码边界

| 包 | 责任 |
| --- | --- |
| `internal/als/service` | ALS 协议、外部输入校验、`RequestRecord` 转换 |
| `internal/als/biz` | 可靠投递用例、状态迁移和依赖接口 |
| `internal/als/data/kafka` | Kafka Producer、Topic 拓扑和错误分类 |
| `internal/als/data/diskqueue` | WAL 编码、排他访问、容量与恢复 |
| `internal/als/server` | 进程入口、后台回放、Topic 监测和探针 |
| `internal/als/metrics` | 低基数 Prometheus 指标 |

这几个包没有抽象出通用“投递框架”。Kafka、WAL 和 Envoy ALS 的语义是组件本身的一部分，提前泛化只会隐藏失败条件。

## 继续阅读

- [Kafka 可靠写入](./kafka/)：副本、ISR、`acks=all` 和 Producer 幂等
- [WAL 与故障恢复](./wal/)：使用本地日志的原因、确认顺序和恢复方式
- [记录 ID 与幂等](./idempotency/)：ID 的边界、重复窗口和下游去重
- [并发与状态迁移](./concurrency/)：故障屏障、在途写入和互斥锁的选择
- [可观测性与验证](./observability/)：指标、Trace、SLO 与端到端故障场景

## 上游资料

- [Envoy gRPC ALS 协议](https://www.envoyproxy.io/docs/envoy/latest/api-v3/service/accesslog/v3/als.proto.html)
- [Envoy gRPC access log 指标](https://www.envoyproxy.io/docs/envoy/latest/configuration/observability/access_log/stats)
- [franz-go 生产与消费说明](https://github.com/twmb/franz-go/blob/master/docs/producing-and-consuming.md)
