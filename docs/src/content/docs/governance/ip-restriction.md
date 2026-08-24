---
title: IP 访问限制
description: 通过允许列表或拒绝列表限制 Gateway 与 Route
---

IP 访问限制适合内网接口、运维入口和固定办公网等来源范围明确的场景。策略可以复用到多个 Gateway 或 Route。

## 配置规则

一条策略必须在允许列表和拒绝列表中二选一：

- **允许列表**：只有列表中的地址可以访问，其他请求返回 `403`
- **拒绝列表**：列表中的地址被拒绝，其他请求正常通过

条目支持 IPv4、IPv6 和 CIDR。单个 IP 会被规范化为 `/32` 或 `/128`，重复网段会在保存时去除。

多个策略同时命中时，请求必须通过全部策略。同一策略同时挂到 Gateway 和其下 Route 时只执行一次。

## 客户端地址

当前使用 Envoy 连接看到的来源 IP，不直接信任客户端可以伪造的 `X-Forwarded-For`。如果 Ingate 前面还有负载均衡或反向代理，需要先建立可信代理边界并保留真实源地址，否则策略看到的是前置代理地址。

策略解析失败或来源地址无法解析时采用失败关闭，避免安全配置失效后静默放行。

## 声明式资源

```yaml
apiVersion: gateway.ingate.io/v1
kind: IPRestrictionPolicy
metadata:
  name: 0fba1dca-86cc-426e-99b7-d0047e092414
spec:
  displayName: 内部接口允许列表
  enabled: true
  targetRefs:
    - kind: Route
      name: 93c0ca26-ff54-4b18-9da7-73ea51347395
  allow:
    - 10.0.0.0/8
    - 192.168.1.20/32
```

`targetRefs` 可以为空，表示策略已保存但尚未应用。IP 判断由 Controller 编译为 Envoy 原生 RBAC Route 配置，不需要 Redis 或外部判断服务。

当前不支持按国家、JWT 身份、用户角色、Header、Query 参数或时间段限制；这些条件不应被塞进同一种 IP 规则。
