---
title: 请求限流
description: 按共享范围、客户端 IP 或 Header 值限制请求频率
---

RateLimitPolicy 为 Gateway 或 Route 设置共享请求速率上限。用户只配置限流语义，不需要了解 Envoy descriptor、Redis Key 或内部服务地址。

## 计数对象

当前支持三种对象：

- **共享**：目标下所有请求共享额度
- **客户端 IP**：每个来源 IP 独立计数
- **请求 Header**：每个指定 Header 值独立计数

策略应用到 Route 时只统计该 Route；应用到 Gateway 时统计该 Gateway 下全部请求。一条策略引用多个目标时，每个目标有独立额度。

## 令牌补充

`requests` 表示令牌桶容量，`windowSeconds` 表示补满这些令牌所需的时间。例如 `100/60` 表示桶内最多保存 100 个请求令牌，并以每 60 秒补充 100 个令牌的速度持续恢复。请求消耗一个令牌，令牌耗尽后必须等待新令牌，不会在整分钟边界一次性重置。

多个 Authz 实例通过 Redis 共享 GCRA 计数状态，Redis 原子完成速率计算和额度消费。超限返回 `429 Too Many Requests` 和 `Retry-After`；Redis 不可用时返回 `503`，不会绕过已声明规则。

多个策略命中时必须全部通过。每条策略独立统计请求尝试，因此后续规则拒绝请求时，前面的策略已经计入的尝试不会回滚。

## 声明式资源

```yaml
apiVersion: gateway.ingate.io/v1
kind: RateLimitPolicy
metadata:
  name: 7c61aa86-3727-42fd-85d7-97c14f463875
spec:
  displayName: 登录接口限流
  enabled: true
  targetRefs:
    - kind: Route
      name: 93c0ca26-ff54-4b18-9da7-73ea51347395
  subject:
    type: IP
  limit:
    requests: 100
    windowSeconds: 60
```

## 执行路径

```text
Request → Envoy ext_authz → Ingate Authz → Redis
                          ↘ allow / 429 / 503
```

公开 Route 只有在命中请求限流时才调用 Authz，不会因此要求 Caller 密钥。Header 计数对象的原始值不会写入 Redis Key 或日志。

当前不提供单实例计数、独立突发容量、自定义失败策略或任意复合计数 Key。桶容量与 `requests` 相同，不需要额外配置。
