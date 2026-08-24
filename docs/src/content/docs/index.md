---
title: Ingate 文档
description: Ingate API 与 AI 网关的安装、配置、治理、观测和架构文档
---

Ingate 是一个基于官方 Envoy 的声明式 API 与 AI 网关。普通 HTTP 服务和模型服务使用统一的流量模型，并通过同一套控制面完成配置发布、访问治理和请求分析。

## 快速开始

1. [安装 Ingate](./getting-started/installation/)
2. [转发第一个 API 请求](./getting-started/first-api/)
3. [发布并调用第一个模型](./getting-started/first-ai-route/)

## 资源模型

```text
Gateway → Route → Service
```

- **Gateway**：监听协议、端口、域名和证书
- **Route**：请求匹配、访问方式和转发目标
- **Service**：HTTP 服务或模型厂商的连接配置

API 与 AI 是 Route 和 Service 的类型，不形成两套平行资源。AI Route 额外发布客户端模型名，并将其映射到实际模型 Service。

![Ingate 系统架构](/ingate/images/architecture/system.png)

## 文档目录

| 分类 | 内容 |
| --- | --- |
| [概念与架构](./concepts/architecture/) | 组件职责、通信链路、数据归属和资源关系 |
| [流量管理](./traffic/gateway/) | Gateway、Route、Service 和 Certificate |
| [访问治理](./governance/caller/) | Caller、IP 访问限制、请求限流和 Token 额度 |
| [插件](./plugins/overview/) | 插件源、安装、升级、Policy 和卸载 |
| [观测分析](./observability/request-records/) | 请求记录、流量分析和 AI 用量 |
| [运维](./operations/overview/) | 健康检查、日志、备份、恢复和升级 |
| [声明式 API](./reference/declarative-api/) | 资源结构、List/Watch、版本和 Status |

## 已实现能力

| 领域 | 能力 |
| --- | --- |
| 流量入口 | HTTP/HTTPS、域名、证书和多个逻辑 Gateway |
| 路由转发 | Host、路径、Method 与 Header 匹配；加权目标、Host 改写、超时与重试 |
| 模型接入 | OpenAI Chat Completions、Anthropic Messages、流式响应和多模型线路 |
| 访问治理 | Caller 密钥、Route 授权、IP 访问限制、请求限流和 Token 额度 |
| 插件扩展 | 插件源、安装、升级、依赖检查和卸载 |
| 观测分析 | 请求记录、响应分布、耗时趋势、资源排行和模型 Token 用量 |

:::note
Ingate 当前处于 `0.x` 阶段。本文档只描述已经实现的能力。MCP、Agent 编排和 Kubernetes CRD 等内容不在当前范围。
:::
