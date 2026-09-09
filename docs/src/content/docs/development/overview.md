---
title: 开发者文档
description: 从代码入口、运行边界和失败语义理解 Ingate 的当前实现
---

这里记录已经落地的组件设计。内容以代码、配置和测试为准，重点说明设计依据、故障行为，以及哪些保证并不存在。

产品概念和页面操作仍放在[概念与架构](../../concepts/architecture/)与[使用指南](../../guide/traffic/gateway/)中。部署、指标和告警处置属于[运维文档](../../operations/overview/)。尚未实现的提案留在仓库的 `docs/design`，不混进当前行为说明。

## 组件设计

| 组件 | 设计文档 | 覆盖内容 |
| --- | --- | --- |
| ALS | [设计总览](../architecture/als/) | Envoy 日志进入 Kafka 的流程；Kafka 失败后的落盘、恢复、去重和观测 |

## 阅读代码的顺序

理解一个组件时，先从外部协议确认输入，再沿主流程走到持久化边界，最后阅读后台任务、探针和测试。以 ALS 为例：

1. `internal/als/service`：Envoy ALS 协议和 `RequestRecord` 转换
2. `internal/als/biz`：Kafka 直写、磁盘降级和回放状态迁移
3. `internal/als/data/kafka`：Producer 配置、Topic 契约和错误分类
4. `internal/als/data/diskqueue`：WAL 条目、容量、排他锁和恢复
5. `internal/als/server`：gRPC、HTTP 探针、Topic 检查和回放任务
6. `internal/als/metrics` 与 `hack/als-e2e`：观测信号和故障场景验证

开发者文档不会逐文件复述源码。代码本身已经能说明的控制流不再抄一遍；文档主要补足跨包约束、失败窗口和方案边界。
