# Route

`Route` 把一个或多个 Gateway 的请求转发到一个或多个 Upstream。

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
      - name: x-api-version
        value: v1
  upstreamRefs:
    - name: 4b911c58-d614-4316-b10a-fb12cb9f138c
      weight: 100
  timeout:
    requestMillis: 30000
  retry:
    attempts: 2
    perTryTimeoutMillis: 5000
```

匹配规则：

- `gatewayRefs[]` 至少包含一个 Gateway ID
- `hostnames[]` 为空时不额外限制 Host，多个域名之间是 OR 关系
- 路径支持 `Prefix` 和 `Exact`
- `methods[]` 为空时匹配所有 HTTP 方法，多个方法之间是 OR 关系
- `headers[]` 使用精确匹配并且必须全部满足

`upstreamRefs[]` 可以配置多个 Upstream，`weight` 表示相对流量权重。Route 还可以使用 `requestHeaderModifier` 和 `responseHeaderModifier` 对 Header 执行 `set`、`add` 和 `remove`。
