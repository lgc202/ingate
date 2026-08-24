---
title: 认识 Ingate
description: Ingate 的产品对象、适用场景与部署边界
---

Ingate 是一个基于官方 Envoy 的声明式 API 与 AI 网关。它提供控制台和声明式 API，用来管理入口、路由、服务连接、访问治理、插件与请求分析。

## 产品模型

所有业务流量都使用同一条路径：

```text
Gateway -> Route -> Service
```

- **Gateway**：监听端口、协议、域名和 TLS 证书
- **Route**：请求匹配、访问方式、转发目标和流量行为
- **Service**：实际连接的 HTTP 服务或模型厂商；声明式 API 中对应 `Upstream`

API、AI 是 Route 与 Service 的类型，而不是两套平行资源。AI Route 发布客户端使用的稳定模型名，真实厂商、API Key 和模型连接属于模型 Service。

## 适合的场景

- 为内部或外部 HTTP API 提供统一入口
- 用同一套网关接入不同模型厂商，并向客户端发布稳定模型名
- 为应用或服务签发访问密钥，并限制可访问的 Route
- 在 Gateway 或 Route 上应用 IP 访问限制、请求限流等治理策略
- 采集请求元数据，查看响应状态、转发结果、耗时和模型 Token 用量

## 一套 Ingate 表示什么

一套 Ingate 对应一个环境、一个配置域和一组配置完全相同的 Envoy 实例，其中可以创建多个逻辑 Gateway。

生产、测试、不同机房或需要强隔离的租户应部署多套 Ingate。资源不会跨 Ingate 实例共享，控制面也不会在一个实例内维护环境选择器。

## 设计边界

- Envoy 是唯一数据平面，不维护 Ingate 私有 Envoy 分支
- 不为 Kong、Nginx 等其他数据平面预设适配抽象
- API Server 是 etcd 的唯一访问者
- 声明式资源是配置事实来源，Controller 重启后重新全量编译
- 请求记录不保存请求 Header、查询参数或正文
- Docker Compose 是当前优先支持的安装方式，不是业务代码的部署依赖

下一步可以直接[安装 Ingate](../installation/)，或先阅读[系统架构](../../concepts/architecture/)。
