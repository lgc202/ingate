# Ingate Next 交接文档

本文档用于给后续 AI 对话快速恢复上下文。当前仓库路径：

```text
/Users/lgc202/workspace/source/lgc202/ingate
```

## 项目目标

Ingate Next 是一次全新重写，不受旧项目 `../ingate` 的命名和实现影响。

核心方向：

- 做一个面向 API Gateway、AI Gateway、多运行时 target 的声明式控制面
- 内部模型优先表达网关领域语义，不绑定某个具体数据面
- Envoy xDS 是第一个 target，但不是核心模型本身
- 控制面采用 Kubernetes-like 思路：apiserver、声明式资源、watch、controller、reconcile
- 既要支持 Kubernetes 部署，也要支持 VM / 裸机部署

重要命名约定：

- 使用 `Upstream`，不要使用 `Backend`
- 使用 `Gateway / Route / Upstream` 作为第一批核心资源
- `RuntimeSnapshot` 表示 controller 编译后给运行时 target 消费的配置快照

## 最新模型决策

Route、Policy 和 Plugin 的长期后端边界已经整理到：

```text
docs/superpowers/specs/2026-06-05-route-policy-plugin-model-design.md
```

后续不要继续扩展 `route.ingate.io/policy-bindings` annotation 方案。它只是早期验证控制台路由策略闭环的 MVP 路径。

长期方向：

- Route 原生能力：Header 改写、超时、重试、URL rewrite、mirror、CORS 等直接进入 `RouteSpec` 的强类型字段或 route filters
- Policy 治理策略：认证、限流、访问控制等可复用、可审计、可绑定能力使用独立 Policy 资源和 `PolicyBinding`
- Plugin 扩展能力：AI proxy、内容安全、Token 统计、OPA、自定义响应等运行时扩展使用 `Plugin` 和 `PluginBinding`

控制台可以继续使用“策略配置”作为产品语言，但 admin-api 保存时必须拆成正式声明式资源。compiler 不能依赖控制台中文展示名或任意表单 JSON。

## 当前代码状态

当前主链路已经做到：

```text
pkg/apis/gateway/v1 API Types
        |
        | code-generator
        v
generated clientset / informer / lister
        |
        v
ingate-apiserver REST storage
        |
        v
ingate-controller watch / workqueue / reconcile
        |
        v
compiler / pipeline / xDS target translator
        |
        v
RuntimeSnapshot API Resource
        |
        v
ingate-xds watch / snapshotStore / Envoy ADS
```

最近关键提交：

```text
136e459 feat: add gateway overview api
23868f5 feat: add admin api read server
fc2e0a3 chore: rename module to ingate
e73af3c chore: remove ingate cli
dea1fd9 refactor: split ads stream state
dd8ce61 feat: push ads updates on snapshot changes
eb2d3e7 feat: build xds discovery resources
```

当前工作区准备更新本文档并提交。

## 已完成能力

### 核心编译链路

早期已经完成本地编译链路：

```text
Resource Bundle -> Compiler -> Logical IR -> Target Translator -> RuntimeSnapshot
```

相关目录：

- `internal/core/compiler`
- `internal/core/ir`
- `internal/core/pipeline`
- `internal/core/runtime`
- `internal/core/target`
- `internal/core/target/xds`
- `internal/core/target/debug`

当前内置 target：

- `debug`
- `xds`

### 服务边界

已经有第一批长期服务入口：

- `cmd/ingate-admin-api`
- `cmd/ingate-apiserver`
- `cmd/ingate-controller`
- `cmd/ingate-xds`

其中：

- `ingate-admin-api`：前端管理 API，当前使用 Gin 并按 `app/server/handler/service/store/pkg` 分层
- `ingate-apiserver`：声明式资源 API
- `ingate-controller`：watch 资源并做状态收敛
- `ingate-xds`：给 Envoy 提供 xDS ADS 服务

早期本地调试 CLI `cmd/ingate` 已删除，避免维护非主线入口。

### API 类型

API 包在：

```text
pkg/apis/gateway/v1
```

当前核心资源：

- `Gateway`
- `Route`
- `Upstream`

已有但还没接入 apiserver storage 的资源：

- `AIRoute`
- `AIProvider`
- `Plugin`
- `AuthPolicy`
- `RateLimitPolicy`
- `PolicyBinding`
- `PluginBinding`

