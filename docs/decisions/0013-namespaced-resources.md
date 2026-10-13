# 0013：显式 Namespace 与同空间资源引用

- 日期：2026-10-12
- 状态：作用域、请求边界和原生 Namespace 生命周期实现方式已确认；编译身份改造已提交，存储与管理链路尚未切换
- 范围：替代 0011、0012 及资源契约中采用集群级主资源的决定；保留共享 Envoy、资源名称和展示名称契约

## 已确认契约

Gateway、HTTPRoute、ModelRoute、AIBackend、MCPRoute、Backend 使用 Namespace 作用域，不固定到一个后台空间。对象身份为同一 Kind 内的 `(namespace, name)`；不同空间允许相同技术名称，展示名称继续允许重复和修改。

管理请求必须显式指定 Namespace。管理 HTTP 集合路径采用 `/api/admin/v1/namespaces/{namespace}/<resources>`，详情、更新、删除再附加资源 name；gRPC 请求同样传递 Namespace。界面可以默认选中一个空间，但不得由后端偷偷把缺省空间或全空间查询当作请求目标。本仓库没有前端，这项默认选择尚未实现。

父引用和后端引用只在源对象自己的 Namespace 内解析，不增加引用上的 namespace 字段，不跨空间兜底查找同名对象。三类路径保持不变：

```text
同一 Namespace
Gateway ← parentRefs ─ HTTPRoute ───────────────→ Backend
        ← parentRefs ─ ModelRoute → AIBackend ──→ Backend
        ← parentRefs ─ MCPRoute ────────────────→ Backend
```

Namespace 不等于独立 Envoy，也不自动产生授权或网络隔离。所有空间的已接纳 Gateway 仍汇总到一个共享数据面；占用相同端口及 Host 的 Listener 必须一起裁决。沿用创建时间优先，再按 Namespace、name 稳定排序，不能因为 Namespace 不同就放行同一个物理入口的冲突。

## 依据和必要差异

