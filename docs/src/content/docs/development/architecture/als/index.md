---
title: ALS 设计总览
description: 从 Envoy 日志入口到 Kafka、WAL 和 ClickHouse 的完整执行过程
---

Ingate ALS 接收 Envoy 产生的访问日志，将其转换为 `RequestRecord`，再交给 Kafka 和 Analytics。它不参与请求转发。即使 ALS、Kafka 或 ClickHouse 故障，Envoy 仍会继续处理业务请求；受影响的是请求记录的完整性和可见时间。

![ALS 从 Envoy 到 ClickHouse 的组件与数据边界](/ingate/images/als/architecture.svg)

## 一条记录经过的代码

下表按照实际调用顺序列出入口。阅读源码时可以沿这一列向下走，不需要先理解整个组件。

| 阶段 | 代码入口 | 本阶段完成的工作 |
| --- | --- | --- |
| 接收 | `internal/als/service.Service.StreamAccessLogs` | 从 Envoy gRPC stream 读取一个批次 |
| 转换 | `internal/als/service.parseRequestRecord` | 校验完成日志，生成 ID，删除查询参数，提取资源和 AI 元数据 |
| 选择写入目标 | `internal/als/biz.Recorder.Write` | 根据 Topic 状态和故障屏障选择 Kafka 或 WAL |
| Kafka 写入 | `internal/als/data/kafka.Client.Publish` | 为每条记录编码 Kafka message，同步等待投递结果 |
| WAL 降级 | `internal/als/data/diskqueue.Queue.Write` | 把整个批次编码成一个可校验条目并同步到磁盘 |
| WAL 回放 | `internal/als/server.DiskQueueReplayer` | 从队首读取，写 Kafka，成功后再 Commit |
| 消费 | `internal/analytics/server.RequestConsumer` | 批量消费、校验和去重，入库完成后提交 offset |
| 存储 | `internal/analytics/data/clickhouse.Store` | 使用稳定记录 ID 写明细表和物化视图 |

`Recorder.Write` 的主流程只有一次目标选择：

```go
if r.state.reserveWriteTarget(r.topic.Status().Compliant) == queueTarget {
	return r.writeQueue(ctx, records)
}

return r.writeKafka(ctx, records)
```

Topic 合规且没有历史积压时，批次直接写 Kafka。其余情况先写 WAL。Kafka 直写在取得资格后仍可能失败，所以 `writeKafka` 会建立故障屏障，再把原批次交给 `writeQueue`。完整实现见 `internal/als/biz/recorder.go`。

## 正常写入

一次正常写入包含以下动作：

1. Envoy 将已结束的 HTTP 请求放入 ALS stream。
2. ALS 校验单条日志并生成 `RequestRecord.id`。
3. Recorder 为这批记录登记一次在途 Kafka 写入。
4. franz-go 将每条 `RequestRecord` 编码成独立 Kafka message，message key 等于记录 ID。
5. `ProduceSync` 等待每条消息的结果。全部成功后，本批处理结束。
6. Analytics 消费消息并写入 ClickHouse，整批入库成功后才提交 Kafka offset。

正常路径不写本地磁盘。WAL 只处理 Kafka 不可用、Topic 不合规或已有积压的情况。

## Kafka 故障后的写入

![ALS 批次在 Kafka、WAL 和拒绝之间的处理流程](/ingate/images/als/failure-flow.svg)

Kafka 失败后，Recorder 不会只保存返回失败的子集。决定数据语义的代码是：

```go
r.state.failKafkaWrite()

if err := r.writeQueue(ctx, records); err != nil {
	return fmt.Errorf("write request records: %w", errors.Join(result.Err, err))
}
```

这里传给 `writeQueue` 的仍是原始 `records`。Kafka 结果可能不确定：Broker 已经写入消息，但确认包在网络中丢失，客户端只能看到超时。保存整批会产生可识别的重复；只保存“看起来失败”的部分可能形成无法发现的缺口。

WAL 追加成功后，本批对 ALS 而言已经可靠接收。后台回放器稍后执行 `Read -> Publish -> Commit`。只有 Kafka 确认成功才执行 Commit，因此回放是至少一次语义。

