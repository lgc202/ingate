# Ingate v1 发布链路与 xDS

## 1. 定位

`Ingate` 的发布链路由 `ingate-xds-server` 承担。

它负责：

- watch `ResolvedGateway`
- 将 `ResolvedGateway` 翻译为发布运行态
- 管理当前 publish version / runtime cache
- 通过 gRPC 提供发现能力
- 将结果映射为 `Programmed`

它不负责：

- 资源 CRUD
- 策略合并
- 用户接口

## 2. 内部结构

`ingate-xds-server` 当前 v1 实现先固定为五层：

```text
watch
  -> translate
  -> cache
  -> discovery
  -> status
```

含义：

- `watch`：监听 `ResolvedGateway`
- `translate`：把 `ResolvedGateway` 翻译成内部运行态
- `cache`：管理当前发布版本与缓存
- `discovery`：向数据面提供发现接口
- `status`：聚合发布结果并回写 `Programmed`

## 3. `pkg/xds` 与 `internal/gateway/translation` 的边界

### 3.1 放在 `pkg/xds` 的

适合放在 `pkg/xds` 的是：

- xDS server 壳
- ADS 服务壳
- snapshot manager
- 通用版本管理
- ACK/NACK 记录框架

### 3.2 放在 `internal/gateway/translation` 的

适合放在 `internal/gateway/translation` 的是：

- `EffectiveConfig -> xDS` 的具体翻译逻辑
- listener/route/backend/policy 到 xDS 的投影
- Ingate 特有翻译规则

一句话边界：

- `pkg/xds` 负责“如何服务 xDS”
- `internal/gateway/translation` 负责“如何把 Ingate 语义翻成 xDS”

## 4. gRPC 接口面

当前 v1 已经改为：

```text
controller-manager -> ResolvedGateway
xds-server -> watch ResolvedGateway
```

也就是 `controller-manager` 负责持久化收敛结果，`xds-server` 只消费 `ResolvedGateway`，不再要求控制面主动调用 publish RPC。

当前保留两类只读/消费型 gRPC 能力：

### 4.1 `DiscoveryService.Resolve`

- 输入：`backend_name`、可选 `backend_type`
- 输出：后端 endpoint 列表
- 用途：给数据面或运维工具查询当前后端解析结果

### 4.2 `ConfigSyncService.GetConfig`

- 输入：`gateway_key`
- 输出：`source_version`、`publish_version`、`updated_at`、`EffectiveConfig`
- 用途：读取当前缓存中的发布快照，便于排障和验证

### 4.3 `ConfigSyncService.ListConfigs`

- 输入：空请求
- 输出：当前已发布 gateway 列表及其版本/更新时间
- 用途：列举本实例缓存内的发布视图，便于排障和值班检查

### 4.4 不再作为主链路的接口

- `PublishConfig` proto 可以保留做兼容演进，但不是当前 v1 主发布路径
- 当前主链路仍然是 `watch ResolvedGateway -> cache -> discovery/status`

## 5. v1 消费模型

当前 v1 里，稳定消费模型是：

**`ResolvedGateway` + xds-server 本地 runtime cache。**

它不是：

- 原始资源集合
- controller 主动推送给 xds-server 的 publish RPC
- Go 领域模型的机械镜像
- Envoy xDS protobuf 的镜像

必须明确区分三层：

### 5.1 资源模型

放在：

- `pkg/apis/gateway/v1alpha1`
- `pkg/apis/policy/v1alpha1`

### 5.2 IR 领域模型

放在：

- `internal/gateway/ir`

### 5.3 Proto 传输模型

源文件放在：

- `proto/ingate/configsync/v1`

生成结果放在：

- `pkg/generated/proto/...`

## 6. `EffectiveConfig` 的内容边界

传输模型只传：

- 已解析的 listener
- 已解析的 route
- 已解析的 backend
- 已生效的 policy 结果
- 版本与来源元信息

不直接传：

- 原始 `Gateway/Route/Backend/AuthPolicy/TrafficPolicy`
- 原始 `metadata/spec/status`
- `RouteConfiguration`
- `Cluster`
- `Any`
- filter `typed_config`

v1 先做：

- 完整快照

不做：

- patch/diff 协议
- 增量发布协议

## 7. ACK/NACK 处理

建议：

- `ads` 负责接收 ACK/NACK 事件
- `status` 负责把 ACK/NACK 映射成资源状态语义

不要让：

- `api` 直接处理 ACK/NACK
- `compiler` 直接处理 ACK/NACK

因为 ACK/NACK 属于运行时发布反馈，不是发布请求处理逻辑。

## 8. 一个典型发布流程

```text
ResolvedGateway update
  -> watch 接收变更
  -> translate 生成 runtime config
  -> cache 更新当前版本
  -> discovery / configsync 暴露最新结果
  -> status 回写 ResolvedGateway.Programmed
```

## 9. 推荐目录

```text
pkg/
  xds/
    cache/
    discovery/
    status/

internal/
  controlplane/
    xds/
      watch/
      translate/
      publish/
```

## 10. 一句话结论

`ingate-xds-server` 当前 v1 应该被设计成：

**一个只消费 `ResolvedGateway`、维护本地发布缓存、通过 discovery/configsync 暴露结果，并把状态回写为 `Programmed` 的发布组件。**