复用 Kubernetes 的 [Namespace 与对象名称作用域](https://kubernetes.io/docs/concepts/overview/working-with-objects/namespaces/)，不建立 Tenant 或另一套标识类型。内部查询键直接使用 controller-runtime 的 `client.ObjectKey`。

Gateway API 的 [HTTPRoute 引用与排序定义](https://github.com/kubernetes-sigs/gateway-api/blob/8bb74df00e56ec8f944d48c25e6c1c9c2f6848e3/apis/v1/httproute_types.go)提供同空间引用缺省值及 Namespace/name 平局顺序；Ingate 当前只采用同空间子集，不实现跨空间附着或 ReferenceGrant。保持资源形状接近上游，不声明完整 Gateway API 兼容。

Higress 的 [watchNamespace 配置](https://higress.io/docs/latest/user/configurations/)区分控制器监听范围与资源本身的作用域；Console 使用配置的控制器空间。这是部署和管理边界的选择，不要求 Ingate 固定一个空间。Envoy Gateway 的 [合并 Gateway 部署模式](https://gateway.envoyproxy.io/v1.9/tasks/operations/deployment-mode/#merged-gateways-onto-a-single-envoyproxy-fleet)说明资源作用域与共享数据面是两个独立维度；本项目保留已有共享 Envoy，不增加部署编排。

## 当前第一步：编译和状态不能混用同名对象

修改前，Gateway/Backend 查询、HTTPRoute 状态归并和 Gateway 冲突归并均只以 name 为键。不同空间同名对象会覆盖引用目标或合并状态；编译出的 Host、Route、Backend 名称也会重叠。先修正这些消费者，避免 API Server 切换后立即出现错误转发。

- 引用、依赖集合、Listener 绑定、状态归并使用完整对象键。
- 编译标识包含 Namespace：例如 `gateway/team-a/edge/listener/http`、`team-a/api/rule/0/match/0`、`team-a/backend`。Envoy 适配层继续把这些标识当作不透明名称，不解析字符串恢复资源身份。
- 输入顺序不影响编译结果；创建时间相同时，以 Namespace/name 裁决。共享入口的冲突范围不缩小。
- 缺少本空间 Backend 时保留匹配并返回原有 500 动作，不转发到另一个空间的同名 Backend。

当前 API Server 的 Strategy 仍为集群级，Admin 和 MySQL 也还没有 Namespace。空 Namespace 的现有输入保持当前编译名称；这是存储切换前的阶段边界，不保留一套旧 API。新作用域行为目前由显式 Namespace 的 fake-client/纯编译测试验证，不宣称真实 API 已接受命名空间资源。

移除本轮未提交的 AIBackend 引用诊断草稿。用户要求的是编译并存储供 AI Proxy 消费的配置，不把单独写 Conditions 视为该能力；此次不新增 AI Watch、运行时资源或模型服务。

## 下一步：完整切换存储和管理链路

后续切片一并修改六种资源的 Strategy/REST Storage、生成客户端及 OpenAPI、管理 protobuf/各层请求传递、MySQL 查询模型和异步投影。所有精确读取、更新、删除、列表筛选、启动补齐和残留清理都必须带 Namespace，数据库主对象唯一索引改为 `(namespace, name)`；子记录仍关联数据库内部主键，不新增 position 表或通用资源表。

不保留集群级 CRUD 兼容入口，不同时维护两套身份，也不自动迁移或删除现有 etcd/MySQL 数据。按新项目方式修改迁移基线，实际运行数据重建仍需另行授权。

存储切换前先实现 Namespace 的原生资源入口、存在性校验和删除生命周期，不把任意字符串分组冒充完整 Namespace 管理。用户已确认下述实现方式；按空间的角色权限和跨空间授权不在范围内。

## 原生 Namespace 的实现方式

使用标准核心组 `v1` 的 Namespace，由调用方显式创建；不启用 OneX 的 NamespaceAutoProvision，不自动创建 Kubernetes 的全部系统空间。Namespace 本身是集群级资源，业务资源才属于 Namespace。

已发布的 `k8s.io/api`、`apimachinery`、`apiserver`、`client-go` 直接复用。Kubernetes 明确[不支持把主仓库作为外部库使用](https://github.com/kubernetes/kubernetes/blob/v1.37.0/README.md#to-start-using-k8s)，因此不引入 `k8s.io/kubernetes`，也不加入前次依赖探查中的 component-helpers、controller-manager、kubelet。Namespace 专属代码按 Kubernetes `v1.37.0` 受控移植，保留许可证和来源，逐项记录本地适配，升级时对照原始文件审查差异。

OneX 本地参考提交为 `10119504e12c316569337b0dbca04fb192c7d7ea`：其 Namespace 存储直接引用 Kubernetes 主仓库，删除协调器则在项目内适配 controller-runtime。Ingate 采用相同职责划分，不复制它的主仓库依赖和隐式自动创建行为。

完整生命周期的职责如下：

- API Server 安装 `/api/v1/namespaces`、`/status`、`/finalize`，复用 Generic Store；Namespace 专属 DELETE 先进入 Terminating，不能绕过 `spec.finalizers` 直接删除。
- 已发布的 NamespaceLifecycle 准入插件读取 Ingate 自己的 Namespace，不以宿主集群的 Namespace 作为存在性依据，也不改变已有委托认证、鉴权的目标。
- Controller Manager 根据 Discovery 清理命名空间级资源，保留 finalizer 等待、冲突重试和 UID 防护；不写死六种业务类型，不直接清空 etcd 前缀或 MySQL 表。

第一小步只提供 Namespace 内部对象、标准 `k8s.io/api/core/v1` 外部对象的转换和三类写入 Strategy。内部对象复用公开的 NamespaceSpec/NamespaceStatus 字段类型，不再复制等价枚举和条件结构。DeepCopy 和转换由仓库锁定的 Kubernetes 工具生成；Scheme 注册仅包含 Namespace/NamespaceList 及标准请求选项，不把整个 Kubernetes 核心资源集合注册进来。来源和本地差异记录在下表，原始许可证保存在 [KUBERNETES-LICENSE](../KUBERNETES-LICENSE)，不单独建立第三方源码目录。

| 本地文件 | Kubernetes v1.37.0 上游文件 | 本地适配 |
| --- | --- | --- |
| [内部对象](../../pkg/apis/core/types.go) | [pkg/apis/core/types.go](https://github.com/kubernetes/kubernetes/blob/v1.37.0/pkg/apis/core/types.go) 的 Namespace/NamespaceList | 保留内部 runtime.Object；Spec/Status、finalizer、阶段和条件类型使用公开的 core/v1 定义；DeepCopy/Conversion 由 v0.37.0 官方工具生成 |
| [字段校验](../../pkg/apis/core/validation/namespace.go) | [pkg/apis/core/validation/validation.go](https://github.com/kubernetes/kubernetes/blob/v1.37.0/pkg/apis/core/validation/validation.go) 的 Namespace 函数、metadata 包装与 finalizer 校验 | metadata 和语法检查调用 apimachinery；保留 metadata/spec 两类 finalizer 的标准名称限制；以公开常量替代主仓库 helper；阶段错误路径使用实际 JSON 字段 `status.phase`，而非上游的 `status.Phase`，错误说明文字简化 |
| [写入策略](../../internal/apiserver/registry/core/namespace/strategy.go) | [pkg/registry/core/namespace/strategy.go](https://github.com/kubernetes/kubernetes/blob/v1.37.0/pkg/registry/core/namespace/strategy.go) 的主资源/status/finalize 策略 | 构造器注入私有 ObjectTyper，不使用 legacyscheme 或全局 Strategy；不接入主仓库 DeclarativeValidation，现行校验由 validation 包显式完成；用 slices.Contains 补入缺失的 finalizer；按 GoCanon 顺序组织声明 |

上述文件基于上游源码适配，保留适用的上游版权、许可证和本地修改声明。目录调整不改变代码来源；不能把修改过的移植代码直接视为完全原创。升级时先比较固定版本文件与目标版本，再审查对应 Strategy、校验、转换和契约测试，不直接覆盖本地实现。后续默认值、REST Storage、字段选择器和删除控制器接入时补充本表；生成代码只能修改类型标记或生成脚本后重新生成。

Strategy 沿用标准行为：创建时设置 Active 并确保 `kubernetes` finalizer；普通更新保留 status 和 spec.finalizers；status 更新保护 spec；finalize 更新保护 status；不通过 Update 创建，允许标准 Namespace 无条件更新。Namespace 不机械套用网关 Spec 变化递增 Generation 的规则。字段校验由 API validation 包拥有，复用 apimachinery 的 metadata、名称及 finalizer 语法校验。

本步尚未安装原生 API、启用本地准入数据源或接入删除协调器，不改变 etcd 存储契约，也不切换业务 scope。默认值、Namespace 特殊 DELETE、存储版本和上述生命周期接线随下一步实现；不能把类型和 Strategy 测试描述成完整 Namespace 已可用。

本步按 GoCanon 快照 `f478423-dirty` 的 APIServer、参数校验、命名/函数、错误和测试草案审查；草案仍不作为已发布规则。逐个审查全部手写文件，核对声明顺序、三类字段保护和 metadata/spec finalizer 校验；生成文件单独核对结果与可重复性。外部对象、列表、编码往返与 DeepCopy 测试通过，名称、finalizer 和非法阶段反例通过。临时源码 overlay 移除普通更新的 Status 保护后，`TestPrepareForUpdate` 失败；错误实现未写入仓库。

`gofmt -l cmd internal pkg`、已跟踪及新增文件空白检查、`golangci-lint run ./...`（0 issues）、`go vet ./...`、`go test ./... -count=1`、`go build ./...` 均通过。生成脚本重跑后 `pkg/apis` 与 `pkg/generated` 的 77 个文件内容哈希一致，原有客户端和 OpenAPI 没有变化。构建仍有共享 module stat cache 写入受限的非致命提示。未修改 API Server 存储契约，本步不运行存储 integration；未验证完整 Namespace API、准入、删除控制器及真实组件联动。用户已 review 通过并授权提交此步。

## 第二小步：Namespace 存储生命周期

第一小步已提交为 `1785f754`。本步补入外部 Namespace 默认值、字段选择器和基于 Generic Store 的主资源/status/finalize 存储入口，暂不安装 `/api/v1`，不改变运行中的业务资源作用域、准入或 Controller Manager。

DELETE 对当前对象补入 UID 前置条件，首次请求在同一次 etcd CAS 更新中设置 deletionTimestamp 和 Terminating；后续请求必须等待 spec.finalizers。更新导致物理删除时，同时沿用 Generic Store 对 metadata.finalizers 和删除时间的检查。主资源不嵌入完整 Store，不提供绕过特殊 DELETE 的 DeleteCollection；status 只提供读取、更新和 Patch，finalize 只提供 Update。

| 本地文件 | Kubernetes v1.37.0 上游文件 | 本地适配 |
| --- | --- | --- |
| [默认值](../../pkg/apis/core/v1/defaults.go) | [pkg/apis/core/v1/defaults.go](https://github.com/kubernetes/kubernetes/blob/v1.37.0/pkg/apis/core/v1/defaults.go) 的 Namespace/NamespaceStatus 默认值 | 在私有 Scheme 中按仓库现有方式仅注册 Namespace 和 NamespaceList；空 phase 默认为 Active；名称标签与 Strategy Canonicalize 一致，不在默认值中补入 spec.finalizers |
| [字段转换](../../pkg/apis/core/v1/field_labels.go) | [pkg/apis/core/v1/conversion.go](https://github.com/kubernetes/kubernetes/blob/v1.37.0/pkg/apis/core/v1/conversion.go) 的 Namespace 字段转换 | 只接纳 metadata.name/status.phase；不增加 namespace 字段或旧 name 别名 |
| [选择器](../../internal/apiserver/registry/core/namespace/selection.go) | [pkg/registry/core/namespace/strategy.go](https://github.com/kubernetes/kubernetes/blob/v1.37.0/pkg/registry/core/namespace/strategy.go) 的 GetAttrs/MatchNamespace/NamespaceToSelectableFields | 改为包内 helper；不保留上游为历史错误兼容的 name 别名 |
| [REST 存储](../../internal/apiserver/registry/core/namespace/etcd.go)、[特殊删除](../../internal/apiserver/registry/core/namespace/delete.go) | [pkg/registry/core/namespace/storage/storage.go](https://github.com/kubernetes/kubernetes/blob/v1.37.0/pkg/registry/core/namespace/storage/storage.go) | 注入私有 Scheme，去掉未使用的主 REST status 字段；使用已发布的默认 TableConvertor，不移植主仓库打印器；复制 DeleteOptions，保留原有 metadata finalizer 顺序；沿用标准 DeleteOptions 传播语义，包括公开结构尚保留的 OrphanDependents，不保留旧业务 API |

存储构造器接收资源专属的 RESTOptionsGetter；集成测试只创建自有隔离 etcd，不复用运行数据。实际 API 安装仍需接入正确的核心组存储编码、OpenAPI 和本地 NamespaceLifecycle 数据源，删除控制器之后再接线。本步验证不能当作完整 Namespace HTTP API 或空间删除流程已可用。

删除的存储错误由公开 storage/errors 转换，保留标准 API StatusError；用 errors.As 判断已包装的状态错误，其余转换为 InternalError。允许直接调用存储时省略删除校验回调，不在返回错误的路径重复打印。错误对象类型的删除判断沿用框架错误处理并拒绝物理删除，不另建日志机制。

本步按相同 GoCanon 规则快照审查了 5 个修改和 9 个新增手写文件，包括存储委托方法、特殊 DELETE、默认值、字段转换、包注释、测试及本文；新增文件单独读取，不依赖 git diff。检查了声明分组、函数签名、错误邻接、字段保护、Options 深复制和标准传播语义。没有改变 API 类型字段、依赖或生成脚本，也没有新增或修改生成产物。

最终 `gofmt -l cmd internal pkg`、已跟踪及全部新增文件空白检查、`golangci-lint run ./...`（0 issues）、`go vet ./...`、`go test ./... -count=1`、`go build ./...` 和 `go test -tags=integration ./internal/apiserver -count=1` 均通过。build 的共享 module stat cache 权限提示仍非致命，退出码为 0。标准弃用字段的两项单元用例采用局部 staticcheck 排除，只验证公开 DeleteOptions 的已有语义，没有放宽整个仓库检查。

Namespace 存储集成用例验证三类写入的字段保护、不匹配选择器、禁止子资源创建、UID/resourceVersion 冲突、dry-run 不落库、删除校验拒绝、Terminating 持久化、重复 DELETE、两类 finalizer 等待、最终物理删除及同名重建后的旧 UID 防护。首轮直接调用 Store 的测试缺少框架要求的空 Namespace 请求上下文而失败，补入标准 request.WithNamespace 后重跑通过；没有通过修改业务策略来回避测试要求。

临时源码 overlay 移除 spec.finalizers 删除保护、忽略标签选择器后，TestShouldDeleteDuringUpdate/spec_finalizer 和 TestMatcher/different_label 分别失败；初次 overlay 构建失败不算断言验证，修正临时错误实现后才得到上述反例结果。错误实现未写入仓库。本步尚未验证原生 Namespace HTTP API、准入数据源、删除控制器或真实组件联动；用户已 review 通过并授权提交本步，提交前复核上述检查均通过。

## 第三小步：按 API Group 选择存储编码

第二小步已提交为 `f5de3d12`。当前 API Server 的 SimpleStorageFactory 共用 Gateway 的存储编码；Namespace 的核心 v1 无法沿此编码写入。接入原生 API 前，先使用已发布 apiserver 的 [DefaultStorageFactory](https://github.com/kubernetes/kubernetes/blob/v1.37.0/staging/src/k8s.io/apiserver/pkg/server/storage/storage_factory.go) 和 ResourceEncodingConfig，让两个 Group 各自写入外部存储版本、读取为本组内部类型；不新建 Codec 或 StorageFactory 接口。

Options 先构造 Factory 并调用 EtcdOptions.ApplyWithStorageFactoryTo，再用 RecommendedOptions 的浅副本调用其余 ApplyTo，仅在副本中置空 Etcd。这样保持原有初始化顺序，健康检查、加密转换与存储监控均只安装一次并使用同一个 Factory；原始启动选项不被关闭，其他组件选项不改写。资源专属服务器配置使用公开 ParseEtcdServersOverrides 和 SetEtcdLocation，不另写解析器。类型注册只追加已有 Namespace/NamespaceList，不注册 Pod、Secret 等核心业务资源。

默认 etcd 根前缀改为 `/registry`；Factory 使用资源名称作为默认前缀，Gateway 位于 `/registry/gateways/<name>`，Namespace 位于 `/registry/namespaces/<name>`。六种业务资源暂时仍是集群级，切换后再由 Generic Store 加入 namespace 层级。旧默认配置把根路径中的 gateway.ingate.io 与 SimpleStorageFactory 的 Group 前缀叠加，本步不为旧路径增加兼容映射，也不迁移或删除已有数据。启动参数 --etcd-prefix 仍由框架接收，含义为所有资源的存储根目录；部署环境的数据重建另行授权。

本步只完成私有 Scheme 和存储 Factory 接线，不安装核心 API，不修改管理协议、MySQL、准入或 Controller Manager。核心 OpenAPI、原生 API 安装和本地 NamespaceLifecycle 接入继续作为后续切片。

首轮按 GoCanon 快照 `f478423-dirty` 检查了 5 个修改和 2 个新增手写文件，但漏检了 Options 方法的文件归属、辅助函数先于调用入口和部分语义阶段黏连；首轮检查结论不能视为布局审查已经通过。单元检查覆盖两个 Group 的对象/列表存储版本、内部类型还原、编码不修改输入、未知 Group 拒绝、资源专属服务器与不匹配资源、解析错误保留原因、默认及显式计数器和自定义存储根路径。私有 Scheme 反例确认 Pod、Secret、Service、ConfigMap 未注册。Namespace 隔离 etcd 用例改用本步的 Factory 和官方 RESTOptionsGetter；已有 Gateway HTTP 契约用例经过实际 Options.Config 接线，两组存储检查均通过。

`gofmt -l cmd internal pkg`、已跟踪与两个新增文件空白检查、`golangci-lint run ./...`（0 issues）、`go vet ./...`、`go test ./... -count=1`、`go build ./...` 和 `go test -tags=integration ./internal/apiserver -count=1` 均通过。build 的共享 module stat cache 写入权限提示仍非致命，退出码为 0。未修改 API 字段、生成脚本、生成产物或依赖，不需要重新生成。未验证 Namespace HTTP API、本地准入数据源、删除控制器或真实组件联动，也未操作部署环境的存量数据。

2026-10-13 按 GoCanon MCP 新快照 `336a025-dirty`（revision `336a025bf2d761d1eac3cf7f5991d152e6d3d7e9`，规则源仍为 `/Users/lgc202/workspace/source/lgc202/gocanon`）重新读取开发入口、规范索引、命名/函数和 APIServer 草案。依据本仓库 AGENTS 明确采用的 GO-FUNC-012..014，把 applyRecommendedOptions 移回 options.go 的 Config 之后；storage.go 只保留 Factory 构造。存储配置准备、编码与资源策略、Factory 构造和返回分开；测试用例表与循环分开，编码、版本核对和解码阶段分开；长签名完整展开，错误赋值与对应检查保持相邻，不给同阶段连续断言机械插入空行。本次只调整声明归属和排版，未改动执行顺序、参数、返回值或断言。重新逐文件检查当前 7 个手写文件，上述全仓检查及隔离 etcd 集成检查均重跑通过。

用户授权复核标准实现方式后提交并继续。提交前对照锁定的 apiserver v0.37.0 的 RecommendedOptions、EtcdOptions 和 DefaultStorageFactory，确认使用公开的存储扩展入口，保持 Etcd 首先且仅应用一次，计数器与监控沿用官方机制；没有自行实现 Codec、StorageFactory 接口、加密或健康检查。再次审查全部手写变更及调用方，未发现阻止提交的问题，上述检查全部重跑通过。本结论只覆盖本切片，不等同于整个 Namespace 生命周期或生产部署已经验证。

## 第四小步：原生 Namespace OpenAPI 定义

第三小步已提交为 `8216baa7`。本步沿用 go.mod 锁定的官方 openapi-gen 和 scripts/generate-api.sh，把公开 core/v1 加入定义生成输入及 readonly-pkg；同时将已有只读 intstr 包加入输入，以补齐核心类型引用的 IntOrString。外部 Namespace 不复制或改名，继续使用 SDK 的 io.k8s.api.core.v1 模型名称。OpenAPI v2/v3 继续共用已有定义函数和私有 Scheme 的 DefinitionNamer，不手写 Schema，也不引入新的生成依赖。

锁定的生成器以包或类型上的 openapi-gen 标记选择类型，没有从外部包只选 Namespace 的 CLI 参数；core/v1 已标记整个包参与生成。因此生成包会包含其它核心类型的定义，这是机械生成范围，不改变私有 Scheme、存储启用集合、Discovery 或 API 安装；不为缩小产物而新增类型镜像、定制筛选器或手改生成文件。API 规则例外只接纳本次新增输入中已经存在的 Kubernetes 字段，逐条对照锁定的 SDK；项目自有 API 的违规仍使生成失败。

本步验证 Namespace 对象、列表、Spec、Status、Condition 定义及 Schema 依赖，不安装核心 HTTP API，不修改准入、业务 scope、MySQL 或删除控制器。原生 API 安装与本地准入数据源仍是后续独立切片。

首轮生成被规则报告门禁拒绝：core/v1 增加 25 项既有字段命名例外，intstr 增加 3 项；逐项对照 v0.37.0 的 types.go 和 intstr.go 后更新例外基线，没有新增 Ingate API 例外。缺少 IntOrString 的首轮产物被依赖断言拒绝，报告 HTTPGetAction、ServicePort、TCPSocketAction 的缺失引用；补齐输入并完成生成后，依赖检查通过。Namespace v2/v3 均由官方 Builder 构建，确认五类 Namespace 定义可达，并检查 Pod 不进入这两个根对象的文档，未用自写遍历器替代官方构建。

逐个审查本次 4 个手写文件：生成脚本、例外基线、Options 测试及本文；按 GO-FUNC-012..014 核对测试入口顺序、调用与错误检查相邻、配置构造与结果断言分段，不改变生产配置链。生成文件单独核对来源、注册项和依赖；连续两次完整生成前后 pkg/apis 与 pkg/generated 的 80 个已跟踪文件哈希一致，SDK 核心模型名文件也未变化。实际差异仅包含 OpenAPI 生成文件，DeepCopy、Conversion、ModelName 和 Clientset 没有变化；go.mod/go.sum 未修改。

`bash -n scripts/generate-api.sh`、`gofmt -l cmd internal pkg`、`git diff --check`、`golangci-lint run ./...`（0 issues）、`go vet ./...`、`go test ./... -count=1`、`go build ./...` 和 `go test -tags=integration ./internal/apiserver -count=1` 均通过。build 的共享 module stat cache 写入提示仍非致命，退出码为 0。集成检查只证明已有 HTTP/存储契约仍可运行，不证明 Namespace HTTP 入口、本地准入、删除控制器或真实组件联动已经完成。用户已授权提交并继续；提交前再次逐个复核手写文件并重跑上述检查，全部通过。

## 第五小步：安装原生 Namespace HTTP API

第四小步已提交为 `568b91ec`。本步在既有 Server 配置链中安装核心组 `v1`，只挂载 Namespace 主资源、status 和 finalize。`registry/core/storage.go` 聚合已有 Namespace 存储；`internal/apiserver/core.go` 按组创建 APIGroupInfo，调用锁定的 apiserver v0.37.0 的 NewDefaultAPIGroupInfo 和 InstallLegacyAPIGroup。核心组的标准前缀是 `/api`；不把空 Group 安装到 `/apis`，不编写自有 HTTP Handler 或复制 CRUD、Discovery、OpenAPI 路由。

安装失败时先销毁本组尚未交付的存储，Server.New 再销毁 GenericAPIServer；安装成功后存储销毁归框架所有，子资源不重复关闭共享 Store。Server 先安装核心组再安装业务组，任一失败都不进入 Run。新增 Group 聚合包沿用既有 registry/gateway 的层级，不创建通用组注册器、资源配置开关或新接口；没有增加依赖、API 字段或生成产物。

隔离 HTTPS 与 etcd 检查通过原生 client-go 验证：Discovery 的资源集合、集群作用域和动词精确匹配 Namespace 三类入口；v2/v3 实际端点包含 Namespace 定义，排除 Pod；v2 文档包含四条 Namespace 路径；Pod 与错误 `/apis/v1/namespaces` 请求返回 NotFound。写入链覆盖服务端身份及默认值、重复创建拒绝、过期版本冲突、普通更新保护 status/finalizers、选择器匹配与不匹配、status 更新保护 finalizers 和保存 Conditions、dry-run 不落库、DELETE 进入 Terminating 而不提前删除、finalize 拒绝 Patch，以及显式清空 finalizer 后对象被删除。

本步不启用本地 NamespaceLifecycle 数据源，不自动创建系统空间，不接 Namespace 删除控制器，不切换六种业务资源的存储、Admin 或 MySQL。当前 DELETE 后仍需显式 finalize，不能表述为已具备命名空间内容自动清理。集成测试沿用隔离启动配置，关闭委托认证、鉴权、Admission 和 CoreAPI；不证明生产权限和宿主集群交互。测试辅助函数只抽出原有 Server/etcd 生命周期，原 Gateway 动态客户端入口不变。

GoCanon MCP 再次确认规则源 `/Users/lgc202/workspace/source/lgc202/gocanon`、revision `336a025bf2d761d1eac3cf7f5991d152e6d3d7e9`、启动快照 `336a025-dirty`。按 APIServer、命名/函数、错误及测试规范审查；草案仍只是 review 依据，仓库 AGENTS 的 GO-FUNC-012..014 约定单独执行。逐一审查 7 个 Go 文件和 4 个文档，包含 4 个未跟踪文件；检查声明顺序、错误相邻、语义分段、共享存储所有权及真实调用路径。同步包注释、文档索引、能力快照和凭据稿的 Namespace 现状，不改变凭据设计的待评审状态。

`gofmt -l cmd internal pkg`、已跟踪及未跟踪文件空白检查、`golangci-lint run ./...`（0 issues）、`go vet ./...`、`go test ./... -count=1`、`go build ./...` 和 `go test -tags=integration ./internal/apiserver -count=1` 均通过。API 类型未修改，无须重新生成；pkg/apis、pkg/generated、go.mod/go.sum 无差异。本轮没有验证失败安装下的 Destroy 调用次数、生产认证授权、自动清理控制器或真实组件整链路；现有成功启动与退出检查不替代这些证据。用户授权复核标准实现方式后提交并继续：再次对照锁定版本的安装与销毁契约、审查全部手写差异、重跑上述检查，未发现阻止提交的问题。下一步只接 NamespaceLifecycle 的本地数据源，保留宿主委托认证/鉴权和框架生命周期。

## 第六小步：官方 NamespaceLifecycle 使用本地数据源

第五小步已提交为 `e649d9f6`，未推送。本步复用锁定的 apiserver v0.37.0 的 NamespaceLifecycle 插件，通过其公开客户端与 Informer 初始化接口接入 Ingate 的 LoopbackClientConfig。不复制或改写插件的存在性、Terminating、保护空间、缓存回退和删除处理规则，不增加依赖或新 API。

原生 RecommendedOptions 先准备 HTTPS、Loopback 和宿主 CoreAPI，再执行 ExtraAdmissionInitializers。本地初始化器保留原额外初始化器，在通用宿主初始化之后，仅替换具体 Lifecycle 插件的客户端和 Namespace Lister；其它插件仍使用宿主依赖。宿主认证、鉴权、动态客户端和 SharedInformerFactory 不改写，也不因此消除原 CoreAPI 的外部依赖。本步不统一调整 Webhook/Policy 的 Namespace 选择器或授权语义。

本地工厂使用 client-go 的 SharedInformerFactory：PostStartHook 按框架 Context 启动；独立 readyz 检查反映已启动 Informer 的缓存同步；GenericAPIServer 的 Destroy 回调等待其退出。Options 应用或 Server 构造失败时关闭尚未启动的工厂；Admission 关闭时不创建本地初始化器或启动 Hook。框架的标准禁用插件参数保持有效，不自动创建 default、kube-system 或 kube-public。

单元检查覆盖本地 Active 放行、仅宿主存在时 NotFound、本地 Terminating/宿主 Active 冲突时 Forbidden、不存在空间的 Update/Delete 保持原插件行为，以及其它插件仍获得宿主客户端与工厂。错误检查覆盖缺失或非法 Loopback、原额外初始化器失败保留错误原因与原函数、Admission 关闭时不注册本地 Hook。

集成检查启动两个各自隔离的 HTTPS API Server 和 etcd，使用临时宿主 kubeconfig，沿实际 Options → Config → Server → PostStartHook → readyz 链运行官方插件；真实 List/Watch 和实时 Get 以本地 Namespace 为准。Gateway 属性直接交给运行中的插件检查，不通过 Gateway HTTP 创建请求，因为六种业务资源本步仍为集群级。测试只启用 NamespaceLifecycle，关闭委托认证、鉴权和 APF；不证明生产权限、完整默认准入链或全部宿主插件行为。

GoCanon MCP 来源和快照仍为 `336a025-dirty`，revision `336a025bf2d761d1eac3cf7f5991d152e6d3d7e9`。完整读取 Runtime 规范并结合已读取的 APIServer、命名/函数、错误、防御和测试规范，审查 7 个 Go 文件和 4 个文档（包含 3 个新增手写文件）；核对声明顺序、原初始化器保留、相邻错误检查、插件依赖边界和退出所有权。规范草案只作审查依据，不作为模板级或生产就绪认证。

`gofmt -l cmd internal pkg`、已跟踪及新增文件空白检查、`go vet ./...`、`go test ./... -count=1`、`go build ./...` 和 `go test -tags=integration ./internal/apiserver -count=1` 均通过。沙箱内 lint 三次因包加载器报 no go files to analyze 失败；使用正常缓存权限重跑 `golangci-lint run ./...` 得到 0 issues，没有修改依赖或运行 go mod tidy。没有修改 API 类型、生成产物或 go.mod/go.sum，无须重新生成。

按用户要求补充源码注释，解释准入职责、本地与宿主数据源、Informer/Lister、周期 resync 与 List/Watch 的区别、额外初始化顺序、readyz 和退出清理所有权；测试注释明确直接调用插件不等于业务 HTTP 已支持 namespaced 请求。只增加注释，不修改执行逻辑、签名、配置或断言。

用户确认准入代码布局后，将接线集中到 [admission_namespace.go](../../internal/apiserver/admission_namespace.go)，单元与集成测试分别使用 [admission_namespace_test.go](../../internal/apiserver/admission_namespace_test.go) 和 [admission_namespace_integration_test.go](../../internal/apiserver/admission_namespace_integration_test.go)。三个文件只改名，内容哈希保持一致。插件装配留在 API Server 组件包；需要实现自有独立准入规则时再放入 admission/<plugin>，不提前建空包或通用注册器。单资源字段校验与字段保护仍属于 validation/Strategy，异步清理仍属于 Controller。

提交前格式、空白、lint（0 issues）、vet、全仓普通测试和构建再次通过；首次全组 integration 因 TestNamespaceStorageDeleteValidation 启动时回环端口已占用而失败，没有运行到行为断言。未修改代码，单独重跑完整 integration 后通过（24.351s）；首次失败不记作通过。

用户授权提交并继续。提交前再次按声明顺序、错误路径、注释和生命周期边界审查全部手写文件，并复核生成目录与依赖未变。没有接入 Namespace 删除控制器、切换业务存储/Admin/MySQL、验证完整默认准入链或真实网关整链路；现有空间仍需显式 finalize。下一切片接入删除协调器，业务作用域切换之后才能验收空间内资源自动清理。

## 第七小步：Namespace 删除控制器

第六小步已提交为 `9182df44`，未推送。本步将 [deletion](../../internal/controllermanager/controller/namespace/deletion/deleter.go) 的删除内核和 [Reconciler](../../internal/controllermanager/controller/namespace/controller.go) 一并接入现有 Manager，形成一个待 review 的切片。不增加 API、配置或依赖；Manager 启动后协调正在删除的 Namespace，通过原生 finalize 释放自身 finalizer。六种业务资源尚未切换为命名空间级，不属于本步清理内容。

来源固定为 Kubernetes `v1.37.0`，标签解引用后的提交为 `f54c212e3a2f75d674b717a9b29052b20b60aefc`。移植代码保留上游 Apache 2.0 文件头；许可证仍见 [KUBERNETES-LICENSE](../KUBERNETES-LICENSE)。直接使用已经锁定的 client-go v0.37.0 的 Namespace、Metadata、Discovery 和 Retry，不依赖 Kubernetes 主仓库。

| 本地文件 | 上游文件与保留语义 | 必要适配 |
| --- | --- | --- |
| [deleter.go](../../internal/controllermanager/controller/namespace/deletion/deleter.go)、[operations.go](../../internal/controllermanager/controller/namespace/deletion/operations.go) | [namespaced_resources_deleter.go](https://github.com/kubernetes/kubernetes/blob/v1.37.0/pkg/controller/namespace/deletion/namespaced_resources_deleter.go)：发现资源、Background 批量删除及逐项回退、再次 List 检查、等待内容 finalizer、只移除自身 Namespace finalizer、冲突和 UID 防护 | 构造只保存依赖，不做 Discovery 或 Fatal；每轮发现并显式过滤集群级对象及子资源；没有 Pod API，省去 Pod 终止和排序分支；原生有界 RetryOnConflict 每次重读并核对 UID；失败返回调用方，Conditions 写入失败也阻止 finalize；聚合错误补充 GVR/空间上下文并保留原因；不导出未消费的接口 |
| [conditions.go](../../internal/controllermanager/controller/namespace/deletion/conditions.go) | [status_condition_utils.go](https://github.com/kubernetes/kubernetes/blob/v1.37.0/pkg/controller/namespace/deletion/status_condition_utils.go)：五类 Namespace Conditions、稳定消息、失败恢复、只在真假变化时更新时间 | 保留官方 Type/Reason/Message 契约；按调用顺序组织声明，补充中文约束注释；不机械套用网关资源的 observedGeneration，因为原生 NamespaceCondition 没有该字段 |
| [controller.go](../../internal/controllermanager/controller/namespace/controller.go) | [namespace_controller.go](https://github.com/kubernetes/kubernetes/blob/v1.37.0/pkg/controller/namespace/namespace_controller.go)：删除缓冲、内容等待与失败重试 | 保留官方缓冲时长和内容等待公式，沿 OneX 的职责划分适配 controller-runtime；使用现有 Manager 的缓存、Watch、队列、单 worker 和退出，不复制上游 Informer/worker 生命周期；事件过滤避免 status 写回自触发；缓冲起点及失败退避差异见下文 |

### Manager 接线与重试

运行入口沿用 `Options.Config → Config.New → SetupWithManager → Manager.Start(ctx)`。[Options](../../internal/controllermanager/options.go) 的私有 Scheme 注册公开的 core/v1 类型，供 Namespace Watch 使用；不注册 API Server 内部存储类型，也不因此安装其它核心组 API。[组合根](../../internal/controllermanager/manager.go) 使用同一份 Ingate REST Config 创建原生 Namespace、Metadata 和 Discovery 客户端；不使用宿主 Kubernetes 的连接。构造和注册阶段不读取 API，不启动工作任务，缓存同步及运行退出由 Manager 负责。

`Reconcile(ctx, ctrl.Request) (ctrl.Result, error)` 保留框架签名，按 Namespace 的集群级对象键读取缓存；删除内核使用直接客户端重读对象和执行写入。Watch 接纳初始 List 中已在删除的 Namespace，以及删除时间、UID 或两类 finalizer 变化；活动对象、仅 status/label 变化及已消失对象不触发内容清理。队列以对象键去重；单 worker 是当前运行选择，不代表跨 Manager 互斥或 HA。

协调流程为：`Namespace 删除事件 → 缓冲检查 → Discovery → 删除并检查内容 → status/finalize → 完成或重排`。沿用官方 5 秒删除缓冲；本地按 API 记录的 deletionTimestamp 计算剩余时间，已超过缓冲期的对象可立即协调，不像上游每次事件都重新 AddAfter 5 秒。这个缓冲给准入缓存传播留出时间，不保证跨资源原子性。内容 finalizer 等待使用官方 `Estimate/2+1` 秒的重排公式：15 秒估计对应 8 秒后检查，返回 nil error，不进入失败退避或重复错误日志。其它错误保留原因和空间身份，交给 controller-runtime 的默认失败重试；没有移植上游自定义的 60 秒退避上限，也不承诺完成期限。

GoCanon 0078 已完整读取，但当前 MCP 没有暴露 `gocanon_plan_controller`/`gocanon_add_controller`，且追加入口要求项目内直接声明的类型和生成基线，不能直接消费 SDK 的 Namespace 类型并改写现有手写 Manager。因此本步按已接受的 Manager 生命周期和原生 Builder API 手写接线，不创建伪资源类型、额外注册表或另一套 Controller 框架。

### 删除语义与验证边界

`New` 要求三个依赖指向同一个 Ingate API Server；不持有请求 Context、工作队列或运行协程。`Delete` 先读取 Namespace，忽略不存在、尚未进入删除或已经清空 spec.finalizers 的对象；通过 status 标记 Terminating，再清理 Discovery 中的空间级主资源。部分发现失败允许已知类型取得进展，但返回错误，不释放 Namespace finalizer。重复执行依靠原生 API 的 NotFound、resourceVersion 冲突和 UID 检查收敛，不承诺跨资源事务。

资源 finalizer 尚未完成时返回 `ResourcesRemainingError`；15 秒仅为官方等待估计，既不保证完成时间，也不会按该估计在 Delete 内等待；短时冲突退避仍由 RetryOnConflict 执行。无 finalizer 的意外残留仍视为失败。沿用官方对不支持 List 的资源处理，不据此声称对任意发现资源都能完成残留验证；Ingate 的持久主资源提供 List。操作不支持缓存保持官方的单实例生命周期语义，不承诺运行中 API 动词变更的重新探测。UID 防护针对 Namespace 更新/finalize，不额外宣称每次内容 Delete 都带 UID 前置条件。

测试使用官方 fake Namespace 与 Metadata 客户端，覆盖构造不联网、空/活动/已完成空间、批量删除、缺失动词和 HTTP 405 回退、另一空间同名对象保留、集群级及子资源过滤、内容 finalizer 等待、部分发现失败、非法版本、读取/删除/状态失败、无 finalizer 残留、同 UID 冲突恢复、有界冲突、同名 Namespace 重建及取消。Conditions 测试覆盖相同错误不重复更新、时间戳稳定、恢复清除和保留其它控制器状态。fake 不自动模拟批量删除及对象 finalizer；对应用例显式注入这些行为，不能当作真实 API Server 或 GC 证据。锁定版本 fake Finalize 的动作是 create/finalize，真实客户端发送 PUT；测试已按 SDK 事实校正探针，初次探针错误导致的失败不记作通过。

Namespace、Metadata 和 Discovery 都传递协调请求的 Context。Discovery 直接绑定锁定版本 client-go 的 `ServerPreferredNamespacedResourcesWithContext`；独立使用其原生构造器，保留默认请求超时。测试覆盖调用前取消、发现回调收到原 Context 及回调取消后原因保留、finalizer 不释放；没有通过真实 Discovery HTTP 请求验证取消时延。

控制器用例覆盖不存在/活动/已完成空间不访问删除客户端、首次删除缓冲、空空间完成、内容等待重排、读取及删除原因保留，以及 status/label 事件过滤的正反例。启动选项用临时 kubeconfig 验证私有 Scheme 识别 Namespace/NamespaceList 及原 Gateway，不识别内部 Namespace；`Config.New` 测试验证 Manager 可构造注册，没有启动缓存或 Watch。controller-runtime fake 要求删除对象具有 metadata finalizer，测试夹具额外设置该字段仅为满足 fake 约束，不改变生产 Namespace 契约。

GoCanon MCP 再次核对规则源 `/Users/lgc202/workspace/source/lgc202/gocanon`、revision `336a025bf2d761d1eac3cf7f5991d152e6d3d7e9`、快照 `336a025-dirty`，服务构建时间为 `2026-10-13T02:16:17Z`。由于 dirty 不固定未提交内容，已重新完整读取开发入口、索引和本次适用的 Controller、RUNTIME、命名/函数、错误、防御及测试规范；草案仍只作 review 依据，已接受的 Manager 生命周期和仓库 AGENTS 单独执行。审查范围包含全部 11 个新增、修改的手写 Go 文件及四个文档，按声明顺序、逐函数职责、错误路径、注释及语义空行检查，不以自动扫描替代人工判断。技术写作审查将固定版本依据、必要适配和未验证项分开记录。

删除内核初次 lint 发现错误文本大小写和泛型集合 API 调用问题，修正后检查通过，不将失败轮次记作通过。Manager 接线及测试补充后，`gofmt -l cmd internal pkg`、`git diff --check`、全部未跟踪文件的原生 Git 空白检查、`golangci-lint run ./...`（0 issues）、`go vet ./...`、`go test ./... -count=1`、`go build ./...` 和 `go test -race ./internal/controllermanager/controller/namespace/... -count=1` 均完成，未报告问题。未跟踪文件与 `/dev/null` 的 no-index 检查退出码 1 表示有差异，没有空白错误诊断；不把它误报为命令退出码 0。race 只证明本轮运行未报告数据竞争，不证明资源泄漏、任意并发调用或真实组件联动。

本步 API 类型、存储契约、生成产物和依赖未变，不重新生成代码或执行新的存储 integration。Manager Watch 已注册，但没有执行真实 Watch/Metadata HTTP/Discovery 与自动删除联动、生产认证授权及实库/Envoy 整链路验证；六种业务资源仍为集群级。保留变更等待用户 review。下一步切换六种业务资源的存储作用域及相应机械产物，再按已确认契约调整 Admin/MySQL；不增加跨空间引用或 Secret 功能。

### 用户 review 后的规范修正

用户指出代码不符合规范后重新审查，确认上轮人工审查遗漏了包装错误匹配、操作上下文、局部命名和复杂表达式排版。此前检查命令通过的结果仍成立，但不足以支持上一轮的手写代码审查结论。本轮只修正当前切片，不推进业务作用域或新增功能。

- 按 ERROR-009，将 Discovery 结构化错误的直接类型断言改为 `errors.AsType`，包装后仍生成官方的失败组数量说明；增加包装错误的状态协议反例测试。
- 按 ERROR-005/008，读取 Namespace、标记 Terminating、Discovery、内容 List/Delete 和 finalize 失败增加具体步骤信息，保留 `%w` 原因；Reconciler 不再重复添加删除内核已提供的空间身份。未新增错误分类或错误日志。
- 按 GO-NAME-008、GO-FUNC-012/014，将 `m`、`ns` 等宽作用域名称换为实际角色，Namespace 状态更新方法放在删除内容方法前；移除只有长度判断的 `finalized` 辅助函数，零值声明和读取、变更、写入阶段分别组织。
- 按 GO-FUNC-011 和 Go 函数、排版审查技能，长匿名函数签名完整换行，复杂测试字面量按字段展开，发现回调从构造调用中提取为具名局部变量。短且清楚的表达式保持原样，不按固定字符数或每个 return 插入空行。

新增用例仅验证 Namespace 读取/finalize 错误原因保留，以及包装后的 Discovery Conditions；已有冲突、有界重试、UID 防护和筛选反例继续运行。状态 Type/Reason、正常等待和调度方式不变；诊断错误文本新增步骤和残留数量。本轮修正后，格式及已跟踪/未跟踪文件的空白检查未报告问题，lint（0 issues）、vet、全仓普通测试、构建和 Namespace 定向 race 重新通过，不使用上轮结果替代本轮验证。按技术写作审查将具体遗漏、修正行为和验证边界分开记录；真实组件联动仍未执行，变更保持未提交。

## 已提交编译身份改造的审查和验证边界

使用 GoCanon MCP，规则源 `/Users/lgc202/workspace/source/lgc202/gocanon`，快照 `f478423-dirty`，revision `f4784239c945ba61e90bdfa588ce01f69b9403f5`。完整读取开发入口与规范索引，按 Controller Manager、命名、函数、错误、防御和测试规范审查；草案只作为审查依据，不当作已发布能力。

逐个检查全部新增和修改的手写文件，按 GO-FUNC-012 检查声明分组，再逐函数检查对象键、职责、错误路径、边界复制和语义分段。生成文件、API 类型、存储契约及依赖未改动。本步测试覆盖两空间同名对象、跨空间 Gateway/Backend 不匹配、状态独立与重复协调不写回、跨空间 Listener 冲突，以及 xDS 目标和端点保持独立。

| 本步检查 | 结果与证明范围 |
| --- | --- |
| 编译和协调反例 | 临时源码 overlay 使用修改前的实现，五项新增 Controller 用例均失败，分别暴露同名目标合并、跨空间附着、跨空间转发、父状态合并和冲突状态误归属；错误实现未写入仓库 |
| 新增 xDS 用例 | 通过；一个共享 Listener 下的两个空间分别关联独立 Cluster、EDS 名称及端点地址，快照引用检查通过；未启动真实 Envoy |
| `gofmt -l cmd internal pkg`、已跟踪及新增文件空白检查 | 通过；新增文件另行检查，不依赖仅覆盖已跟踪文件的 `git diff --check` |
| `golangci-lint run ./...`、`go vet ./...` | 通过，lint 为 0 issues；不能代替人工语义审查 |
| `go test ./... -count=1`、`go build ./...` | 通过；普通测试只使用自有回环服务。构建提示共享 module stat cache 写入受限，但退出码为 0 |

首次在沙箱内执行整个 Envoy 包测试时，两个已有 Server 用例无法绑定回环端口而失败；授权后全仓普通测试重跑通过，未把首次失败记为成功。本步没有改变 API Server 存储契约，不运行存储 integration；没有修改 API 类型或 protobuf，不重新生成机械产物。

这些测试不证明真实 API Server 命名空间请求、MySQL 索引/投影、Envoy ACK 与真实流量已验证。按用户决定，真实组件联动仍留到统一测试阶段；存储契约改变时再执行 API Server integration，并重新生成和核对机械代码。
