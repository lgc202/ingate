# Ingate v1 控制面核心

## 1. 定位

`Ingate` 的控制面核心由两个内部组件协同完成：

- `ingate-controller-manager`
- `ingate-xds-server`

其中：

- `controller-manager` 负责 watch 资源、解析引用、合并策略、生成最终有效配置、回写状态
- `xds-server` 负责 watch 收敛结果、翻译为 xDS、推送给数据面并回写发布状态

一句话：

**把声明式资源持续收敛成可发布的有效网关配置，再由专门的发布组件对外分发。**

## 2. 进程与控制循环

### 2.1 进程边界

v1 保持两个独立进程：

- `ingate-controller-manager`
- `ingate-xds-server`

原因：

- `controller-manager` 负责“算”
- `xds-server` 负责“发”
- 便于后续横向扩展
- 便于故障隔离
- 符合企业级控制面常见做法

这不是“部署很多个 controller 进程”。

企业级做法通常是：

- 一个 `controller-manager` 进程
- 进程内部运行多个 controller

因此 v1 的部署形态仍然是：

- 1 个 `apiserver`
- 1 个 `controller-manager`
- N 个 `xds-server`
- N 个 `gateway`

### 2.2 controller 角色划分

v1 采用：

- 多资源 controller 负责触发
- 单一 `resolvedgateway` 收敛 controller 负责最终产物

触发型 controller：

- `gateway controller`
- `route controller`
- `backend controller`
- `certificate controller`
- `authpolicy controller`
- `trafficpolicy controller`

这些 controller 的统一职责是：

- watch 自己负责的资源
- 通过索引找出受影响的 `Gateway`
- 将 `gateway key` 投递到 `resolvedgateway` 队列

它们不负责：

- 直接装配最终运行配置
- 直接生成 `ResolvedGateway`
- 直接做最终策略裁决

最终收敛 controller：

- `resolvedgateway controller`

它负责：

- 读取一个 `Gateway` 相关的完整资源集合
- 解析引用关系
- 合并认证与流量保护
- 生成 `ResolvedGateway`
- 回写原资源状态
- 回写 `ResolvedGateway.status`

### 2.3 为什么不是一开始就每个资源各自独立收敛

`Ingate` 的最终发布结果不是某个单独资源，而是：

- 一个 `Gateway` 维度的最终有效配置

它同时依赖：

- `Gateway`
- `Route`
- `Backend`
- `Certificate`
- `AuthPolicy`
- `TrafficPolicy`

因此：

- 多资源 controller 适合做事件入口
- 单一收敛 controller 适合做最终发布结果

这样既保留了 Kubernetes 常见的多 controller 模式，又避免多个 controller 同时争抢写最终产物。

## 3. 收敛产物：ResolvedGateway

v1 引入一个内部资源：`ResolvedGateway`。

它的定位是：

- 不是用户声明资源
- 不是 xDS 原始协议对象
- 而是 controller 收敛后的最终有效网关配置

可以把它理解成控制面发布链路里的“中间表示”，但不直接命名成 `IR`。

原因：

- `IR` 太技术、太宽泛
- `ResolvedGateway` 更贴近业务语义
- 便于阅读与排障

### 3.1 命名原则

- 资源名：`ResolvedGateway`
- controller 层命名保持朴素
- 复杂语义主要放在资源名和文档里，而不是硬塞进 controller 类型名里

### 3.2 为什么不叫 GatewaySnapshot

不采用 `GatewaySnapshot` 的原因是：

- “snapshot” 容易让人联想到副本、备份、导出快照
- 不能很好表达“引用已解析、策略已合并、可直接发布”的语义
- `ResolvedGateway` 更准确表达：这是已经解析完成的 Gateway 运行态结果

## 4. ResolvedGateway 资源模型

### 4.1 基本原则

`ResolvedGateway` 的设计原则是：

- 比用户声明资源更具体
- 比 xDS 原文更抽象
- 只为发布链路和排障服务
- 只读，不允许用户直接编辑

### 4.2 metadata

建议：

- `metadata.name = gateway.name`
- 一网关一个 `ResolvedGateway`
- 与 `Gateway` 同 namespace

这样有两个好处：

- 排障直接
- 便于 `kubectl get/list/watch`

### 4.3 spec

`ResolvedGateway.spec` 放 controller 计算后的最终有效配置。

v1 建议包含这些部分：

- `gatewayRef`
- `version`
- `listeners`
- `routes`
- `backends`
- `extensions`

其中：

`listeners` 包含：

- 端口
- 协议
- 主机名集合
- TLS 引用与解析结果

`routes` 包含：

- host/path/method/header 匹配
- rewrite 结果
- header 处理结果
- 认证结果
- 流量保护结果
- backend 引用结果

`backends` 包含：

- 已解析 endpoint 集
- 协议
- 负载均衡方式

`extensions`：

- 预留扩展位
- v1 先不实现插件执行逻辑
- 只保证模型可演进

关键原则：

- `ResolvedGateway.spec` 中应该直接体现最终生效结果
- `xds-server` 不应再次回头查原始 `AuthPolicy` / `TrafficPolicy` 做二次合并

### 4.4 status

`ResolvedGateway.status` 面向发布链路，而不是面向用户声明语义。

建议至少包含：

- `observedGeneration`
- `phase`
- `conditions`
- `lastReconciledTime`
- `publishedVersion`

v1 统一 condition 类型：

- `Accepted`
- `Resolved`
- `Programmed`

含义：

- `Accepted`：controller 是否接受该 gateway 进入收敛
- `Resolved`：引用和策略是否已解析完成
- `Programmed`：xds-server 是否成功消费并发布

## 5. 状态分工

### 5.1 原资源 status

原始资源 `status` 继续保留，并且必须存在。

