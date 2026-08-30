---
title: 路由
description: 配置 API 与 AI 请求匹配、访问方式和目标服务
---

Route 决定哪些外部请求进入网关，以及请求要转发到哪个 Service。API Route 和 AI Route 使用同一条 `Gateway → Route → Service` 路径。

## API Route

创建 API Route 时依次配置：

1. **生效网关**：选择一个或多个 Gateway
2. **匹配条件**：Host、路径和 Method
3. **访问方式**：公开，或要求调用方密钥
4. **目标服务**：选择 HTTP Service 和相对权重
5. **转发设置**：Host 改写、超时和重试

匹配语义如下：

- 多个 Host 或 Method 之间是 OR
- Admin API 配置的多个 Header 精确匹配条件必须全部满足；当前 Console 可以查看但不新增 Header 匹配
- 路径支持前缀匹配和精确匹配
- Host 留空时只使用 Gateway 的 Host 范围

同一入口上有多条 Route 可匹配请求时，依次按更长的路径、精确路径、更多 Header 条件和明确 Method 提高优先级。若两条 Route 能命中同一请求且这些条件仍无法区分优先级，Controller 会拒绝发布，而不会依赖资源 ID 或写入顺序决定流量去向。

多个目标按权重分配流量。权重是相对值，`80:20` 与 `4:1` 表达相同分配比例。

## 超时与重试

未配置请求超时时，默认总超时为 30 秒。显式值范围为 100–300000 毫秒。

重试次数表示首次尝试失败后最多再调用 Service 的次数，范围为 1–5；单次尝试超时范围为 100–60000 毫秒，并且不能大于请求总超时。当前对连接失败、连接拒绝、连接重置和 5xx 响应重试。多目标 Route 重试时仍从符合条件的目标中选择线路。

## AI Route

AI Route 发布一个稳定的客户端模型名，并选择一个或多个模型 Service 线路。每条线路同时指定：

- 模型 Service
- 厂商实际模型名
- 相对权重

客户端只看到 Route 发布的模型名。切换模型厂商、真实模型或线路权重时，不需要改客户端调用地址。

AI Route 使用 OpenAI Chat Completions 作为当前统一入口。AI ExtProc 在转发前完成模型名改写、厂商凭据注入和必要的协议转换。

## 公开与受保护

- **公开**：不要求 Ingate 调用方密钥，适合公开 API 或已有自身鉴权的服务
- **受保护**：请求必须携带 Caller 签发的访问密钥，并且该 Caller 被授权访问当前 Route

公开 Route 命中请求限流策略时仍会执行限流，但不会因此要求访问密钥。

## Host 转发

Route 可以控制发往上游的 Host：

- **使用服务地址**：使用目标 Service 端点的主机名，代理公开 HTTPS 服务时通常选它
- **保持请求主机**：把客户端原始 Host 继续传给上游
- **自定义主机名**：使用明确指定的 Host

例如把 `localhost:8080` 转发到 `httpbin.org` 时，应选择“使用服务地址”，否则上游可能按 Host 返回错误站点或重定向。

## 声明式资源

```yaml
apiVersion: gateway.ingate.io/v1
kind: Route
metadata:
  name: a71f5f69-69e4-43ea-b678-27d0f2d784cc
spec:
  displayName: order-api
  enabled: true
  gatewayRefs:
    - 418c2c32-646a-4ef2-8b31-5a2f08c58fc3
  match:
    path:
      type: Prefix
      value: /orders
    methods: [GET, POST]
  upstreamRefs:
    - name: 4b911c58-d614-4316-b10a-fb12cb9f138c
      weight: 100
  timeout:
    requestMillis: 30000
```

Route 与 Service 的引用始终使用不可变 ID，不使用可编辑名称。
