---
title: 记录 ID 与幂等
description: 解释 RequestRecord ID 的生成位置、Envoy Request ID 的用途和端到端去重边界
---

每条 `RequestRecord` 在 ALS 完成解析时生成一个 UUID v4。这个 ID 随记录一起进入 Kafka 和 WAL，Kafka message key 也使用同一个值。重放不会重新生成 ID。

![RequestRecord ID 在 ALS、Kafka、Analytics 和 ClickHouse 之间的传递](/ingate/images/als/idempotency.svg)

## 在 ALS 生成记录 ID

ID 要标识的是“一条进入 Ingate 分析链路的请求事实”，它的生命周期从 ALS 解析成功开始。此时生成有几个明确结果：

- 不依赖客户端是否发送 `x-request-id`；
- 不信任外部输入的唯一性和格式；
- 同一条记录经过 Kafka 重试或 WAL 回放时保持不变；
- API 请求 ID 与分析事实 ID 分开，各自表达自己的边界。

UUID v4 不需要共享时钟、节点编号或持久化序列。ALS 重启不会复用内存计数，也不要求 Envoy Node ID 稳定。对这条异步分析链路来说，随机碰撞概率已经远低于其他丢失或重复来源，引入 Snowflake 节点租约、数据库序列或复合业务键只会增加状态和故障点。

## Envoy Request ID 的边界

Envoy 的 Request ID 仍保存在 `request_id` 字段，用于把代理日志、上游日志和用户请求串起来，但它不适合作为存储主键：

- 客户端可以缺省、复用或伪造它；
- 重试、内部调用和跨代理传播会改变“一个 ID 对应一条事实”的含义；
- 同一个请求可能在不同观察点产生多条记录；
- 未来更换生成策略时，不应改变 Ingate 的幂等边界。

两者不是重复字段。`id` 回答“这条分析记录是谁”，`request_id` 回答“它可能和哪些请求日志相关”。

## 内容哈希不适合记录 ID

确定性哈希看起来可以让 ALS 重启后再次解析同一内容仍得到同一 ID，但真实记录很难定义稳定且无歧义的规范输入。排除时间和动态字段会把两次合法请求误判为重复；包含全部字段又会让微小格式变化生成新 ID。哈希还会把敏感字段选择、协议演进和主键稳定性绑在一起。

当前设计只承诺“记录对象创建后 ID 稳定”。如果 Envoy 在新的 gRPC 消息里重发同一请求，ALS 会再次解析并生成新 ID；这属于 ALS 协议入口之前的重复，不能靠现有 ID 消除。

## 重复产生的位置

| 窗口 | ID 是否相同 | 谁处理 |
| --- | --- | --- |
| franz-go 在同一 Producer 会话内重试 | Kafka 幂等序列处理 | Kafka Broker |
| Kafka 已写入，但 ALS 收到不确定错误后整批落 WAL | 相同 | Analytics / ClickHouse |
| WAL 已发布，Commit 失败后再次回放 | 相同 | Analytics / ClickHouse |
| Analytics 入库后、提交 offset 前退出 | 相同 | Analytics / ClickHouse |
| Envoy 在协议层重新发送，ALS 再次解析 | 不同 | 当前无法可靠识别 |

这张表说明“幂等 Producer”不等于端到端恰好一次。每层只能消除它能观察到、并拥有稳定标识的重复。

## Analytics 去重

Analytics 先写 ClickHouse，再提交 Kafka offset。一个消费批次内：

- 相同 ID、相同内容视为重投，只保留一份；
- 相同 ID、不同内容视为数据冲突，拒绝继续把它当正常重复。

写入 ClickHouse 时使用记录 ID 作为 `insert_deduplication_token`，并开启异步插入及依赖物化视图的去重设置。源表使用 `ReplacingMergeTree`，给查询与后台合并再留一层保护。

去重窗口不是无限的。当前设置覆盖在线重试和常见故障恢复；很久以前的离线历史数据若重新灌入，可能超出 ClickHouse 的去重窗口。此类操作应按时间范围重建明细和聚合，不能直接依赖旧 token 永久有效。

当前 Compose 使用 ClickHouse 26.7。ClickHouse 26.1 才修复异步插入在依赖物化视图上的端到端去重，因此不应把运行版本降到 26.1 之前而仍宣称这条保证成立。

## ID 的证明边界

稳定 ID 能把重复投递变成可识别问题，也能关联 Kafka、WAL、Analytics 和 ClickHouse。它不能证明数据从未丢失，不能替代持久化确认，也不能把没有到达 ALS 的日志补出来。

完整性要同时看：Envoy 发送侧丢弃指标、ALS 的 `valid/rejected/discarded`、WAL 积压和 Analytics 消费状态。只看 ID 是否唯一，会漏掉最重要的缺口。

## 参考

- [Google UUID 包](https://pkg.go.dev/github.com/google/uuid)
- [ClickHouse 26.1：异步插入与物化视图去重](https://clickhouse.com/blog/clickhouse-release-26-01)