它用于表达：

- 用户配置是否有效
- 当前引用是否可解析
- 当前资源是否可被控制面接受

这些状态直接面向：

- console
- admin-api
- kubectl
- 事件系统

### 5.2 ResolvedGateway status

`ResolvedGateway.status` 用于表达发布链路状态，例如：

- 这份有效配置是否构建成功
- 最近一次收敛是否成功
- xds-server 是否已消费并发布
- 最近一次发布是否收到错误

### 5.3 统一原则

- 原资源 `status`：面向资源健康与配置有效性
- `ResolvedGateway.status`：面向发布结果与分发链路

不要把所有状态都塞进某一边。

## 6. 控制循环与收敛流程

一个完整的 v1 控制链路如下：

```text
resource changed
  -> resource controller finds affected gateways
  -> enqueue gateway key
  -> resolvedgateway controller reconcile
  -> load related objects
  -> validate refs
  -> merge policies
  -> build ResolvedGateway
  -> persist ResolvedGateway
  -> update original resource status
  -> update ResolvedGateway.status
```

### 6.1 触发流程

任一资源变化：

- `Gateway`
- `Route`
- `Backend`
- `Certificate`
- `AuthPolicy`
- `TrafficPolicy`

对应资源 controller 收到事件后：

- 用索引找出受影响的 `Gateway`
- 将 `gateway key` 放入 `resolvedgateway` 队列
- 做好去重，避免同一批事件引起大量重复调谐

### 6.2 resolvedgateway reconcile

`resolvedgateway controller` 对单个 `gateway key` 的固定流程：

1. 读取 `Gateway`
2. 读取关联 `Route`
3. 读取 `Route` 关联 `Backend`
4. 读取监听器关联 `Certificate`
5. 读取作用于这些对象的 `AuthPolicy` / `TrafficPolicy`
6. 校验引用关系
7. 做策略绑定与冲突判断
8. 生成 `ResolvedGateway.spec`
9. Upsert `ResolvedGateway`
10. 回写原始资源 `status`
11. 回写 `ResolvedGateway.status`

### 6.3 错误处理原则

如果收敛中途失败：

- 仍然尽可能回写错误状态
- 不允许只打日志、不写对象状态
- 不做隐式魔法修正

原则：

**宁可显式失败，也不做不可预测的自动兜底。**

## 7. xds-server 职责

`xds-server` 不再直接从原始资源重新推导配置，而是：

1. watch `ResolvedGateway`
2. 维护本地 cache
3. 将 `ResolvedGateway.spec` 翻译为 xDS 结构
4. 推送给连接的 gateway
5. 根据 ACK/NACK 或发布结果更新 `ResolvedGateway.status.Programmed`

职责划分必须清楚：

- `controller-manager` 负责语义收敛
- `xds-server` 负责协议翻译与分发

## 8. namespace 策略

v1 建议：

- 默认支持全 namespace watch
- 通过配置可以收窄 watch scope

但语义上先限制为：

- `ResolvedGateway` 与 `Gateway` 同 namespace
- `Route/Certificate/AuthPolicy/TrafficPolicy` 默认只允许同 namespace 绑定
- 跨 namespace 引用在 v1 暂不支持，或显式拒绝

这样既保留企业级控制面的扩展能力，也控制实现复杂度。

## 9. 工程组织方式

这一部分参考 `onex`，但不照搬其业务模型。

### 9.1 参考 onex 的地方

- `cmd/<component>/main.go` 保持很薄
- 启动编排放在 `cmd/controller-manager/app/`
- flags/options/config 拆清楚
- controller 名字和注册集中管理
- leader election / healthz / readyz / metrics 统一放在 manager 侧处理

### 9.2 不直接照搬 onex 的地方

`onex` 很多场景是一资源一 Reconciler，且收敛目标主要还是资源本身。

`Ingate` 不同：

- 最终收敛目标是 `ResolvedGateway`
- 它依赖多种资源共同计算
- 因此不能把最终收敛责任分散到各个资源 controller 中

### 9.3 推荐目录

```text
cmd/controller-manager/
  main.go
  app/
  options/
  names/

internal/controlplane/controller/
  gateway/
  route/
  backend/
  certificate/
  authpolicy/
  trafficpolicy/
  resolvedgateway/
  shared/
```

其中：

- `gateway/route/backend/certificate/authpolicy/trafficpolicy/`
  - 负责 watch 和 enqueue
- `resolvedgateway/`
  - 负责最终收敛和持久化 `ResolvedGateway`
- `shared/`
  - 放 queue key、enqueue helper、通用 controller option 等共享能力

## 10. 禁止的实现方式

### 10.1 不要让每个资源 controller 各自拼最终发布结果

否则会导致：

- 最终收敛责任分散
- 状态结论不一致
- 策略冲突处理漂移

### 10.2 不要让 xds-server 再做一轮控制面策略决策

否则会导致：

- controller 和 xds-server 职责重叠
- 同一规则在两边各算一遍
- 行为不可预测

### 10.3 不要在原始资源 status 中塞发布协议细节

否则会导致：

- 原始资源状态污染
- 用户难以理解真正的资源健康问题
- 发布链路状态与资源健康状态混杂

### 10.4 不要在 v1 过早引入插件执行逻辑

v1 只预留扩展位，不应在第一阶段把插件装配、插件下发、插件运行时全部拉进控制链路。

## 11. 一句话结论

`Ingate v1` 控制面采用：

- 双进程：`controller-manager + xds-server`
- 多资源 controller 负责触发
- 单一 `resolvedgateway` controller 负责最终收敛
- `ResolvedGateway` 作为发布链路中的内部只读有效配置资源
- `xds-server` 只消费 `ResolvedGateway`，不再重做控制面语义计算
