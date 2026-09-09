---
title: ALS 恢复与迁移
description: 安全处理 WAL 积压、目录迁移、备份恢复和数据损坏
---

WAL 保存 ALS 已接收但尚未向 Kafka 确认完成的请求记录。任何手工操作前都要停止对应 ALS 实例，避免复制过程中队首继续变化。一个 WAL 目录不能同时由两个进程打开。

## 先判断是积压还是损坏

Kafka 故障、Topic 不合规和临时网络错误只会造成积压。此时不要操作 WAL 文件，先恢复 Kafka，观察：

```text
ingate_als_disk_queue_records
rate(ingate_als_records_committed_total[5m])
ingate_als_disk_queue_oldest_entry_age_seconds
```

`committed` 持续增长、记录数和最老年龄下降，说明队列正在正常排空。

启动日志若报告无法打开分段、未知 `format_version`、CRC32C 不一致或某个 sequence 解码失败，则按损坏处理。当前项目没有通用的 WAL 就地修复命令，不要直接编辑、截断或删除分段。

## 迁移目录

最安全的迁移时机是 Kafka 正常且 `ingate_als_disk_queue_records` 已归零。操作顺序如下：

1. 停止对应 ALS 实例，确认进程已经退出。
2. 复制整个 WAL 目录到新的持久卷，保留权限和属主。不要只复制看起来有数据的分段，锁文件和存储元数据也属于目录状态。
3. 把原目录保留为只读回滚副本，不在原地修改。
4. 修改 `disk_queue.path`，启动一个 ALS 实例。
5. 检查启动日志、`/readyz`、`ingate_als_disk_queue_records` 和 `ingate_als_records_committed_total`。
6. 队列排空并稳定运行后，再按环境的数据保留流程处置原副本。

迁移仍有积压的目录时，目标版本必须能够读取现有 `QueueEntry` 格式。当前格式版本为 1；未知版本会阻止启动，不会自动升级。

## 中间条目损坏

1. 停止实例，并对整个 Volume 做快照或复制到外部存储。
2. 记录启动日志中的 sequence、错误类型、组件版本和 WAL 目录状态。
3. 不在唯一副本上尝试修复。先从副本判断可读前缀、损坏条目和后续条目的范围。
4. 在“恢复旧备份”和“接受明确的数据缺口后创建空队列”之间做事故决策。两者都会影响数据完整性，不能由启动脚本自动选择。
5. 若要保留损坏条目之后的数据，需要编写与当前 `QueueEntry` 版本匹配的离线导出工具，并在副本上验证 CRC、记录大小和业务字段。仓库目前没有这项工具。

恢复前，应把预计丢失的 sequence 与时间范围记入事故记录。恢复后检查拒绝计数、Kafka 消费进度和 ClickHouse 可见性，而不是只看进程重新启动。

## 同时恢复 Kafka 与 WAL

Ingate 的整体备份可能同时包含 Kafka 消息和 ALS WAL，它们不是一个原子快照。恢复后，同一稳定记录 ID 可能同时出现在 Kafka 和 WAL，回放会形成重复。这是至少一次语义的预期结果。

Analytics 与 ClickHouse 的在线去重可以处理正常恢复窗口内的重复。很久以前的备份可能已经超出 ClickHouse 去重窗口；恢复此类数据前，应确定时间范围，并准备重建对应明细和聚合。不要把历史全量恢复当作普通在线重试。

## 恢复完成条件

- `/readyz` 返回 `200`；
- `ingate_als_records_rejected_total` 不再增长；
- `ingate_als_disk_queue_writable` 为 1；
- `ingate_als_records_committed_total` 在有积压时持续增长；
- 队列记录数归零，最老条目年龄归零；
- Analytics 消费继续推进，新请求能在 ClickHouse 查询到；
- 恢复期间产生的重复由稳定 ID 收敛，没有重复累计聚合。
