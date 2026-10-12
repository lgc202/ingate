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
