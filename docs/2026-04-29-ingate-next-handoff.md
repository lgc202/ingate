# Ingate Next 交接文档

本文档用于给后续 AI 对话快速恢复上下文。当前仓库路径：

```text
/Users/lgc202/workspace/source/lgc202/ingate-next
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
```

最近关键提交：

```text
885e8b9 feat: index routes for gateway reconcile
c62383c feat: reconcile gateways by queue key
e624d76 feat: add controller informer loop
6b53f4b feat: generate gateway clients
ade13b3 feat: add upstream rest storage
b55734d feat: add route rest storage
3da7724 feat: add gateway status storage
9fbecfc feat: run ingate apiserver
6bc212d feat: add gateway rest storage
9128626 feat: add apiserver config skeleton
```

当前工作区在生成本文档前是干净的。

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

- `cmd/ingate`
- `cmd/ingate-admin-api`
- `cmd/ingate-apiserver`
- `cmd/ingate-controller`
- `cmd/ingate-xds`

其中：

- `ingate`：本地 CLI 和调试入口
- `ingate-apiserver`：声明式资源 API
- `ingate-controller`：watch 资源并做状态收敛
- `ingate-xds`：后续给 Envoy 提供 xDS
- `ingate-admin-api`：后续给前端管理端使用

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

现在不是全量 reconcile 所有 Gateway，而是按 Gateway key reconcile。

当前仍会读取全量 Gateway 列表，原因是现有 compiler 会校验 Route 的全部 `ParentRefs` 是否存在。后续可以继续把 compiler 改成真正的单 Gateway 编译，去掉这个全量 Gateway 读取。

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

优先继续打通输出链路，不要继续盲目增加更多资源类型。

推荐顺序：

1. 让 compiler 支持真正的单 Gateway 编译
   - 目标：`reconcileGateway(name)` 不再需要读取全量 Gateway
   - 当前原因：compiler 会校验 Route 的全部 `ParentRefs`

2. 新增 `RuntimeSnapshot` API 资源
   - 这是 controller 的输出资源，不是用户声明式输入资源
   - 建议先做非命名空间资源
   - 字段大致包括：
     - `spec.target`
     - `spec.gateway`
     - `spec.version`
     - `spec.config runtime.RawExtension`
   - 先不一定需要 `/status`

3. 给 `RuntimeSnapshot` 接入 apiserver REST storage
   - 类似 `Gateway / Route / Upstream`
   - 需要更新 `pkg/apis/gateway/v1/register.go`
   - 需要更新 `internal/apiserver/server/config.go`
   - 需要更新 codegen 注解并重新 `make generate`

4. controller reconcile 后 create/update `RuntimeSnapshot`
   - name 可以先用稳定格式，例如 `<target>-<gateway>`
   - 后续再决定是否加入 workspace/tenant 前缀
   - update 时注意 resourceVersion 语义

5. `ingate-xds` watch `RuntimeSnapshot`
   - 先 watch target=`xds` 的 snapshot
   - 不急着实现完整 Envoy xDS 协议
   - 可以先把 watch 到的 snapshot 保存在 xDS 服务内部状态

6. 做最小 e2e
   - 启动 apiserver
   - apply Gateway/Route/Upstream
   - controller 生成 RuntimeSnapshot
   - xDS 观察到 RuntimeSnapshot

## 不建议马上做的事

暂时不要优先做：

- 大量补 `AIRoute / AIProvider / Plugin / Policy` storage
- 复杂多租户模型
- Admin API 前端接口
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
你在 /Users/lgc202/workspace/source/lgc202/ingate-next 仓库继续开发。
先阅读 AGENTS.md 和 docs/2026-04-29-ingate-next-handoff.md。
当前项目是 Ingate 的全新重写，不要参考旧 ../ingate。
已经完成 Gateway/Route/Upstream 的 apiserver REST storage、generated client/informer/lister、controller informer watch 和按 Gateway key reconcile。
下一步优先让 compiler 支持真正的单 Gateway 编译，然后新增 RuntimeSnapshot API 资源并让 controller 写入，供 ingate-xds watch。
开发前先 git status，完成后运行 make test 和 make build。不要自动提交，除非用户明确说提交。
```