当前 `Gateway / Route / Upstream` 使用：

```go
// +genclient
// +genclient:nonNamespaced
```

也就是当前作为非命名空间资源处理。这样做是为了避免内部控制面过早绑定 Kubernetes Namespace 语义。后续多租户隔离更可能设计为 `Tenant / Workspace / Project / Environment`，而不是直接把 Kubernetes Namespace 当成产品模型。

### code-generator

代码生成脚本：

```text
hack/update-codegen.sh
```

当前生成：

- deepcopy
- clientset
- informer
- lister

生成产物：

```text
pkg/generated
```

常用命令：

```bash
make generate
```

### apiserver

apiserver 相关代码：

```text
internal/apiserver/app
internal/apiserver/server
internal/apiserver/registry/gateway
internal/apiserver/registry/route
internal/apiserver/registry/upstream
```

已接入 `genericregistry.Store` 的真实 REST storage：

- `gateways`
- `gateways/status`
- `routes`
- `routes/status`
- `upstreams`
- `upstreams/status`

注意：

- 没有直接操作 etcd client
- 通过 Kubernetes generic apiserver 的 `RESTOptionsGetter` 走真实存储路径
- 现在没有 fake in-memory store

### controller

controller 相关代码：

```text
internal/controller/app/app.go
internal/controller/controller/controller.go
```

启动参数：

- `--master`
- `--kubeconfig`
- `--target`
- `--resync-period`

controller 当前做了：

- 使用生成的 clientset 连接 ingate-apiserver
- 使用 generated informer 监听 `Gateway / Route / Upstream`
- 使用 `workqueue.TypedRateLimitingInterface[string]` 按 Gateway name 入队
- 使用 Route informer indexer 建索引：
  - `parentRef`：按 Gateway 找 Route
  - `upstreamRef`：按 Upstream 找 Route
- `Gateway` 事件：入队自身
- `Route` 事件：根据 `spec.parentRefs` 入队相关 Gateway
- `Upstream` 事件：根据 `upstreamRef` 索引找到相关 Route，再入队相关 Gateway
- `reconcileGateway(name)` 当前会构造当前 Gateway 相关 Bundle 并走 pipeline 编译 snapshot
- reconcile 成功后 create/update 对应 `RuntimeSnapshot`
- Gateway 删除后删除对应 `RuntimeSnapshot`

现在不是全量 reconcile 所有 Gateway，而是按 Gateway key reconcile。

当前仍会读取全量 Gateway 列表，原因是现有 compiler 会校验 Route 的全部 `ParentRefs` 是否存在。后续可以继续把 compiler 改成真正的单 Gateway 编译，去掉这个全量 Gateway 读取。

### RuntimeSnapshot

`RuntimeSnapshot` 已作为 controller 输出资源接入主链路：

- API 类型在 `pkg/apis/gateway/v1`
- apiserver storage 在 `internal/apiserver/registry/runtimesnapshot`
- generated client / informer / lister 已更新
- 当前作为非命名空间资源处理
- `spec.target` 用于区分运行时 target
- `spec.gateway` 表示该 snapshot 属于哪个 Gateway
- `spec.version` 当前由 xDS target translator 生成
- `spec.config` 使用 `runtime.RawExtension` 保存 target-specific 配置

### xDS 服务

`ingate-xds` 当前已经从“观察 snapshot”推进到“可响应 Envoy ADS”：

- `internal/xds/app` 只保留 CLI/app wiring
- `internal/xds/server` 承载 RuntimeSnapshot watch、snapshotStore、gRPC server、ADS 逻辑
- `snapshotStore` 只缓存当前 target 的 `RuntimeSnapshot`，按 Gateway 覆盖，不按版本追加
- ADS 支持 State-of-the-World `StreamAggregatedResources`
- Delta ADS 仍明确返回 `Unimplemented`
- 已支持构建 LDS/RDS/CDS/EDS 响应
- ADS request 会记录 type、version、nonce、resource count、snapshot count
- ADS ACK/NACK 会记录确认或拒绝原因
- 同一 stream 内对已 ACK 且未变化的响应会跳过重复发送
- RuntimeSnapshot 更新或删除后，会通知已连接 ADS stream 主动推送已订阅类型的新响应

当前使用 Envoy 官方 generated proto：

