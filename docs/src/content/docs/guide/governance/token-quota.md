---
title: Token 额度
description: 限制调用方在自然日、周或月内的模型 Token 消耗
---

TokenQuotaPolicy 限制 Caller 在自然日、自然周或自然月内可以消费的模型 Token 总量。它只作用于 AI Route，不限制普通 HTTP 请求数，也不是模型厂商账单。

## 额度语义

一条策略可以同时配置每日、每周和每月上限，并引用一个或多个 Caller：

- 未配置某个周期，表示该周期不限制
- 没有关联 Caller，只保存策略，不检查也不计数
- Caller 没有命中任何已启用策略，不限制 Token
- 同时命中多条策略时，必须全部有余额

周期使用策略配置的时区计算。自然周从周一开始，Redis Key 中保存周期起点，因此同一个周期内的请求会落入同一个计数桶，周期结束后自动过期。

## 请求前检查与响应后结算

模型的实际 Token 只有在厂商返回响应后才能确定：

```text
请求进入
  → 读取当前周期累计值，已达到上限则拒绝
  → 调用模型厂商
  → 从响应 usage 读取输入与输出 Token
  → 原子增加各个命中策略的周期计数
```

请求前检查避免已经耗尽的 Caller 继续调用。最后一个并发请求仍可能让累计值略微越过上限，因为准确用量只能在响应后结算；后续请求会被拒绝。Console 会明确说明这一语义。

## 实时计数与长期分析

- Redis 保存当前周期的同步放行计数
- ClickHouse 保存请求与模型调用事实，并维护长期用量聚合

两者用途不同。Analytics 或 ClickHouse 暂时不可用不影响实时额度判断；Redis 数据丢失时，当前额度不会自动从 ClickHouse 反向重建。

Redis 读取失败时采用失败关闭。结算失败不会把模型已经成功返回的响应改成错误，避免客户端重试造成再次计费，但会记录错误供运维处理。

## 声明式资源

```yaml
apiVersion: gateway.ingate.io/v1
kind: TokenQuotaPolicy
metadata:
  name: 13afbbdf-64df-4e7b-bb5e-e3c4f53be160
spec:
  displayName: 研发助手额度
  enabled: true
  timezone: Asia/Shanghai
  callerRefs:
    - 85b19a59-6c0c-49ca-b393-0f16449818d5
  limits:
    weeklyTokens: 20000000
    monthlyTokens: 60000000
```

当前额度按模型返回的 `usage` 结算。厂商响应没有用量信息时，请求可以返回，但实时额度无法增加。
