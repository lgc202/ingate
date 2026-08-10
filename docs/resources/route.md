# Route 资源

Route 表示一组请求匹配条件和一个转发行为。一个 Route 可以挂载到多个 Gateway，但不会在资源内部再嵌套多条规则；需要不同匹配或转发行为时创建多个 Route。

这种粒度让 Route 自身成为稳定的配置、状态和策略作用目标，避免再引入只存在于 Route 内部的规则名称和版本边界。

## 声明式资源

普通 HTTP Route：

```yaml
apiVersion: gateway.ingate.io/v1
kind: Route
metadata:
  name: 992090fd-70ad-4806-bb45-5ee770530609
spec:
  displayName: order-api
  enabled: true
  gatewayRefs:
    - 5cb83268-6e5c-42af-a4d0-3f40fd449b66
  hostnames:
    - api.example.com
  match:
    path:
      type: Prefix
      value: /orders
    methods:
      - GET
      - POST
    headers:
      - name: x-tenant
        value: public
  upstreamRefs:
    - name: 10c29251-b22f-48fe-acfb-891e11a23882
      weight: 3
    - name: e2ca967e-af6c-4f5c-8f8c-33f5424f6f61
      weight: 1
  requestHeaderModifier:
    set:
      - name: x-ingate-source
        value: public-gateway
  timeout:
    requestMillis: 30000
  retry:
    attempts: 2
    perTryTimeoutMillis: 5000
```

模型 Route：

```yaml
apiVersion: gateway.ingate.io/v1
kind: Route
metadata:
  name: a8fab5fc-1969-4cb3-ad5e-c39fde1c615c
spec:
  displayName: public-models
  enabled: true
  gatewayRefs:
    - 5cb83268-6e5c-42af-a4d0-3f40fd449b66
  match:
    path:
      type: Exact
      value: /v1/chat/completions
    methods:
      - POST
  modelRouting:
    models:
      - model: chat-default
        upstreamRef: 86a343b7-5044-4e61-8ae1-ff5b06a57d67
        upstreamModel: qwen-max
      - model: claude-sonnet
        upstreamRef: fa713111-5695-48d3-8625-e745de9e0b76
        upstreamModel: claude-sonnet-4
```

`metadata.name` 是不可变资源 ID，Admin API 创建 Route 时生成 UUID。`spec.displayName` 是同类资源内唯一的用户展示名称，资源引用均使用 ID。

## 匹配语义

- `gatewayRefs` 至少包含一个 Gateway ID；同一个 Route 可以复用到多个 Gateway
- `hostnames` 为空表示不限制 Host；多个 Host 之间是 OR 关系，支持精确域名和 `*.example.com` 通配域名
- `match.path.type` 当前支持 `Prefix` 和 `Exact`
- `match.methods` 为空表示匹配所有受支持的 HTTP 方法；多个 Method 之间是 OR 关系
- `match.headers` 只支持精确匹配，多个 Header 必须同时满足
- 一个 Route 只有一组匹配条件，不在资源内嵌套 `rules[]`

多个 Route 同时可能命中时，数据面按照路径具体程度、精确路径、Header 数量和 Method 限定程度生成确定性顺序。完全相同且挂载范围重叠的有效 Route 属于冲突配置，由 Controller 在 status 中给出最终裁决。

## 转发语义

普通 Route 使用 `upstreamRefs[]`，模型 Route 使用 `modelRouting`，两者必须且只能配置一个。

普通 Route 的 `weight` 是 `1-1000` 的相对权重，不是百分比。例如 `3:1` 和 `75:25` 表达相同的流量比例。目标必须是使用 HTTP 协议的非模型 Upstream。

模型 Route 固定精确匹配 `POST /v1/chat/completions`。每个公开模型名称独立引用一个模型 Upstream 和可选厂商模型名称；`upstreamModel` 为空时沿用公开模型名称。模型 Route 当前不支持 Route 重试，协议转换、访问认证和实际模型选路由 `ingate-ai-proxy` 执行。

## Header、超时和重试

`requestHeaderModifier` 和 `responseHeaderModifier` 都支持：

- `set`：覆盖 Header
- `add`：追加 Header 值
- `remove`：删除 Header

同一个 Header 在同一修改器中只能配置一种动作。模型 Route 不能修改认证、请求体 framing 和 Ingate 内部选路使用的请求 Header。

请求总超时范围为 `100-300000` 毫秒。重试次数范围为 `1-5`，单次重试超时范围为 `100-60000` 毫秒，且不能大于请求总超时。未显式配置总超时时，校验使用系统默认值 30000 毫秒。

## Admin API

Admin API 返回平铺的 Route 产品对象，不暴露声明式资源的 `metadata/spec/status`：

```json
{
  "id": "992090fd-70ad-4806-bb45-5ee770530609",
  "name": "order-api",
  "enabled": true,
  "gatewayIDs": ["5cb83268-6e5c-42af-a4d0-3f40fd449b66"],
  "hostnames": ["api.example.com"],
  "match": {
    "path": {"type": "ROUTE_PATH_MATCH_PREFIX", "value": "/orders"},
    "methods": ["HTTP_METHOD_GET", "HTTP_METHOD_POST"],
    "headers": []
  },
  "upstreams": [
    {"upstreamID": "10c29251-b22f-48fe-acfb-891e11a23882", "weight": 1}
  ],
  "state": "READY",
  "message": "配置已生效",
  "version": 3,
  "createdAt": "2026-08-10T10:00:00Z",
  "updatedAt": "2026-08-10T10:15:00Z"
}
```

查询、创建和更新接口直接返回 Route。删除成功返回空对象。列表与 Gateway 一样使用不透明的 `limit/cursor` 游标分页。

更新和删除必须提交读取到的 `version`。`version` 映射 `metadata.generation`；只有 Route spec 真实变化时才推进版本和 `updatedAt`，Controller 更新 status 不改变二者。

## MVP 边界

Route 当前不支持查询参数匹配、正则路径、正则 Header、重定向、URL Rewrite、请求镜像、故障注入、CORS、会话保持和模型 fallback。治理能力通过独立强类型 Policy 引用 Route，不作为 Route 内部过滤器暴露。