```text
github.com/envoyproxy/go-control-plane/envoy
```

暂时不引入 Ingate 自有 proto。Envoy xDS 协议直接使用官方 proto；Ingate 自有 proto 等 Admin/Agent/Plugin RPC 边界明确后再设计。

### admin-api

`ingate-admin-api` 已从占位入口推进到可运行的 Gin HTTP 服务：

- `internal/adminapi/app`：启动参数、apiserver client 初始化、server 组装
- `internal/adminapi/server`：HTTP 生命周期和 Gin 路由注册
- `internal/adminapi/handler`：按资源子目录组织 HTTP handler
- `internal/adminapi/service`：按资源子目录组织业务用例
- `internal/adminapi/store`：按资源子目录封装 generated client 访问
- `internal/adminapi/pkg`：admin-api 内部公共能力，例如 response、requestid、middleware

当前已接入：

- `GET /healthz`
- `GET /api/gateways` / `GET /api/gateways/:id`
- `GET /api/routes` / `GET /api/routes/:name`
- `GET /api/upstreams` / `GET /api/upstreams/:name`
- `GET /api/runtime-snapshots` / `GET /api/runtime-snapshots/:name`
- `GET /api/gateways/:id/overview`（历史接口，当前已移除）

当前这个 Admin API 只是让服务跑起来并验证分层，不应作为最终前端契约。后续需要先设计前端信息架构和页面流程，再反推 Admin API 的 DTO、CRUD 语义和聚合接口。前端不应该直接依赖 Kubernetes 风格资源对象；`Gateway / Route / Upstream / RuntimeSnapshot` 可以作为内部资源模型，但 Admin API 应返回面向页面和用户操作的产品 DTO。

## 设计取舍

### 为什么不直接依赖 Kubernetes CRD

目标是同时支持 K8s、VM、裸机，所以内部控制面不应直接依赖 Kubernetes 作为唯一控制面。

当前路线是：

```text
自研 ingate-apiserver + Kubernetes API Machinery + generated client/informer
```

后续 K8s 可以通过 operator / adapter 接入，但不是唯一真相源。

### 为什么不用本地 manifest / 文件桥

用户明确指出本地 manifest 后面可能删除，不应在非主线能力上磨太久。

当前方向是把真实链路打通：

```text
apiserver resource -> informer watch -> controller reconcile -> RuntimeSnapshot -> xDS watch
```

不要再加临时文件 store、内存状态服务或 file bridge。

### 为什么 xDS Listener 默认绑定 `0.0.0.0`

当前内部 xDS target 的 `Listener` 只表达网关监听端口和对应的 `RouteConfig`，还没有把 bind address 设计成 Gateway API 的领域字段。

Envoy `Listener` 又必须有明确监听地址，所以当前生成 LDS 时使用 `0.0.0.0` 作为默认 bind address：

- 符合 API Gateway 默认对外接收流量的语义
- 本地开发、容器和常见 VM 部署都可以直接工作
- 比 `127.0.0.1` 更接近真实网关部署形态

代码中使用 `defaultBindAddress` 常量，是因为这是明确的运行时默认值，不应散落成魔法字符串。后续如果 Gateway API 增加 `spec.listeners[].address` 或类似字段，再把该默认值下沉为“用户未配置时的默认值”。

### 为什么当前资源是非命名空间

当前内部模型没有租户、工作空间、RBAC、配额等完整隔离设计。直接使用 Kubernetes Namespace 会过早绑定 K8s 语义。

当前判断：

- `Gateway`：倾向全局资源
- `Route / Upstream`：后续可能属于 Workspace/Project
- `AIProvider`：可能全局，也可能租户级
- `Consumer / Credential / Policy`：大概率需要租户或工作空间隔离

后续应该单独设计产品隔离模型，而不是现在直接套 Kubernetes Namespace。

## 编码约定

遵守 `AGENTS.md`。尤其注意：

- 使用 Go 1.26
- 注释使用中文
- 注释不需要以句号结尾
- 不要写没意义的防御性编程
- 不要过度封装小函数
- 尽量用 receiver 收拢行为，不要散落过多游离函数
- 不要提前定义接口，尤其不要只为测试定义接口
- 接口尽量定义在消费者侧
- 能定义常量的地方定义常量，避免魔法字符串和魔法数字
- enum-like 常量使用专用类型
- 文件内组织顺序尽量为：常量、变量、结构体、导出函数、工具函数
- 可以用新 Go 标准库能力，例如 `slices.Contains`
- 可以用 `lo`，但只有在让代码更清楚时使用
- 不写低价值测试，例如 CLI help 文案测试
- 完成开发后运行：

