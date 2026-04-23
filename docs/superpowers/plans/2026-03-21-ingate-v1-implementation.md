# Ingate v1 Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 以服务驱动方式实现 `Ingate` v1，按 `apiserver -> controller-manager -> xds-server -> admin-api -> policy -> reliability` 的顺序推进，每个服务都具备可运行结果和对应学习文档。

**Architecture:** 采用 `generic apiserver + etcd + controller-manager + xds-server + Envoy` 主链路，但实现顺序严格按服务推进。共享网关核心保留在 `internal/gateway`，真正的学习和实现入口优先从 `internal/controlplane/*` 与 `internal/adminapi/*` 展开，并显式参考 `OneX` 的同类目录结构和代码组织。

**Tech Stack:** Go, Kubernetes apimachinery/generic apiserver, etcd, gRPC, buf/protobuf, Envoy xDS, Make, shell scripts.

---

## 实施原则

- 按服务推进，不按抽象层推进。
- 不要求一开始就掌握全部 K8s 知识。
- 每一阶段都要有：
  - 一个可运行服务
  - 一个最小验证路径
  - 一篇对应学习文档
  - 对应 `OneX` 参考入口
- 当前阶段需要的知识，当前阶段再补，不提前铺满。
- 当前执行方式不强制 TDD，先跑通最小服务，再补最值钱的测试。

## 服务推进顺序

1. `apiserver`
2. `controller-manager`
3. `xds-server`
4. `admin-api`
5. `policy`
6. `reliability`

## 总体目录基线

```text
cmd/
  apiserver/
  admin-api/
  controller-manager/
  xds-server/
  gateway/
  ingatectl/

proto/
  ingate/
    configsync/v1/
    discovery/v1/

pkg/
  apis/
    gateway/v1alpha1/
    policy/v1alpha1/
  generated/
    clientset/
    informers/
    listers/
    proto/
  apiserver/
  controller/
  xds/
  discovery/
  config/

internal/
  controlplane/
    apiserver/
    controller/
    xds/
    discovery/
  adminapi/
    contract/
    handler/
    biz/
    view/
    mapping/
  gateway/
    model/
    policy/
    ir/
    translation/
  pkg/
    options/
    tls/
    middleware/
    conditions/
    idempotent/
```

## 参考方式

后续每个服务阶段都要同时看两类材料：

1. `Ingate` 自己的 spec
2. `OneX` 对应服务代码

### OneX 参考基线

- `generic apiserver`：
  - `cmd/onex-apiserver`
  - `internal/apiserver`
  - `internal/controlplane/apiserver`
- `controller-manager`：
  - `cmd/onex-controller-manager`
  - `internal/controlplane/controller`
  - `internal/controller`
- `gateway / 产品服务`：
  - `cmd/onex-gateway`
  - `internal/gateway/{handler,biz,model,store,pkg}`
- 项目内共享件：
  - `internal/pkg`

## Stage 0: 仓库骨架与生成链路

**状态：已完成**

目标：
- 新仓库初始化
- 目录骨架建立
- 最小 `Makefile`
- 最小二进制可编译
- 设计与计划文档迁入新项目

验证：
- `make build-ingate-binaries`

学习文档：
- `docs/superpowers/learning/phase-00-repo-and-codegen.md`

## Stage 1: `apiserver` 服务

**目标**

先把 `apiserver` 做成一个最小可运行服务，而不是先做全量资源系统。

本阶段只要求：
- `pkg/apis/gateway/v1alpha1`
- `pkg/apis/policy/v1alpha1`
- `pkg/apis/scheme`
- `internal/controlplane/apiserver`
- `cmd/apiserver`
- 接入 `etcd`
- 支持最小资源 CRUD、list/watch

### 本阶段需要掌握的知识

只学这些：
- `generic apiserver` 是什么
- `TypeMeta/ObjectMeta`
- `Spec/Status`
- `Scheme`
- `registry/storage/admission` 的职责区别

不需要现在就掌握：
- controller-runtime
- CRD machinery 全套
- Gateway API 细节

### OneX 参考入口

- `/Users/guangcaili/workplace/code/onex/cmd/onex-apiserver`
- `/Users/guangcaili/workplace/code/onex/internal/apiserver`
- `/Users/guangcaili/workplace/code/onex/internal/controlplane/apiserver`

### Files
- Create: `pkg/apis/gateway/v1alpha1/register.go`
- Create: `pkg/apis/gateway/v1alpha1/types_gateway.go`
- Create: `pkg/apis/gateway/v1alpha1/types_route.go`
- Create: `pkg/apis/gateway/v1alpha1/types_backend.go`
- Create: `pkg/apis/policy/v1alpha1/register.go`
- Create: `pkg/apis/policy/v1alpha1/types_authpolicy.go`
- Create: `pkg/apis/policy/v1alpha1/types_trafficpolicy.go`
- Create: `pkg/apis/scheme/scheme.go`
- Create: `internal/controlplane/apiserver/app/server.go`
- Create: `internal/controlplane/apiserver/options/options.go`
- Create: `internal/controlplane/apiserver/registry/gateway/storage.go`
- Create: `internal/controlplane/apiserver/registry/policy/storage.go`
- Create: `internal/controlplane/apiserver/admission/validation.go`
- Modify: `cmd/apiserver/main.go`
- Modify: `tools/hack/generate-apis.sh`
- Modify: `tools/hack/generate-clients.sh`
- Create: `docs/superpowers/learning/phase-01-apiserver-and-k8s-api-machinery.md`

