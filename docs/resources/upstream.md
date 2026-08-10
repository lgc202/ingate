# Upstream

`Upstream` 表示一个可以接收 HTTP 请求的逻辑服务。Route 只引用 Upstream ID，不直接保存网络地址。

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

`address` 可以是 IP 或 DNS 名称。多个端点按相对 `weight` 分配流量；未填写权重时按 `1` 处理。负载均衡支持 `RoundRobin` 和 `LeastRequest`。

访问 HTTPS 上游时配置：

```yaml
spec:
  tls:
    serverName: api.example.com
```

Envoy 使用系统 CA 根证书校验上游证书，并将 `serverName` 同时用于 SNI 和证书身份校验。未配置 `tls` 时使用明文 HTTP。
