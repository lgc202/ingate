# Ingate v1 可用性、恢复与测试

## 1. 目标

这份文档统一回答三个问题：

- 系统重启后如何恢复
- 各组件如何做高可用与横向扩展
- 测试如何分层覆盖关键风险

## 2. 恢复策略

`Ingate` 的恢复原则是：

**以 etcd 为资源真相源，以重新收敛为恢复机制。**

### 2.1 `apiserver` 重启

- 从 `etcd` 恢复资源服务
- 资源和状态都必须持久化
- watch 能重新建立

### 2.2 `controller-manager` 重启

- 重新 `list/watch`
- 重建本地 cache
- 重新 reconcile
- 重新生成 `EffectiveConfig`
- 重新发布给 `xds-server`

### 2.3 `xds-server` 重启

v1 采用：

- snapshot 可丢失
- 由 `controller-manager` 重新发布当前版本恢复

### 2.4 Envoy 重启

- 重新连接 ADS/xDS
- 重新拉取当前有效配置

一句话：

**控制面中间状态可以丢，但必须可重建。**

## 3. 高可用与横向扩展

### 3.1 `ingate-apiserver`

- 无状态多副本
- 前面挂 LB
- 后接同一个 etcd 集群

### 3.2 `ingate-controller-manager`

- 多副本部署
- `leader election`
- 同一时刻只有 leader 执行主控制循环

这是 v1 最稳的企业级方案。

### 3.3 `ingate-xds-server`

- 多副本多活
- 基于 `config_version` 幂等处理发布请求
- Envoy 可通过 LB 连任意实例

### 3.4 `Envoy`

- 数据面天然多副本
- 每个实例通过 ADS 拉相同配置

### 3.5 `admin-api`

- 无状态多副本

### 3.6 `etcd`

- 独立 HA 集群
- 这不是普通业务服务扩容，而是底座高可用

## 4. 测试分层

`Ingate` v1 的测试应分成五层：

1. 领域单元测试
2. 控制面单元/模块测试
3. 契约测试
4. 组件集成测试
5. 端到端测试

## 5. 第一层：领域单元测试

覆盖对象：

- `Gateway`
- `Route`
- `Backend`
- `AuthPolicy`
- `TrafficPolicy`
- policy merge 规则
- IR 构建逻辑

建议位置：

- `internal/gateway/model/...`
- `internal/gateway/policy/...`
- `internal/gateway/ir/...`

## 6. 第二层：控制面单元/模块测试

覆盖对象：

- controller 调度逻辑
- reconcile 主流程
- status 回写逻辑
- `EffectiveConfig` 发布触发逻辑

建议位置：

- `internal/controlplane/controller/...`

## 7. 第三层：契约测试

覆盖对象：

- `controller-manager -> xds-server` gRPC 契约
- `service-discovery -> controller-manager` gRPC 契约
- proto 兼容性

建议位置：

- `proto/...`
- `pkg/xds/...`
- `pkg/discovery/...`

## 8. 第四层：组件集成测试

覆盖对象：

- `ingate-apiserver + etcd`
- `ingate-controller-manager + fake/real apiserver`
- `ingate-xds-server + Envoy xDS client`

重点验证：

- `list/watch` 是否正常
- `PublishConfig` 是否幂等
- xDS snapshot 是否正确构建
- ACK/NACK 是否能回写状态

## 9. 第五层：端到端测试

覆盖对象：

- apiserver
- controller-manager
- xds-server
- Envoy
- backend service

黄金路径应至少覆盖：

- HTTP 基本转发
- HTTPS 基本转发
- JWT 生效
- APIKey 生效
- timeout/retry/rate limit 生效

## 10. 一句话结论

`Ingate` 的可用性设计应遵循：

**以 etcd 为真相源、以重新收敛为恢复手段、以 controller leader election 和其余组件多副本为 HA 基线，并用五层测试覆盖控制面到数据面的关键风险。**
