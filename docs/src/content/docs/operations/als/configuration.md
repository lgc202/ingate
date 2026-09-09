---
title: ALS 配置
description: 配置 Kafka 可靠性契约、本地磁盘队列、回放和 Trace 出口
---

ALS 的默认配置适合本地单 Broker 联调。生产环境需要显式切换可靠性模式，并为 Kafka Topic 与 WAL 提供相匹配的配置。

## 数据层配置片段

下面只展示 ALS 数据层相关字段，不能作为可直接启动的完整配置。部署时仍需提供 `server.grpc`、`server.http`、`shutdown`、`logging` 和 `telemetry`；生产环境还应按基础设施要求补充 gRPC、Kafka 与 OTLP 的 TLS、认证和凭据注入。完整字段结构以 `deploy/docker/configs/ingate-als.yaml` 和对应 protobuf 配置定义为准。

```yaml
data:
  reliability_mode: "PRODUCTION"
  kafka:
    brokers: ["kafka-1:9092", "kafka-2:9092", "kafka-3:9092"]
    topic: "ingate.request-records"
    write_timeout: "5s"
    dial_timeout: "5s"
    topic_check_timeout: "2s"
  disk_queue:
    path: "/data/ingate/als/queue"
    segment_bytes: 20971520
    capacity_bytes: 10737418240
    min_free_bytes: 2147483648
    replay_batch_size: 500
    replay_min_backoff: "0.5s"
    replay_max_backoff: "30s"
    sync: true
```

容量数字只是示例，不能直接当作生产建议。先在接近生产流量的环境测量一段时间内 WAL 目录的物理增量，再计算单条记录的平均物理占用：

```text
基础容量 = 峰值记录速率 × 平均物理字节/记录 × 目标缓冲时间
```

目标缓冲时间要包含发现故障、人工恢复 Kafka，以及 Kafka 恢复后边回放边接收新记录的时间。回放吞吐必须高于持续写入速率，否则队列不会排空。计算结果之外还要留下下面说明的截断恢复空间和文件系统安全余量。

## Kafka

`brokers` 是发现集，不要求列出集群全部 Broker。客户端通过元数据找到目标分区的 Leader。

`write_timeout` 限制一批记录等待 Kafka 最终结果的时间。设得过短会增加“Broker 可能已经写入，但客户端没收到确认”的不确定失败；设得过长会拖慢进入 WAL 的速度。应结合跨机房延迟、Broker 负载和故障切换时间调整。

ALS 不创建 Topic。新建 Topic 时需要同时指定分区数、副本数和最小同步副本数。下面的 `12` 只是命令示例；实际分区数由峰值写入吞吐和 Analytics 消费并行度决定：

```bash
bin/kafka-topics.sh \
  --bootstrap-server kafka-1:9092 \
  --create \
  --topic ingate.request-records \
  --partitions 12 \
  --replication-factor 3 \
  --config min.insync.replicas=2
```

已有 Topic 可以直接修改 `min.insync.replicas`：

```bash
bin/kafka-configs.sh \
  --bootstrap-server kafka-1:9092 \
  --entity-type topics \
  --entity-name ingate.request-records \
  --alter \
  --add-config min.insync.replicas=2
```

副本数不是普通 Topic 配置，已有 Topic 不能用上面的命令从 1 改成 3；需要先为每个分区生成并审核副本重分配方案，再按 Kafka 的分区重分配流程执行。完成后验证实际拓扑：

```bash
bin/kafka-topics.sh \
  --bootstrap-server kafka-1:9092 \
  --describe \
  --topic ingate.request-records
```

每个分区都应列出至少三个 Replica，并在健康状态下有足够 ISR；Topic 配置中应显示 `min.insync.replicas=2`。Producer 固定使用 `acks=all`。Topic 每分钟复查一次；检查请求暂时失败时保留最近一次可确定的结果，首次检查无法完成则使用 WAL。

生产模式接受更高的值，例如副本数 4、`min.insync.replicas` 3。副本数取所有分区中的最小值，因而任何一个分区低于 3 都不合规。这里检查的是配置的副本拓扑和 Topic 的 `min.insync.replicas`，不是当前 ISR；运行时 ISR 不足由 Kafka 写入结果和告警反映。

## 磁盘队列

`segment_bytes` 同时限制 WAL 分段目标和单个编码批次。单批超过该值会被拒绝，避免无法给截断恢复空间设定上界。

单条 `RequestRecord` 最多编码为 64 KiB。用 `ingate_als_batch_records` 观察 Envoy 批次规模，并用接近上限的记录做容量测试，确认最大编码批次小于 `segment_bytes`。只看平均批次会漏掉低频的大批次拒绝。

`capacity_bytes` 限制队列目录的物理占用，不是 protobuf 载荷总和。分段、锁文件、文件系统分配块和截断时的临时副本都会占空间。ALS 还会保留至少两个分段的恢复余量；因此容量必须大于 `2 × segment_bytes`。

`min_free_bytes` 是留给同一文件系统其他工作负载的安全余量。生产模式要求它大于零。容量预算同时受目录上限和文件系统剩余空间限制，任何一项不足都会阻止新追加。

`sync: true` 表示 WAL 追加在文件同步成功后才返回。生产模式强制启用；关闭后，进程崩溃通常还能从页缓存恢复，但主机掉电或内核崩溃可能丢掉已经返回成功的数据。

## 回放

`replay_batch_size` 限制一轮读取的记录数，但不会拆开原始 WAL 条目。若一个条目本身超过限制，仍会完整返回。

临时失败后，等待时间从 `replay_min_backoff` 指数增加到 `replay_max_backoff`，并加入向上的随机抖动。永久错误会暂停本进程的回放，需要先修复 Topic、认证或消息限制，再重启 ALS 重新判断队首。

## Trace

`telemetry.tracing.endpoint` 为空时不创建 OTLP 出口。配置后，批次接收、Kafka 发布、WAL 追加和回放会产生 Span。导出队列是有界且异步的；Collector 不可用会丢 Trace，不会阻塞请求记录投递。

正式环境通常不需要采样全部批次。根据故障诊断需求设置 `sample_ratio`，再用 Trace 丢弃指标检查出口是否过载。
