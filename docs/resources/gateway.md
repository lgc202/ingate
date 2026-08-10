# Gateway 资源

Gateway 表示一组对外流量入口。一套 Ingate 可以创建多个 Gateway，这些 Gateway 共同编译到同一组配置完全相同的 Envoy 实例。

Gateway 只声明监听协议、端口、Host 范围和 HTTPS 证书。路由规则、转发目标和治理策略由其他资源负责。

## 声明式资源

```yaml
apiVersion: gateway.ingate.io/v1
kind: Gateway
metadata:
  name: 5cb83268-6e5c-42af-a4d0-3f40fd449b66
spec:
  displayName: public-gateway
  enabled: true
  listeners:
    - name: http
      protocol: HTTP
      port: 8080
      hostname: "*.example.com"
    - name: https
      protocol: HTTPS
      port: 8443
      hostname: "*.example.com"
      certificateRef: 51d0a788-8104-49fa-97d5-1e2f29f592f9
status:
  conditions:
    - type: Accepted
      status: "True"
      observedGeneration: 3
    - type: ResolvedRefs
      status: "True"
      observedGeneration: 3
    - type: Programmed
      status: "True"
      observedGeneration: 3
```

`metadata.name` 是不可变资源 ID。Admin API 创建 Gateway 时生成 UUID，资源引用始终使用该 ID。

`spec.displayName` 是用户可编辑的展示名称，同类资源内唯一。`spec.enabled` 表示当前 Gateway 是否应当出现在数据面配置中。

每个 Listener 包含：

- `name`：Gateway 内唯一的稳定标识，使用 DNS label 格式
- `protocol`：当前只支持 `HTTP` 和 `HTTPS`
- `port`：Envoy 实际监听端口，范围为 `1-65535`
- `hostname`：可选；空值表示匹配所有 Host
- `certificateRef`：Certificate 资源 ID；HTTPS 必填，HTTP 禁止填写

同协议、同端口的多个 Listener 可以通过互不重叠的 Host 范围共享一个 Envoy Listener。这允许不同 HTTPS Listener 在同一个端口使用不同证书，不需要在 MVP 中引入单 Listener 多证书模型。

## Admin API

Admin API 面向控制台提供平铺的产品对象，不暴露声明式资源的 `metadata/spec/status` 结构：

```json
{
  "id": "5cb83268-6e5c-42af-a4d0-3f40fd449b66",
  "name": "public-gateway",
  "enabled": true,
  "listeners": [
    {
      "name": "https",
      "protocol": "GATEWAY_PROTOCOL_HTTPS",
      "port": 8443,
      "hostname": "*.example.com",
      "certificateID": "51d0a788-8104-49fa-97d5-1e2f29f592f9"
    }
  ],
  "state": "READY",
  "message": "配置已生效",
  "version": 3,
  "createdAt": "2026-08-10T10:00:00Z",
  "updatedAt": "2026-08-10T10:15:00Z"
}
```

`state` 是 Admin API 根据当前 `spec.enabled`、`metadata.generation` 和 `status.conditions` 计算的控制台视图，不单独持久化：

- `DISABLED`：停用配置已经进入数据面生效版本
- `PENDING`：当前配置版本尚未完成校验或下发
- `READY`：当前配置版本已经由 Envoy 接受
- `ERROR`：当前配置版本无法生效

`message` 是与当前状态对应的最小用户提示，不向前端暴露 Envoy、xDS、ACK、NACK 或 Condition Reason。

查询、创建和更新接口直接在统一响应的 `data` 中返回 Gateway，不增加只有一个 `gateway` 字段的包装层。删除成功后 `data` 返回空对象；调用方已经持有被删除的资源 ID，不再返回重复的 `success` 和 `id`。

### 列表分页

Gateway 列表使用不透明游标分页：

```http
GET /api/v1/gateways?limit=100&cursor=<opaque>
```

```json
{
  "gateways": [],
  "nextCursor": ""
}
```

- `limit` 为 `0` 时使用默认值 `100`，大于 `200` 时按 `200` 处理
- `cursor` 为空表示第一页，调用方不能解析或拼接游标内容
- `nextCursor` 为空表示已经到达列表末尾
- MVP 不提供页码、总数和 `hasMore`；列表顺序和数据变化时，这些值会产生虚假的精确性

## 版本和时间

Admin API 使用 `version` 进行多人编辑的乐观并发控制。客户端更新或删除 Gateway 时必须提交读取到的版本，版本不匹配时返回 `409 Conflict`。

版本和时间由系统维护：

- `version` 映射 `metadata.generation`
- `createdAt` 映射 `metadata.creationTimestamp`
- `updatedAt` 映射系统保留的 `gateway.ingate.io/updated-at` annotation
- 只有 `spec` 发生真实变化时才推进 `generation` 和 `updatedAt`
- Controller 更新 `status` 不改变版本和更新时间
- data/apiserver 层使用 `resourceVersion` 完成原子更新，不向 Admin API 暴露

## 校验规则

- 展示名称必填且同类资源内唯一
- 至少配置一个 Listener
- Listener 名称必填、符合 DNS label 格式且在 Gateway 内唯一
- HTTP Listener 不能引用证书
- HTTPS Listener 必须引用存在的 Certificate
- 停用 Gateway 仍需通过资源结构和证书引用校验，只是不参与入口冲突检查
- 同一端口不能同时声明 HTTP 和 HTTPS
- 同协议、同端口的 Listener 之间 Host 范围不能重叠
- 空 Host 与同端口的任意其他 Host 都冲突
- `*.example.com` 与 `api.example.com` 冲突
- Host 范围不重叠时，不同 Gateway 可以共享协议和端口
- Admin API 提供即时引用与冲突校验，Controller status 是声明式 API 并发写入后的最终裁决

## MVP 边界

Gateway 当前不包含描述、独立 HostBinding、自定义监听 IP、HTTP/2 开关、TLS 版本、加密套件、mTLS 和单 Listener 多证书配置，也不向用户暴露 Envoy FilterChain 等实现细节。
