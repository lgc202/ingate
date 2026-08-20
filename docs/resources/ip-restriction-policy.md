# IPRestrictionPolicy 资源

IPRestrictionPolicy 为 Gateway 或 Route 配置客户端 IP 访问限制。一条策略使用一个允许列表或拒绝列表，并通过多个 `targetRefs` 复用到不同目标。

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

`metadata.name` 是不可变资源 ID。`spec.displayName` 是同类资源内唯一的展示名称。

`targetRefs` 支持 Gateway 和 Route，可以为空。空列表表示策略已经保存但尚未应用；策略不会自动匹配未来创建的资源。Admin API 创建和更新时要求目标存在，直接使用声明式 API 写入的缺失目标由 `status.targets` 表达。

## 允许列表和拒绝列表

`allow` 和 `deny` 必须且只能配置一个：

- `allow`：只允许列表中的客户端 IP，其他请求返回 HTTP `403`
- `deny`：拒绝列表中的客户端 IP，其他请求正常通过

列表项支持 IPv4、IPv6 和 CIDR。单个 IP 会被规范化为 `/32` 或 `/128` 精确网段，CIDR 会被规范化为网络前缀；重复项会被删除并稳定排序。

多个 IPRestrictionPolicy 命中同一个请求时必须全部允许。一个策略同时通过 Gateway 和 Route 引用命中同一请求时只执行一次。没有挂载 IPRestrictionPolicy 的请求不执行 IP 判断。

## 客户端 IP 语义

数据面使用网关连接看到的来源 IP，不信任客户端可直接伪造的 `X-Forwarded-For` 等请求 Header。网关前存在负载均衡或反向代理时，部署方必须在可信网络边界保留真实源地址，否则策略看到的可能是代理地址。

插件配置解析失败或来源地址无法解析时拒绝请求，避免安全策略失效后静默放行。拒绝响应固定为 HTTP `403`，不在资源中提供自定义响应内容。

## Admin API

Admin API 返回平铺的 IP 访问限制策略对象：

```json
{
  "id": "0fba1dca-86cc-426e-99b7-d0047e092414",
  "name": "内部接口允许列表",
  "enabled": true,
  "targets": [
    {
      "kind": "POLICY_TARGET_KIND_ROUTE",
      "id": "93c0ca26-ff54-4b18-9da7-73ea51347395",
      "name": "内部管理接口",
      "state": "READY",
      "message": "配置已生效"
    }
  ],
  "allow": ["10.0.0.0/8", "192.168.1.20/32"],
  "deny": [],
  "state": "READY",
  "message": "配置已生效",
  "version": 2,
  "createdAt": "2026-08-10T10:00:00Z",
  "updatedAt": "2026-08-10T10:15:00Z"
}
```

查询、创建和更新接口直接返回 IPRestrictionPolicy，删除成功返回空对象。列表使用不透明的 `limit/cursor` 游标分页。创建接口不接收 `enabled`，新策略固定默认启用；更新和删除必须提交映射 `metadata.generation` 的 `version`，更新时通过 `enabled` 调整启停状态。

不提供独立启停接口，启停和其他字段一样通过完整资源更新完成。停用配置进入当前生效版本后状态为 `DISABLED`；没有目标的启用策略状态为 `READY`，消息为“策略已保存，尚未应用”。

## MVP 边界

当前不支持国家或地区、JWT 身份、用户角色、HTTP Header、Query 参数、时间段和标签选择器等访问控制条件。这些能力有不同的数据来源和执行语义，不放进 IPRestrictionPolicy。