### Steps
- [ ] 定义 5 个最小资源类型与 `register.go`
- [ ] 建立 `pkg/apis/scheme` 聚合注册
- [ ] 接入最小资源生成链，输出到 `pkg/generated/{clientset,informers,listers}`
- [ ] 建立 `generic apiserver` 最小装配
- [ ] 先打通 `Gateway / Route / Backend` 的最小 CRUD、list/watch
- [ ] 再补 `AuthPolicy / TrafficPolicy` 的资源注册和最小 CRUD
- [ ] 写 `phase-01-apiserver-and-k8s-api-machinery.md`
- [ ] 运行验证：

```bash
make generate
make verify-generated
make build-ingate-binaries
# 启动 etcd
# 启动 apiserver
# 通过 HTTP/JSON 验证 CRUD 和 list/watch
```

### 完成标准
- `apiserver` 能启动
- 至少一个资源能完整 CRUD
- list/watch 可用
- 学习文档已补

## Stage 2: `controller-manager` 服务

**目标**

在 `apiserver` 可用的前提下，实现最小控制循环：
- watch 资源
- reconcile
- 构建最小 IR
- 回写基础 status

### 本阶段需要掌握的知识

只学这些：
- informer
- workqueue
- reconcile
- `Accepted / ResolvedRefs` 这类状态回写

### OneX 参考入口

- `/Users/guangcaili/workplace/code/onex/cmd/onex-controller-manager`
- `/Users/guangcaili/workplace/code/onex/internal/controlplane/controller`
- `/Users/guangcaili/workplace/code/onex/internal/controller`

### Files
- Create: `internal/controlplane/controller/app/server.go`
- Create: `internal/controlplane/controller/options/options.go`
- Create: `internal/controlplane/controller/controllers/gateway/controller.go`
- Create: `internal/controlplane/controller/controllers/route/controller.go`
- Create: `internal/controlplane/controller/controllers/backend/controller.go`
- Create: `internal/controlplane/controller/status/writer.go`
- Create: `internal/gateway/model/*.go`
- Create: `internal/gateway/ir/types.go`
- Create: `internal/gateway/ir/builder.go`
- Modify: `cmd/controller-manager/main.go`
- Create: `docs/superpowers/learning/phase-02-controller-manager-and-reconcile.md`

### Steps
- [ ] 建立 controller-manager 启动框架
- [ ] 接入 shared informer factory
- [ ] 先实现 `gateway/route/backend` 3 个最小 controller
- [ ] 在 `internal/gateway` 中实现最小共享模型和 IR
- [ ] 实现基础 status 回写
- [ ] 写 `phase-02-controller-manager-and-reconcile.md`
- [ ] 运行验证：

```bash
make build-ingate-binaries
# 启动 apiserver + controller-manager
# 创建 Gateway/Route/Backend
# 验证 status 和最小 IR 产出
```

### 完成标准
- controller-manager 能稳定 watch 资源
- 最小 IR 可产出
- 基础状态可回写

## Stage 3: `xds-server` 服务

**目标**

把控制面有效配置发布给 Envoy，先只打通最小 HTTP 主链路。

### 本阶段需要掌握的知识

只学这些：
- `LDS/RDS/CDS/EDS`
- `ADS`
- `snapshot`
- `ACK/NACK`

### OneX 参考方式

`OneX` 没有直接对应 Envoy xDS 组件，所以这里主要参考它的：
- 组件壳组织方式
- `internal/controlplane/*` 的分层方式

网关翻译链路仍以 Ingate spec 为准。

### Files
- Modify: `proto/ingate/configsync/v1/configsync.proto`
- Create: `internal/controlplane/xds/app/server.go`
- Create: `internal/controlplane/xds/options/options.go`
- Create: `internal/controlplane/xds/api/publish.go`
- Create: `internal/controlplane/xds/snapshot/store.go`
- Create: `internal/controlplane/xds/ads/server.go`
- Create: `internal/controlplane/xds/status/writer.go`
- Create: `internal/gateway/translation/http.go`
- Modify: `cmd/xds-server/main.go`
- Modify: `tools/hack/generate-proto.sh`
- Create: `docs/superpowers/learning/phase-03-xds-server-and-envoy.md`

### Steps
- [ ] 定义最小 `PublishConfig` 契约
- [ ] 生成 `pkg/generated/proto`
- [ ] 建立 xds-server 的 `api/snapshot/ads/status` 最小结构
- [ ] 实现最小 HTTP 翻译器
- [ ] 打通 `controller-manager -> xds-server`
- [ ] 接入一个最小 Envoy 验证路由
- [ ] 写 `phase-03-xds-server-and-envoy.md`
- [ ] 运行验证：

