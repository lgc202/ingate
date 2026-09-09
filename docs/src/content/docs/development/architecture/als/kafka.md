---
title: Kafka 可靠写入
description: 解释副本、ISR、acks=all、幂等 Producer 和 ALS 的 Topic 契约
---

ALS 使用 franz-go 的幂等 Producer，等待 `acks=all`，并在生产模式下要求每个目标分区至少有 3 个副本、`min.insync.replicas >= 2`。这三项解决的问题不同，不能互相替代。

![Kafka 三副本、ISR 与 acks all 的确认关系](/ingate/images/als/kafka-isr.svg)

## 副本数不是 Broker 数

`replication.factor=3` 表示 Topic 的每个分区有三份副本，通常分布在三个不同 Broker 上。集群即使有五个或十个 Broker，这个分区仍只保留三份；其余 Broker 可以承载同一 Topic 的其他分区。

每个分区有一个 Leader。Producer 写 Leader，Follower 从 Leader 复制。Kafka 把跟得上 Leader、满足存活与复制条件的副本放进 ISR（in-sync replicas）。ISR 是动态集合，不等于配置的全部副本。

## `acks=all` 的确认范围

`acks=all` 等待当前 ISR 中所有副本确认，不是等待集群全部 Broker，也不一定等到配置的全部副本。如果三副本分区当前 ISR 为 `{B1, B2, B3}`，三个都要确认；如果 ISR 已缩到 `{B1, B2}`，两个都要确认。

`min.insync.replicas=2` 是写入门槛。当 ISR 少于 2 时，即使 Leader 仍在线，`acks=all` 写入也会失败。常用的三副本、最少两个 ISR 组合允许一台副本 Broker 故障，同时避免只剩一份同步副本时继续确认写入。

| 配置 | 防护范围 | 保证边界 |
| --- | --- | --- |
| `replication.factor=3` | 分区拓扑只配置一份副本 | 三份副本始终同步或在线 |
| `min.insync.replicas=2` | ISR 只剩一个时仍确认写入 | Producer 一定请求多副本确认 |
| `acks=all` | Leader 单机写完就返回 | 一定等待配置的全部三副本 |

Kafka 官方对 `min.insync.replicas` 的说明也给出三副本、最少两个 ISR、`acks=all` 这一典型组合。它偏向持久性，代价是 ISR 不足时降低写入可用性。ALS 用本地 WAL 承接这段不可用时间，而不是把确认等级降为 `acks=1`。

## Producer 幂等的作用

Kafka 为幂等 Producer 分配 Producer ID，并按分区维护序列号。网络抖动导致客户端重试同一批时，Broker 可以识别同一 Producer 会话内的重复序列，避免把客户端内部重试写成多份。

它不等于端到端恰好一次：

- ALS 重启后通常建立新的 Producer 会话，不能用旧会话序列识别历史重放；
- Kafka 已经写入，但 ALS 只收到超时或网络错误时，整批仍会进入 WAL；
- Kafka 写入成功，而 WAL `Commit` 失败时，同一条目下次还会重放；
- Analytics 在入库后、提交消费 offset 前退出时，Kafka 会再次投递。

所以幂等 Producer 仍有价值：它消除最常见的会话内重试重复。稳定的记录 ID 和消费端去重负责更长的故障窗口。

## 幂等与 `acks=all` 的关系

幂等序列必须随着可选 Leader 的复制状态保存。若 Leader 只在本机写入就返回，随后在 Follower 复制前故障，新 Leader 既可能没有消息，也可能没有对应的序列状态，Producer 无法同时保证不丢和不重复。Kafka 因此要求幂等模式使用 `acks=all`、正数重试次数和受限的在途请求数。

Ingate 不手工覆盖 franz-go 的幂等重试与并发默认值，只明确设置 `acks=all`、Zstd 压缩和写入超时，避免一组选项彼此冲突。

## 发布错误分类

一批记录由 `ProduceSync` 返回逐条结果。ALS 先判断消息已经写入的可能性，再看错误能否重试：

| 类别 | 典型情况 | 回放行为 |
| --- | --- | --- |
| `temporary` | 客户端缓冲已满，或 Kafka 标记为可重试且能确定未写入 | 留在 WAL，指数退避后重试 |
| `uncertain` | 网络错误、上下文取消、请求或记录超时、重试耗尽、append 后 ISR 不足 | 留在 WAL；重试可能重复 |
| `permanent` | 其余无法靠重试恢复的配置或记录错误 | 队首再次确认后暂停回放 |

同一批有多个错误时，以 `permanent > uncertain > temporary` 的顺序决定批次状态。无论属于哪一类，只要有记录失败，Recorder 都把原批次完整写入 WAL。分类控制回放节奏和告警，不用来删掉“看起来已经成功”的子集。

## Topic 契约缓存

TopicMonitor 在服务接收流量前做首次检查，随后每分钟刷新。它读取每个分区的副本拓扑和 Topic 的 `min.insync.replicas`：

- 已知合规：允许 Kafka 直写；
- 已知不合规：停止直写，`/readyz` 返回 `topic_noncompliant`；
- Kafka 暂时不可达：保留最近一次有效结果；进程刚启动且没有结果时，先写 WAL。

把检查放在后台缓存中，是为了让每批写入和健康探针不再同步查询 Kafka 元数据。缓存不替代实际写入结果；Topic 合规但网络仍可能失败，Recorder 仍会降级到 WAL。

当前契约没有检查 Broker 的机架或可用区分布，也没有读取集群级的非同步副本选主配置。三个副本若落在同一故障域，不能提供跨故障域容灾；允许非同步副本成为 Leader，也可能用可用性换取数据缺口。生产部署必须在 Kafka 集群侧管理这两项，不能把“副本数为 3”理解成完整的多可用区保证。

## 参考

- [Apache Kafka Producer 配置](https://kafka.apache.org/40/generated/producer_config.html)
- [Apache Kafka Topic 配置](https://kafka.apache.org/41/generated/topic_config.html)
- [Apache Kafka 的持久性说明](https://kafka.apache.org/20/design/design/#availability-and-durability-guarantees)
- [franz-go 生产与消费说明](https://github.com/twmb/franz-go/blob/master/docs/producing-and-consuming.md)
