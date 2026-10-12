# 0013：显式 Namespace 与同空间资源引用

- 日期：2026-10-12
- 状态：作用域和请求边界已确认；用户授权提交第一步编译身份改造，存储与管理链路尚未切换
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

存储切换前须明确 Namespace 的原生资源入口、存在性校验和删除生命周期，复用 Kubernetes 的机制与约束，不把任意字符串分组冒充完整 Namespace 管理。此处尚未批准 Namespace CRUD、级联删除、终止回收、按空间的角色权限或跨空间授权实现。命名空间化不能被描述成这些能力已经完成。

## 本步审查和验证边界

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
