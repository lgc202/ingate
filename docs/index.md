---
layout: home
title: Ingate 文档
---

Ingate 是一个基于官方 Envoy 的声明式 API 与 AI 网关。普通 HTTP 服务和模型服务共享统一的流量路径：

```text
Gateway -> Route -> Service
```

## 开始了解

- [项目架构](architecture.html)
- [安装与运维](operations.html)
- [插件体系](plugins.html)
- [Gateway](resources/gateway.html)
- [Route](resources/route.html)
- [Service](resources/upstream.html)
- [Certificate](resources/certificate.html)

## 治理策略

- [IP 访问限制](resources/ip-restriction-policy.html)
- [流量限制](resources/rate-limit-policy.html)
- [Token 额度](resources/token-quota-policy.html)

快速开始和当前能力范围请先阅读 [项目 README](https://github.com/lgc202/ingate#readme)。