## 四段不同的可靠性

“ALS 不丢数据”会掩盖协议和存储边界。实际保证分成四段：

| 区间 | 语义 | 失败结果 |
| --- | --- | --- |
| 请求完成到 Envoy 发送缓冲 | Envoy 尽力发送 | 缓冲溢出或进程退出时，日志可能没有到达 ALS |
| ALS 收到有效记录到 Kafka/WAL | Kafka 确认或同步 WAL 二选一 | 两者都失败时终止 stream，并增加拒绝计数 |
| WAL 到 Kafka | 至少一次 | Kafka 成功而 Commit 失败时，同一 ID 会再次发布 |
| Kafka 到 ClickHouse | 至少一次，加有限窗口去重 | 入库后、提交 offset 前退出会重投；长期历史恢复需单独处理 |

请求记录用于排障、趋势分析和允许小误差的用量统计。AI ExtProc 与 Redis 在请求路径上执行 Token 额度判断，ALS 不是额度状态的事实来源。需要逐笔守恒的付费计量应使用独立计量事件、持久 outbox、复制存储和账单对账，不能只提高现有 ALS 的重试次数。

## 包的责任

| 包 | 责任 | 不承担的责任 |
| --- | --- | --- |
| `internal/als/service` | Envoy ALS 协议和外部输入转换 | Kafka 重试与持久化 |
| `internal/als/biz` | 写入目标选择、状态迁移和回放语义 | Kafka、文件系统的具体 API |
| `internal/als/data/kafka` | Producer、Topic 拓扑和错误分类 | 决定整批是否写 WAL |
| `internal/als/data/diskqueue` | WAL 编码、容量、独占访问和恢复 | 后台调度与告警 |
| `internal/als/server` | gRPC、HTTP 探针和后台任务生命周期 | 业务记录转换 |
| `internal/als/metrics` | 低基数 Prometheus 指标 | 产品维度的请求分析 |

这些包共同实现一个明确的投递流程，没有抽象成通用消息框架。Kafka 的不确定确认、WAL 的队首 Commit 和 Envoy ALS 的无响应协议都是本组件的具体约束。

## 阅读顺序

1. [阅读路线与术语](./reading-guide/)：先建立知识地图，补齐 Kafka、WAL、文件系统和 Trace 的基础概念
2. [协议入口与记录转换](./ingestion/)：Envoy stream、发送缓冲、坏记录处理和字段来源
3. [Kafka 可靠写入](./kafka/)：ISR、Producer 序列号、不确定确认和有界取消
4. [WAL 与故障恢复](./wal/)：追加、同步、分段、截断与崩溃恢复
5. [故障模型与投递语义](./failure-model/)：逐段列出确认边界、重复窗口和丢失窗口
6. [记录 ID 与幂等](./idempotency/)：三个 ID、重复窗口和 ClickHouse 去重
7. [并发与状态迁移](./concurrency/)：故障屏障、在途写入和恢复条件
8. [容量与吞吐规划](./capacity/)：从请求速率和记录大小推导 WAL 容量及回放能力
9. [ALS 可观测性](./observability/)：日志、指标、Trace、探针、SLO 和故障实验
10. [设计推演](./reasoning/)：把确认丢失、ISR 收缩、进程退出和下游故障组合起来推导
11. [验证与测量](./verification/)：区分源码、单元测试、故障演练和性能数据各自能证明什么

## 上游资料

- [Envoy gRPC ALS 协议](https://www.envoyproxy.io/docs/envoy/latest/api-v3/service/accesslog/v3/als.proto.html)
- [Envoy gRPC access logger 配置](https://www.envoyproxy.io/docs/envoy/latest/api-v3/extensions/access_loggers/grpc/v3/als.proto.html)
- [Envoy gRPC access log 指标](https://www.envoyproxy.io/docs/envoy/latest/configuration/observability/access_log/stats)
- [franz-go v1.21.0 生产与消费说明](https://github.com/twmb/franz-go/blob/v1.21.0/docs/producing-and-consuming.md)
