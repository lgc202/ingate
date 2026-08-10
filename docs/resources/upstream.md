# Upstream 资源

Upstream 表示一个逻辑上游服务，对应 Envoy Cluster、Kubernetes Service 或 Kong Upstream 这类成熟网关概念。应用、模型、MCP 和 Agent 共用同一资源，`type` 只表达业务分类。

Upstream 不提供启停开关。Gateway 和 Route 是流量暴露边界，应在这两类资源上控制流量是否生效；给共享 Upstream 增加开关会让所有引用它的 Route 被动失效。

## 普通服务

```yaml
apiVersion: gateway.ingate.io/v1
kind: Upstream
metadata:
  name: 10c29251-b22f-48fe-acfb-891e11a23882
spec:
  displayName: order-service
  type: Application
  endpoints:
    - address: order-1.internal.example.com
      port: 8080
      weight: 2
    - address: order-2.internal.example.com
      port: 8080
      weight: 1
  loadBalancing: LeastRequest
  healthCheck:
    path: /healthz
    intervalSeconds: 10
    timeoutSeconds: 2
```

`type` 当前支持 `Application`、`Model`、`MCP` 和 `Agent`。除模型服务需要协议转换外，其他分类都按普通 HTTP 上游处理，不要求用户重复配置 `protocol`。

Endpoint 只包含：

- `address`：DNS 名称或 IP 地址，不包含协议和端口
- `port`：范围为 `1-65535`
- `weight`：范围为 `1-1000` 的相对权重，省略时默认为 `1`

Endpoint 没有手写 ID 和独立启停状态。端点列表使用完整声明更新；暂时不接收流量的端点应从列表中移除。同一个 Upstream 内不能重复配置相同的地址和端口。

`loadBalancing` 当前支持 `RoundRobin` 和 `LeastRequest`，省略时默认为 `RoundRobin`。

## HTTPS 和健康检查

```yaml
tls:
  serverName: api.example.com
```

配置 `tls` 表示使用 HTTPS。`serverName` 同时用于 SNI 和服务端证书身份校验，数据面使用系统 CA 根证书包，不提供关闭证书校验的开关。

`healthCheck` 对象存在即启用 HTTP 主动健康检查，不再增加内部 `enabled` 字段。`path` 必填；检查间隔默认 10 秒，超时默认 2 秒。超时必须小于检查间隔。当前固定以 HTTP 成功响应判断端点健康，不暴露 Envoy 健康检查实现参数。

## 模型服务

```yaml
apiVersion: gateway.ingate.io/v1
kind: Upstream
metadata:
  name: 86a343b7-5044-4e61-8ae1-ff5b06a57d67
spec:
  displayName: 通义千问生产
  type: Model
  endpoints:
    - address: dashscope.aliyuncs.com
      port: 443
  tls:
    serverName: dashscope.aliyuncs.com
  model:
    provider: Qwen
    basePath: /compatible-mode/v1
    models:
      - qwen-max
      - qwen-plus
    apiKey: secret
```

模型 `provider` 当前支持 `OpenAI`、`DeepSeek`、`Qwen`、`Anthropic`、`Gemini` 和 `Custom`。Provider 直接决定请求协议和认证 Header 规则，不再让用户同时维护一份容易冲突的 `protocol`。

`model.models[]` 只保存允许 Route 引用的厂商模型名称。公开模型名称由 Route 的 `modelRouting` 定义，因此 Upstream 不再为模型目录维护重复的展示名称和启停字段。移除一个仍被 Route 引用的厂商模型时，Admin API 会拒绝更新。

API Key 直接保存在模型配置中。配置或保留 API Key 时必须同时配置 TLS。Admin API 不回显密钥，仅返回 `apiKeyConfigured`；更新时省略 `apiKey` 表示保留原值，提交非空值表示替换，提交空字符串表示清除。

## Admin API

Admin API 返回平铺的服务对象：

```json
{
  "id": "10c29251-b22f-48fe-acfb-891e11a23882",
  "name": "order-service",
  "type": "UPSTREAM_TYPE_APPLICATION",
  "endpoints": [
    {"address": "order.internal.example.com", "port": 8080, "weight": 1}
  ],
  "loadBalancing": "LOAD_BALANCING_POLICY_ROUND_ROBIN",
  "apiKeyConfigured": false,
  "state": "READY",
  "message": "配置已生效",
  "version": 3,
  "createdAt": "2026-08-10T10:00:00Z",
  "updatedAt": "2026-08-10T10:15:00Z"
}
```

查询、创建和更新接口直接返回 Upstream，删除成功返回空对象。列表使用不透明的 `limit/cursor` 游标分页。更新和删除必须提交映射 `metadata.generation` 的 `version`。

## MVP 边界

当前不支持服务发现注册中心、DNS SRV、Unix Socket、gRPC、TCP/UDP、上游 mTLS、自定义 CA、被动健康检查、熔断参数、连接池参数、会话保持和 Endpoint 运行状态写回。只有出现明确场景时再扩展，不把 Envoy Cluster 的所有参数直接暴露给用户。