```bash
make test
make build
```

Git 规则：

- 用户说“提交”或“提交并继续”再提交
- 每次开发完成后说明做了什么、验证结果、下一步计划
- 不要自动提交未获确认的改动
- 不要提交 `_output/`、`.gocache/`、`.gomodcache/`

## 当前验证方式

常用命令：

```bash
make generate
make test
make build
```

最近一次开发完成后已验证：

```text
make test
make build
```

均通过。

## 下一步建议

当前主链路已经推进到：

```text
声明式输入资源 -> controller 编译 -> RuntimeSnapshot 输出 -> xDS ADS 消费
```

接下来不建议继续盲目补 Admin API CRUD。更合理的顺序是先设计前端，再反推后端契约：

1. 先做前端信息架构和页面草图
   - Gateway 列表页需要哪些摘要字段
   - Gateway 详情页如何展示 Listener、Route、Upstream、RuntimeSnapshot
   - Route / Upstream 的创建和编辑流程如何组织
   - 哪些操作是页面级动作，哪些只是内部资源字段

2. 基于前端设计重新定义 Admin API DTO
   - 不直接把 Kubernetes 风格资源对象暴露给前端
   - list 接口返回页面摘要 DTO
   - detail 接口返回详情 DTO
   - form 接口或 schema 明确创建/编辑需要的字段
   - overview 接口只保留页面真正需要的聚合信息

3. 再补 Admin API CRUD
   - Gateway create/update/delete
   - Route create/update/delete
   - Upstream create/update/delete
   - 先通过 generated client 写入 ingate-apiserver
   - handler 只处理 HTTP，service 承载产品语义，store 封装资源读写

4. 之后补最小可运行示例文档
   - 说明如何启动 `ingate-apiserver / ingate-controller / ingate-xds / ingate-admin-api`
   - 给出 Gateway/Route/Upstream 示例资源
   - 给出 Envoy ADS bootstrap 示例

5. 再让 compiler 支持真正的单 Gateway 编译
   - 目标：`reconcileGateway(name)` 不再需要读取全量 Gateway
   - 当前原因：compiler 会校验 Route 的全部 `ParentRefs`

## 不建议马上做的事

暂时不要优先做：

- 大量补 `AIRoute / AIProvider / Plugin / Policy` storage
- 复杂多租户模型
- Envoy ADS 完整协议
- K8s operator
- VM agent
- 插件 runtime
- AI runtime

这些都重要，但现在主线应该先把：

```text
声明式输入资源 -> controller 编译 -> RuntimeSnapshot 输出 -> xDS 消费
```

这条链路打通。

## 给下个 AI 的启动提示

可以直接使用下面这段作为下一轮对话起点：

```text
你在 /Users/lgc202/workspace/source/lgc202/ingate 仓库继续开发。
先阅读 AGENTS.md 和 docs/2026-04-29-ingate-next-handoff.md。
当前项目是 Ingate 的全新重写，不要参考旧 ../ingate。
已经完成 Gateway/Route/Upstream/RuntimeSnapshot 的 apiserver REST storage、generated client/informer/lister、controller informer watch、按 Gateway key reconcile、RuntimeSnapshot 写入，以及 ingate-xds watch RuntimeSnapshot 并响应 Envoy ADS。
ingate-admin-api 已接入 Gin，并按 app/server/handler/service/store/pkg 分层，当前提供只读资源接口和 Gateway overview。
Route、Policy 和 Plugin 的长期模型以 docs/superpowers/specs/2026-06-05-route-policy-plugin-model-design.md 为准，不要继续扩展 route.ingate.io/policy-bindings annotation 方案。
下一步优先把 Header 改写、超时、重试迁入 RouteSpec 强类型字段或 route filters，再反推 Admin API DTO 与控制台保存契约；不要直接把 Kubernetes 风格资源对象作为最终前端 API。
开发前先 git status，完成后运行 make test 和 make build。不要自动提交，除非用户明确说提交。
```