```bash
make generate-proto
make build-ingate-binaries
# 启动 apiserver/controller-manager/xds-server/envoy
# 发一个 HTTP 请求验证最小路由转发
```

### 完成标准
- Envoy 能通过 ADS 拉到配置
- 最小 HTTP 路由能生效

## Stage 4: `admin-api` 服务

**目标**

给用户提供产品化接口，而不是直接暴露资源对象。

### 本阶段需要掌握的知识

只学这些：
- 资源接口和产品接口的边界
- 写工作流与聚合读接口的区别
- mapping/view 层为什么存在

### OneX 参考入口

- `/Users/guangcaili/workplace/code/onex/internal/gateway`
- `/Users/guangcaili/workplace/code/onex/internal/usercenter`
- `/Users/guangcaili/workplace/code/onex/internal/nightwatch`

重点看：
- `handler`
- `biz`
- `model`
- `store`
- `pkg`

### Files
- Create: `internal/adminapi/contract/types.go`
- Create: `internal/adminapi/handler/http.go`
- Create: `internal/adminapi/biz/gateway.go`
- Create: `internal/adminapi/biz/route.go`
- Create: `internal/adminapi/view/route_status.go`
- Create: `internal/adminapi/mapping/resources.go`
- Modify: `cmd/admin-api/main.go`
- Create: `docs/superpowers/learning/phase-04-admin-api-and-product-workflows.md`

### Steps
- [ ] 建立 `admin-api` 启动框架
- [ ] 实现最小写工作流：创建网关、路由、backend
- [ ] 实现最小聚合读接口：查看路由状态
- [ ] 写 `phase-04-admin-api-and-product-workflows.md`
- [ ] 运行验证：

```bash
make build-ingate-binaries
# 启动 admin-api 和控制面
# 通过 admin-api 创建最小网关与路由
# 通过 admin-api 查看状态
```

### 完成标准
- 用户不需要直接操作资源对象，也能完成最小工作流

## Stage 5: `policy` 能力

**目标**

补最小 `AuthPolicy / TrafficPolicy`，让治理能力进入 IR 和 xDS。

### 本阶段需要掌握的知识

只学这些：
- 策略挂载
- 策略优先级
- 冲突检测
- 为什么策略不能直接耦合 Envoy 细节

### Files
- Create: `internal/controlplane/controller/controllers/authpolicy/controller.go`
- Create: `internal/controlplane/controller/controllers/trafficpolicy/controller.go`
- Modify: `internal/gateway/policy/*.go`
- Modify: `internal/gateway/ir/builder.go`
- Modify: `internal/gateway/translation/http.go`
- Modify: `pkg/apis/policy/v1alpha1/*.go`
- Create: `docs/superpowers/learning/phase-05-policy-attachment-and-merge.md`

### Steps
- [ ] 实现 `authpolicy` / `trafficpolicy` controller
- [ ] 实现最小 policy merge
- [ ] 让翻译器消费策略结果
- [ ] 写 `phase-05-policy-attachment-and-merge.md`
- [ ] 运行验证：

```bash
make build-ingate-binaries
# 启动整条主链路
# 验证最小 JWT/APIKey、timeout/retry/rateLimit
```

### 完成标准
- 至少一个认证和一个治理能力真实生效

## Stage 6: `reliability` 收口

**目标**

补齐恢复、HA 和最关键测试，不再扩大功能面。

### 本阶段需要掌握的知识

只学这些：
- 为什么 controller 一般做 leader election
- 为什么中间状态可以丢但必须可重建
- 测试为什么要分层

### Files
- Modify: `internal/controlplane/controller/app/server.go`
- Modify: `internal/controlplane/xds/app/server.go`
- Modify: `internal/controlplane/xds/status/writer.go`
- Create: `test/ingate/integration/minimal_routing_test.go`
- Create: `test/ingate/integration/policy_test.go`
- Create: `test/ingate/e2e/smoke_test.sh`
- Create: `docs/superpowers/learning/phase-06-reliability-and-testing.md`

### Steps
- [ ] 补最小 leader election
- [ ] 补幂等发布和恢复路径
- [ ] 建立最小集成测试
- [ ] 建立最小 e2e smoke
- [ ] 写 `phase-06-reliability-and-testing.md`
- [ ] 运行验证：

```bash
go test ./test/ingate/integration/...
sh test/ingate/e2e/smoke_test.sh
```

### 完成标准
- 关键主链路可重复启动、可恢复、可验证

## 阶段完成标准

每个服务阶段完成前都必须满足：
- 该服务可以单独启动
- 有最小可运行路径
- 有一篇对应学习文档
- 有最关键的最小验证

## 当前不做

- Gateway API / CRD 兼容
- 多集群
- Istio 集成
- 插件平台
- AI provider/model route
- 复杂灰度/镜像/流量编排

## 一句话结论

`Ingate` 后续应按服务推进：先学并实现 `apiserver`，再学并实现 `controller-manager`，再接 `xds-server` 和 `Envoy`，最后补 `admin-api`、策略和可靠性。这比先啃抽象层更适合当前阶段，也更贴近 `OneX` 的实践方式。
