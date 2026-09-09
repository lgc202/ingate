---
title: 服务
description: 接入 HTTP 服务或模型厂商并管理连接方式
---

Service 描述 Ingate 如何连接真实上游。Route 只选择 Service，不直接保存网络地址或厂商凭据。

Console 使用 **Service** 作为产品名称；声明式 API 当前使用 `Upstream` 表达同一个对象。

## HTTP Service

HTTP Service 配置一组端点：

- 地址：IP 或 DNS 名称
- 端口
- 权重：同一 Service 内端点的相对流量比例
- TLS：是否使用 HTTPS，以及证书校验使用的服务器名称
- 负载均衡：轮询或最少请求
- 健康检查：路径、间隔和超时

未配置 TLS 时使用明文 HTTP。启用 TLS 后，Envoy 使用系统 CA 校验上游证书，并把服务器名称同时用于 SNI 和证书身份校验。

```yaml
apiVersion: gateway.ingate.io/v1
kind: Upstream
metadata:
  name: 4b911c58-d614-4316-b10a-fb12cb9f138c
spec:
  displayName: order-service
  endpoints:
    - address: order-1.internal
      port: 8080
      weight: 1
    - address: order-2.internal
      port: 8080
      weight: 1
  loadBalancing: RoundRobin
  healthCheck:
    path: /healthz
    intervalSeconds: 10
    timeoutSeconds: 2
```

## 模型 Service

模型 Service 保存连接模型厂商所需的信息：

- 协议类型，例如 OpenAI 或 Anthropic
- 一个或多个服务地址与端口
- 是否使用 HTTPS，以及证书校验使用的服务名称
- 访问凭据
- 负载均衡与可选健康检查

厂商真实模型名不保存在 Service 顶层，而由 AI Route 的目标线路指定。这样一个厂商连接可以被多个对外模型名复用。

凭据只在服务端保存和使用，不会返回给 Console，也不会出现在请求记录中。AI ExtProc 在发往模型厂商前注入对应凭据，并删除 Ingate 内部关联 Header。

Service 的地址不包含 URL Path。AI Route 当前接收 `/v1/chat/completions`，OpenAI 兼容线路使用该路径；Anthropic 线路由 AI ExtProc 转换为 Messages 请求。

## 权重层级

两层权重解决不同问题：

- Service 端点权重：同一个逻辑服务的多个实例之间分流
- Route 目标权重：多个逻辑 Service 或模型线路之间分流

权重分流主要用于灰度、容量分担和对比验证。当前没有独立的主备或失败切换配置；需要对失败请求重试时，在 AI Route 上配置重试次数和单次超时。

## 当前边界

当前 Service 支持普通 HTTP 与模型服务。MCP 尚未进入正式能力，不会在 Console 中提前展示空类型或占位字段。
