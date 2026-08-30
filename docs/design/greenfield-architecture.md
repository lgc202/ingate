# Ingate 绿地架构与工程设计

## 文档状态

- 状态：实现基线候选稿
- 受众：负责架构评审、核心实现和长期维护的高级 Go 开发
- 范围：从管理入口到 Envoy 数据面的完整配置链路，以及鉴权、AI、观测、插件和助手的协作边界
- 兼容性：不兼容现有 Go 包、Proto、etcd key、数据库结构和进程装配
- 基线：Go 1.26、官方 Envoy、一个 Ingate 对应一个环境和配置域、一个配置域包含多个逻辑 Gateway

本文不是当前仓库的渐进式重构方案。当前实现只用于确认已经存在的产品能力、不可丢失的运行约束和已发生的设计问题。新仓库必须能够独立解释，读者不需要理解旧目录和旧类型。

## 1. 设计目标

### 1.1 产品目标

1. 对普通 HTTP API 和 AI 模型流量提供统一的入口、路由、上游和治理模型。
2. 支持声明式配置、乐观并发、配置 Watch、状态回写和 Envoy xDS 发布。
3. 配置管理、实时鉴权、AI 协议处理和流量观测可以独立扩缩容和降级。
4. 精简部署与企业部署使用同一套协议和代码路径，只改变副本数和基础设施形态。

### 1.2 工程目标

1. 每类状态只有一个权威所有者。
2. 每个组件都能说明输入、输出、依赖、并发模型和失败语义。
3. 领域判断和配置编译优先实现为同步、确定性函数。
4. 关键状态机、并发约束和崩溃恢复由自动化测试证明。
5. 包、类型、函数和变量名称表达领域职责、成本和副作用。
6. 新开发者可以从协议和目录理解主流程，不需要追踪多层转发代码。

### 1.3 非目标

- 不支持多租户控制面；生产、测试、机房或租户隔离通过部署多套 Ingate 实现。
- 不为其他数据平面预留接口；Envoy 是唯一数据平面。
- 不把 xDS、Envoy filter、ext_authz、ext_proc、Redis、Kafka 或 ClickHouse 暴露为用户资源。
- 不建立通用 CRUD、Repository、事件总线、工作流或数据平面适配框架。
- 不预留没有验收场景的 MCP、Agent、计费、审批和多集群抽象。
- 不以减少进程数或目录数作为架构目标。

## 2. 不可破坏的不变量

以下规则高于具体包结构和框架选择：

1. 用户声明的资源是配置事实；缓存、索引、编译结果和 xDS Snapshot 都可重建。
2. 只有 Resource API 可以直接访问 etcd。
3. 资源写入成功只代表已经持久化，不代表已经被数据面应用。
4. Controller 和 xDS 位于同一进程，已发布配置只保存在内存。
5. 任意资源 revision 必须产生确定的编译结果；输入相同则输出字节和排序相同。
6. Envoy NACK 不能覆盖用户声明，也不能被另一个 Envoy 的 ACK 掩盖。
7. Authz、AI 和 Envoy 处理同一请求时必须能够识别所使用的配置 revision。
8. Analytics、Assistant、插件目录和外部模型服务故障不能阻断核心配置 CRUD。
9. 不记录访问密钥、私钥、模型凭据、完整请求正文和内部错误详情。
10. 所有网络入口都验证调用者身份；内部网络不被视为可信边界。

## 3. 统一领域语言

### 3.1 核心流量模型

产品、Proto、Console、日志字段和代码统一使用：

```text
Gateway -> Route -> Upstream -> Endpoint
```

| 概念 | 职责 | 不负责 |
| --- | --- | --- |
| Gateway | 定义流量入口、监听地址、端口、协议和证书 | 请求匹配和上游连接 |
| Route | 定义请求匹配、治理行为和转发目标 | 保存真实实例地址 |
| Upstream | 定义如何连接一组后端端点或模型供应商 | 入口监听和客户端匹配 |
| Endpoint | Upstream 中一个可连接的地址、端口和权重 | 独立生命周期和治理策略 |

`Service` 不作为顶层产品资源。它在 Go 服务层、gRPC service 和业务系统中含义过多，而当前对象实际保存 Endpoint、负载均衡、健康检查、TLS 和模型供应商凭据，`Upstream` 更准确。

只有将来出现拥有独立负责人、API 契约、版本和 SLA，并能映射到多个 Upstream 的真实业务对象时，才引入 `Service`。届时关系是 `Route -> Service -> Upstream`，不能把现有 Upstream 直接改名伪装成 Service。

### 3.2 顶层资源

第一阶段只实现已经有明确执行链路的资源：

| 资源 | 核心内容 | 主要消费者 |
| --- | --- | --- |
| Gateway | Listener、TLS、启停状态 | Controller |
| Certificate | 证书链、加密私钥、有效期 | Controller |
| Route | Gateway 引用、匹配条件、UpstreamRef、治理行为 | Controller、Authz、AI |
| Upstream | Endpoint、TLS、负载均衡、健康检查或模型供应商配置 | Controller、AI |
| Caller | 调用方身份、访问密钥摘要、有效期 | Authz |
| IPRestrictionPolicy | IP 允许或拒绝规则、目标引用 | Controller |
| RateLimitPolicy | 请求速率和突发规则、目标引用 | Authz |
| TokenQuotaPolicy | Token 配额和 Caller 引用 | AI |
| HeaderTransformationPolicy | 强类型 Header 变换、目标引用 | Controller |
| MockResponsePolicy | 强类型模拟响应、目标引用 | Controller |
| PluginSource | 插件来源、信任和同步策略 | Plugin Reconciler |
| Plugin | 不可变版本、摘要、依赖和生命周期 | Plugin Reconciler、Controller |

API、AI 是 Route 和 Upstream 的强类型变体，不是平行的资源体系。暂不创建 MCP 类型；出现真实协议和验收场景后再扩展 `oneof`。

### 3.3 标识和版本

- `id`：UUIDv7，创建后不可变，所有引用只使用 ID。
- `name`：用户可编辑的展示名称，不参与资源定位。
- `object_version`：单个资源期望状态的乐观锁版本。
- `expected_version`：更新或删除时调用方要求匹配的版本。
- `config_revision`：整个配置域每次成功变更后递增的版本。
- `published_revision`：Controller 已生成并安装到发布器的配置版本。
- `applied_revision`：某个运行实例实际 ACK 并使用的配置版本。
- `release_version`：二进制或镜像版本。

第一版要求同类资源 `name` 经过 Unicode trim 和大小写折叠后唯一。唯一性不是通过 Admin 的分页 List 猜测，而是在同一配置 revision 上完成校验和原子提交。

### 3.4 核心领域规则

| 范围 | 必须满足的规则 |
| --- | --- |
| Gateway Listener | 同一配置域内不能产生相同 bind address、port 和 transport 的监听冲突；TLS Listener 必须引用有效 Certificate |
| Route 绑定 | Route 至少引用一个 Gateway；hostname 必须被目标 Gateway 接受；无法区分优先级的重叠 Route 拒绝提交 |
| Route 排序 | 精确 hostname 优先于 wildcard；精确 path 优先于最长 prefix；其余条件使用稳定字段顺序，不能依赖 map 或创建时间 |
| UpstreamRef | 普通 Route 至少引用一个启用的 HTTP Upstream；权重必须大于零；同一 Route 中 Upstream ID 不重复 |
| AI Target | AI Route 的公开模型名在 Route 内唯一；每条线路引用 Model Upstream，并明确真实模型名和权重 |
| Upstream | 至少一个 Endpoint；address、port 组合不重复；TLS server name、健康检查和协议字段与 Upstream 类型匹配 |
| 引用删除 | 被引用资源不能删除；Admin 基于 Snapshot 校验后以同一 base revision 提交，避免检查与删除之间的竞争 |
| Policy 目标 | target kind 必须在该 Policy 的允许集合内；同一 target 不能被两个同类型 Policy 直接重复绑定 |

Gateway 与 Route 同时存在 Policy 时，叠加规则必须按 Policy 类型定义，不能使用一条通用“子级覆盖父级”：

- IPRestriction：Gateway 和 Route 规则同时满足才允许，Route 不能绕过 Gateway 限制。
- RateLimit：Gateway 和 Route 配额分别计数，任一超限即拒绝。
- HeaderTransformation：先执行 Gateway，再执行 Route；同名 `set` 由 Route 的值生效，非法组合在编译前拒绝。
- MockResponse：一个请求最多命中一个；同时存在 Gateway 和 Route Mock 时拒绝配置，避免隐藏优先级。
- TokenQuota：只绑定 Caller，由 AI Route 的模型调用消费。

这些规则由 Admin biz 在提交前给出即时反馈，并由 Compiler 再次验证声明式事实。两处使用同一个纯领域规则包，不复制两套判断。

## 4. 系统拓扑

```text
                         management traffic
Browser / CLI
     |
     v
+----------------+       signed user identity       +------------------+
| ingate-console |---------------------------------->| ingate-admin-api |
| OIDC / session |                                   | product API      |
+----------------+                                   +----+---------+---+
                                                          |         |
                                              resource RPC|         |query RPC
                                                          v         v
                                                +----------------+  +------------------+
                                                |ingate-apiserver|  |analytics-query   |
                                                |resource truth  |  +------------------+
                                                +-------+--------+
                                                        |
                                                        v
                                                      etcd
                                                        ^
                                                        | Watch / Status / Lease
                                                        |
                                                +-------+---------+
                                                |ingate-controller|
                                                |Compiler+Delivery|
                                                |xDS              |
                                                +--+------+-----+-+
                                                   |      |     |
                                     authz config  | AI  |     | xDS
                                                   v      v     v
                                              +---------+ |  +---------------------+
                                              | authz   | |  | Envoy + AI companion|
                                              +----+----+ |  +----------+----------+
                                                   |      |             |
                                                 Redis    +-------------+
                                                                        |
Client ---------------------------------------------------------------->+--> Upstream
                                                                        |
                                                                        v ALS
                                                                +---------------+
                                                                | ingate-als    |
                                                                | local WAL     |
                                                                +-------+-------+
                                                                        |
                                                                        v
                                                                      Kafka
                                                                        |
                                                                        v
                                                               +------------------+
                                                               |analytics-ingest  |
                                                               +--------+---------+
                                                                        |
                                                                        v
                                                                   ClickHouse
```

Plugin Reconciler 和 Assistant 是可选组件，不进入核心同步链路：

```text
PluginSource/Plugin -> plugin-reconciler -> verified digest/status -> Controller
Assistant -> dedicated read-only Admin API -> Resource and Analytics projections
Resource audit outbox -> audit-exporter -> enterprise audit sink
```

## 5. 组件职责

### 5.1 `ingate-console`

- 提供静态前端资源和浏览器 BFF。
- 完成 OIDC Authorization Code + PKCE、会话、CSRF 和安全 Header。
- 向 Admin API 转发可验证的用户访问令牌，不注入未经签名的用户 Header。
- 对用户展示稳定错误 reason 的本地化文案。

Console 禁止直接访问 Resource API、Analytics、etcd 或 Redis，也不能承担 Admin API 的最终认证和授权。它可以在精简部署中与 Admin API 同 Pod，但逻辑身份和监听端口仍然分离。

### 5.2 `ingate-admin-api`

- 提供 `api/admin/v1` 产品协议及 HTTP/JSON 映射。
- 验证用户或服务身份并执行产品 RBAC。
- 将协议输入转换成领域 Draft，执行规范化和业务规则。
- 在一致的资源视图上执行引用、冲突、唯一性和运行规则。
- 通过 Resource API 原子提交资源变更。
- 组合 Resource 状态和 Analytics 查询结果，但不拥有二者的数据。
- 产生包含 actor、action、target、request ID 和变更摘要的审计上下文。

Admin API 保持 Kratos `server / service / biz / data` 分层：

| 层 | 内容 | 禁止 |
| --- | --- | --- |
| server | Kratos 装配、认证、授权、超时、日志、错误编码 | 领域判断 |
| service | Proto 参数校验、DTO 映射、调用 biz | 数据访问和跨资源规则 |
| biz | Draft、领域规则、用例编排、消费者侧接口 | Kratos transport 和数据库实现 |
| data | Resource、Analytics、Identity 等外部客户端实现 | 产品规则 |

### 5.3 `ingate-apiserver`

Resource API 是声明式资源的唯一持久化入口，也是唯一 etcd 客户端。它不是对 Admin CRUD 的再次包装。

Resource API 是内部控制面协议，不直接面向浏览器或普通用户。所有产品写入，包括 CLI 和 GitOps 客户端的声明式写入，都通过 Admin API 执行业务校验；只有 Admin 服务身份拥有 Commit 权限。

- 提供类型化 Snapshot、Get、List、Commit、Watch、Status 和 Lease RPC。
- 使用 etcd transaction 原子比较全局配置 revision 并提交一批资源变更。
- 单独存储期望状态和 Controller 状态。
- 加密敏感字段，执行消息大小、资源类型和权限边界校验。
- 在资源事务内写入不可丢失的审计 outbox。
- 对 Watch compaction、断线恢复和线性一致读提供确定语义。

新仓库不使用 Kubernetes Generic API Server。当前产品不需要 CRD、kubectl、admission webhook 或 Kubernetes API 聚合，却要承担其复杂依赖、安全默认值和运维面。Resource API 只实现 Ingate 实际需要的 etcd 语义，并通过契约测试证明正确性。

### 5.4 `ingate-controller`

Controller、Compiler、Delivery 和 xDS 位于同一进程。

- Watch Resource API 并维护一个不可变、完整的配置视图。
- 将同一 `config_revision` 确定性编译为 Envoy、Authz 和 AI 配置。
- 先向实时扩展发布对应 revision，再发布引用该 revision 的 xDS Snapshot。
- 跟踪每个 Envoy 和扩展实例的 ACK、NACK、连接和 applied revision。
- 写入资源编译状态和控制面发布状态。
- 通过 Resource API Lease 完成单活发布和热备切换。

Controller 禁止直接访问 etcd、修改用户期望状态，或在 Compiler 中访问网络、文件、时钟和全局变量。

### 5.5 `ingate-authz`

- 接收 Controller 产生的最小 Authz 配置，不读取通用 Resource API。
- 使用访问密钥摘要完成常量时间验证。
- 使用 Redis 执行共享限流。
- 按请求携带的 `config_revision` 选择配置，并短期保留上一稳定版本。
- 配置过期超过允许窗口后退出 readiness；受保护 Route 始终失败关闭。

Authz 是同步请求路径中的独立故障和扩缩容边界。

### 5.6 `ingate-ai`

AI ExtProc 与 Envoy 组成一个部署单元，每个 Envoy 配置一个本地 AI companion。

- 负责下游请求与上游尝试的流关联。
- 执行 OpenAI、Anthropic 等强类型协议转换。
- 使用 Redis 执行 Token 配额结算。
- 接收与该 Envoy 对应的 AI 配置 revision。
- 对并发请求数、单请求正文、关联状态和等待时间设置硬上限。

AI 不作为共享中心服务。没有 AI Route 时，Envoy 的普通路由不依赖该组件。

### 5.7 `ingate-als`

- 验证并规范化 Envoy Access Log 事件。
- 先持久化本地 WAL，再异步发布 Kafka。
- Kafka 成功确认后推进 WAL checkpoint。
- 使用稳定事件 ID 支持下游幂等写入。
- Kafka 暂时不可用时继续接收，直到 WAL 达到安全水位。

ALS 是有本地持久状态的组件。企业部署必须使用独立持久卷和稳定实例身份，不能把 WAL 放入临时容器文件系统。

ALS 只有在 WAL record 完整写入并满足配置的 group fsync 窗口后才确认接收。WAL segment 带版本、长度和 checksum；尾部残缺记录在恢复时截断并计数。磁盘达到硬水位时拒绝新日志并触发告警，不能在内存中无限积压。

### 5.8 `ingate-analytics`

Analytics 是一个领域、一个代码库、两种显式部署角色：

- `ingest`：消费 Kafka，批量写入 ClickHouse，成功后提交 offset。
- `query`：向 Admin API 提供只读聚合查询。

两种角色不共享 readiness。Kafka 故障不能使查询实例下线，ClickHouse 查询压力也不能阻塞消费循环。

Ingest 只在 ClickHouse 幂等写入成功后提交 Kafka offset。无法解码或违反永久 schema 约束的事件经过有界重试后进入 quarantine topic，并携带安全 reason；不能由一条 poison message 永久阻塞整个 partition，也不能静默跳过。

### 5.9 `ingate-plugin-reconciler`

- 同步 PluginSource，解析版本并校验摘要、签名和依赖。
- 使用受限网络身份下载不可信产物。
- 将观察结果和验证状态写回 Resource API。
- Controller 只接受已验证的不可变 digest，并在加载时再次校验摘要。

Plugin Reconciler 通过受 mTLS 保护的 Artifact RPC 按 digest 提供已验证产物。源仓库仍是持久化事实，Reconciler 的本地内容寻址缓存可以重新下载；Controller 不保存第二份插件事实，也不直接持有插件源凭据。

插件提供的业务配置继续由强类型 Policy 承载。产品 API 不暴露 Wasm VM、Envoy filter 或插件执行协议。

### 5.10 `ingate-assistant`

- 对话和执行状态存储在自己的 MySQL，短期租约存储在自己的 Redis。
- 只调用专用的只读 Admin Query API，服务端按方法执行 allowlist。
- 不访问 Resource API，不持有配置写权限，不复用管理员用户令牌。
- 模型输出视为不可信输入，不能直接形成资源变更。

Assistant 是默认关闭的独立信任域。

### 5.11 `ingate-audit-exporter`

- 通过 Resource API 领取审计 outbox 事件，不直接访问 etcd。
- 至少一次写入 Kafka 审计 topic 或企业指定的不可变审计目标。
- 目标确认后按 event ID ACK，Resource API 才删除 outbox 记录。
- 多副本通过独立 Audit Lease 单活导出，失去 Lease 后停止 ACK。

导出目标不可用时，资源读取继续工作，变更在 outbox 安全容量内继续提交；达到硬上限后拒绝新的 mutation 并返回 `AuditUnavailable`，不能静默丢弃审计记录。

## 6. 数据所有权

| 数据 | 权威所有者 | 持久化 | 其他组件如何读取 |
| --- | --- | --- | --- |
| 资源期望状态 | Resource API | etcd | 类型化 Snapshot/Get/List/Watch |
| 资源编译状态 | Controller | etcd 独立 status key | Resource API 查询 |
| 全局配置 revision | Resource API | etcd | Commit 返回、Watch 事件 |
| 审计 outbox | Resource API | etcd | Audit exporter |
| 已导出审计事件 | Enterprise audit sink | Kafka 或不可变审计存储 | 合规查询工具 |
| 当前编译结果 | Controller | 内存 | Delivery 内部 |
| Envoy Snapshot | Controller/xDS | 内存 | ADS |
| Rollout 摘要 | Controller | Resource API 的 controller status | Admin 状态查询 |
| 实例连接与 applied revision | Controller | 内存和指标 | 只读 ControlPlane Query RPC |
| Authz 配置 | Controller 产生、Authz 应用 | Authz 内存 | 版本化配置流 |
| AI 配置 | Controller 产生、AI 应用 | AI 内存 | 版本化配置流 |
| 请求限流和 Token 计数 | Authz/AI | Redis | 只有对应组件 |
| 待发布日志 | ALS | 本地 WAL | ALS 发布循环 |
| 传输中的观测事件 | Kafka | Kafka 日志 | Analytics ingest |
| 请求事实和聚合 | Analytics | ClickHouse | Analytics query API |
| Assistant 对话和执行 | Assistant | MySQL/Redis | Assistant API |
| 已验证插件产物缓存 | Plugin Reconciler | 本地可重建缓存 | Artifact RPC 按 digest 读取 |

任何缓存都必须注明来源 revision、重建方式、最大陈旧时间和失效行为。没有这些信息的缓存不允许进入设计。

## 7. Resource API 存储协议

### 7.1 Key 布局

```text
/ingate/v1/config/revision
/ingate/v1/resources/{kind}/{id}
/ingate/v1/status/{reporter}/{kind}/{id}
/ingate/v1/events/{config_revision}
/ingate/v1/audit/outbox/{config_revision}/{event_id}
/ingate/v1/leases/{name}
```

资源正文使用确定性 Protobuf 编码。敏感字段在进入 etcd 前使用带资源 ID、kind 和字段路径作为 AAD 的 AEAD envelope encryption。

### 7.2 一致快照

`Snapshot` 返回配置 revision、全部强类型资源和一个只在 Resource API 内解释的 opaque read token。Resource API 在一个线性一致事务中读取全局配置 revision 和资源前缀。调用方只能基于同一个 Snapshot 执行业务判断，不能组合多次 List 得到伪快照。read token 允许分页查询在 etcd compaction 前继续读取同一存储 revision，但产品 API 不暴露 etcd revision。

### 7.3 原子提交

```go
type Commit struct {
	BaseRevision Revision
	Mutations    []Mutation
	Audit        AuditContext
}
```

Resource API 在一个 etcd transaction 中：

1. 比较 `/config/revision == BaseRevision`。
2. 校验每项 Mutation 的对象版本前提。
3. 写入资源期望状态。
4. 将配置 revision 增加一。
5. 写入包含完整变更批次的事件记录。
6. 写入不含敏感值的审计 outbox。

比较失败返回 `RevisionConflict`，不执行部分写入。Admin API 重新读取 Snapshot 并重新运行纯业务判断；默认最多重试两次，超过后向调用方返回冲突。

全局 revision 会串行化配置写入。这是单配置域的有意选择：配置写吞吐远低于数据面流量，确定的一致性优先于无意义的写并行。

### 7.4 Status

Status 使用独立 key，不增加配置 revision，也不进入配置 Watch。`reporter` 隔离写入者：Controller 只写 ApplyStatus，Plugin Reconciler 只写 PluginStatus。Controller 更新 Status 时必须携带 `observed_object_version` 和当前 Lease epoch；Resource API 在同一事务中比较对象版本和 Lease epoch，旧 Controller 不能覆盖新资源版本或新领导者的结果。

```go
type ApplyStatus struct {
	ObservedVersion   ObjectVersion
	PublishedRevision Revision
	Result            ApplyResult
	Effective         bool
	Reason            ApplyReason
	Message           string
}
```

- `Pending`：`observed_version < object_version`。
- `Accepted`：当前资源已被 Compiler 接受；`effective=false` 可以表示显式停用或没有目标。
- `Rejected`：当前资源无法编译，`reason` 是稳定枚举，`message` 是安全说明。

Envoy 是否 ACK 不写入每个资源的 ApplyStatus。数据面实例状态属于 Rollout 和运行健康，不能与资源正确性混为一谈。

### 7.5 Watch

Watch 先发送一个完整 Snapshot，再发送严格递增的 CommitEvent。消费者只应用连续 revision；出现断点、etcd compaction、连接重建或进程重启时丢弃本地视图并重新获取 Snapshot。

事件记录允许多个资源作为一个批次提交，消费者不得观察到半个事务。Watch 具备有界发送队列；慢消费者被断开并要求重新同步，不能无限占用服务端内存。

CommitEvent 只保留配置的恢复窗口，不作为永久审计日志。Resource API 按 revision 和时间执行有界回收；客户端请求的起始 revision 早于最早事件时返回 `ResyncRequired`。审计 outbox 只有在 exporter ACK 后删除，并另外设置容量告警和 mutation 硬限制。

## 8. Admin API 写入与查询

### 8.1 写入流程

以创建 Upstream 为例：

```text
HTTP/gRPC request
  -> service: protocol validation and mapping
  -> biz: Normalize UpstreamDraft
  -> data: load consistent Resource Snapshot
  -> biz: validate name, references and domain rules
  -> data: Commit(base revision, Create Upstream)
  -> Resource API: atomic etcd transaction and audit outbox
  -> response: saved object with Pending apply status
```

协议格式在 service 层校验；名称、引用、冲突、版本和运行状态等系统规则在 biz 层校验。data 层只实现 biz 定义的窄接口。

### 8.2 Draft 和 Mutation Intent

创建和更新首先转换为不携带持久化元数据的 Draft：

```go
type UpstreamDraft struct {
	Name          string
	Endpoints     []Endpoint
	TLS           *UpstreamTLS
	LoadBalancing LoadBalancingPolicy
	HealthCheck   *HealthCheck
	Model         *ModelUpstreamDraft
}
```

同一 `NormalizeUpstreamDraft` 和 `ValidateUpstreamDraft` 同时服务创建与更新。更新额外携带 `expected_version`，而不是复制一套 createSpec/updateSpec。

敏感值必须表达 `Preserve`、`Replace`、`Clear` 三种更新意图，不得用空字符串或字段缺失同时表达多个含义。

### 8.3 查询

- Get 通过 ID 查询，不使用 name 作为主键。
- List 的 cursor 包含 kind、排序字段、最后 ID 和 opaque read token，并使用服务端密钥签名。
- cursor 对应 revision 已被压缩时返回 `CursorExpired`，客户端重新开始查询。
- Analytics 查询失败只影响 Analytics 相关接口，不改变 Admin 核心 readiness。
- Assistant 使用独立只读 Proto，不能因为复用 Admin 客户端而获得写方法。

## 9. 编译与发布

### 9.1 Compiler

```go
func Compile(view resource.View) (CompiledConfig, Diagnostics)
```

Compiler 是纯函数：

- 不接受 `context.Context`，因为不执行 I/O。
- 不访问网络、文件、环境变量、时钟和日志器。
- 所有 map 在输出前按稳定 key 排序。
- 不原地修改输入资源或生成的 Protobuf。
- 同时生成 Envoy、Authz 和 AI 所需的最小配置。
- Diagnostics 使用资源 ID、字段路径和稳定 reason，不包含凭据。

编译失败不会覆盖当前发布配置。Controller 为失败资源写入 Rejected Status，其他已发布流量继续使用上一配置。

一个 `config_revision` 是原子发布单元。只要其中任一启用资源无法编译，整个 revision 都不进入 Delivery；Compiler 不通过静默丢弃错误资源来产生“部分成功”配置。Admin API 应在提交前阻止可预见的领域错误，Compiler 的拒绝主要保护声明式并发、协议差异和实现缺陷。

### 9.2 发布顺序

同一 revision 的发布顺序固定：

1. 编译并完成静态一致性检查。
2. 向本 revision 使用的 Authz 实例发布 Authz 配置。
3. 向对应 Envoy companion 发布 AI 配置。
4. 等待所需扩展实例验证并 ACK；未使用的能力不参与等待。
5. 将 xDS Snapshot 安装到 Controller 内存发布器。
6. Envoy 获取新配置并分别 ACK 或 NACK。
7. 汇总 Rollout 状态和每个实例的 applied revision。

Envoy 发往 Authz 和 AI 的请求携带 `config_revision`。扩展组件保留当前版本和上一稳定版本，使配置发布、在途请求和回退期间仍能按正确版本处理。

### 9.3 Rollout 状态机

```text
Observed -> Compiled -> PreparingExtensions -> Published
                    \-> Rejected

Published -> Stable
          -> Degraded
          -> TimedOut
```

- `Stable`：发布开始时登记的所有 ready Envoy 都完成所需 type URL 的 ACK，并且所需扩展配置已就绪。
- `Degraded`：至少一个实例 NACK，其他实例可能已经 ACK。
- `TimedOut`：目标实例在期限内未完成确认。
- 发布期间新连接的 Envoy 接收目标 revision，但不改变本次完成条件。
- 断开的目标实例保留在等待集合直到超时，不能被当作成功。
- 发布开始时没有任何 ready Envoy，则状态保持 `Published` 并标记 `NoDataPlane`，不能伪装成 `Stable`。

默认在首次 NACK 时向已经 ACK 的实例重新发布上一 Stable Snapshot，减少同一数据平面的版本分裂。用户声明仍保持当前 revision，并显示 Rejected/Degraded；回退的是派生配置，不是资源事实。

冷启动时如果内存中不存在上一 Stable Snapshot，Controller 只重新编译当前资源。Envoy 会保留自身最后接受的配置；Controller 不伪造无法重建的历史 Snapshot。

### 9.4 Controller 高可用

- 至少两个 Controller 副本持续 Watch 并预编译最新 revision。
- 只有持有 Resource API Lease 的副本对外 ready、发布 xDS、推送扩展配置和写 Status。
- Lease 有明确 holder、epoch、过期时间和基于服务端时钟的续约语义。
- 失去 Lease 的实例立即停止接受新流并关闭现有发布入口，不继续以旧 epoch 写状态。
- Envoy 连接到稳定服务地址，领导者切换时自动重连并继续使用最后接受的配置。

不运行两个互不协调的发布者，也不把编译结果持久化为第二份配置事实。

## 10. 协议设计

### 10.1 产品协议

`api/admin/v1` Proto 是 Console、CLI 和外部管理客户端的唯一产品协议：

- 使用 `id`、`name`、`expected_version` 等产品术语。
- HTTP 路径使用复数资源名，例如 `/api/v1/upstreams/{id}`。
- 返回平铺产品对象，不暴露 etcd revision、xDS type URL 和内部 Status key。
- 字段新增遵守 Proto 兼容规则；删除字段必须 reserve tag 和 name。
- Console 客户端只由 Proto/OpenAPI 生成，不手写重复模型。

### 10.2 内部协议

内部协议按能力拆分，不创建一个全能 InternalService：

- `resource/v1`：Snapshot、Commit、Watch、Status、Lease。
- `audit/v1`：Claim、Ack 和 Audit Lease，不返回敏感资源正文。
- `runtimeconfig/v1`：AuthzConfig、AIConfig 的发布和 ACK/NACK。
- `controlplane/v1`：只读 Rollout 和数据面实例健康查询。
- `artifact/v1`：按不可变 digest 读取已验证插件产物。
- `analytics/v1`：只读查询。
- `assistantquery/v1`：Assistant 方法级只读投影。
- 官方 xDS 和 Envoy ext_authz/ext_proc/ALS 协议保持上游定义。

内部协议支持当前版本和前一个 minor 版本的滚动升级。每个长连接在握手时交换 protocol version、capability 和 instance ID；不兼容时明确拒绝，不靠未知字段碰运气。

### 10.3 错误协议

```json
{
  "error": {
    "reason": "UPSTREAM_VERSION_CONFLICT",
    "message": "The upstream was changed by another request.",
    "requestID": "01J...",
    "fields": []
  }
}
```

- `reason` 是稳定机器码，由领域或边界包定义。
- HTTP/gRPC 状态只在 service/server 边界映射一次。
- Console 根据 reason 本地化；服务端 message 使用安全英文兜底。
- 内部错误使用 `%w` 保留 cause，但不能返回客户端。
- 同一个错误只在拥有处理责任的边界记录一次。
- 日志记录 reason、operation、resource kind/ID 和 request ID，不记录敏感参数。

## 11. 安全设计

### 11.1 用户身份

- Console 使用 OIDC Authorization Code + PKCE。
- Admin API 自己验证 issuer、audience、signature、expiry 和 required claims。
- 本地管理员只作为可审计、可禁用的 break-glass 账号，不与普通登录共享配置。
- 用户权限按资源 kind 和 action 表达；默认拒绝。

### 11.2 服务身份

所有内部 RPC 使用双向 TLS，并从证书 SAN 解析服务身份。禁止依赖源 IP、容器网络或裸 `X-Forwarded-User`。

| 身份 | 最小权限 |
| --- | --- |
| Admin API | Resource Snapshot/Get/List/Commit、ControlPlane Query；不可写 Status 和 Lease |
| Controller | Snapshot/Watch、写 Status、获取 Controller Lease |
| Plugin Reconciler | 读取插件资源、写插件 Status、提供已验证 Artifact |
| Authz/AI | 只能连接各自 runtime config 方法 |
| Analytics Query | 只读 ClickHouse，不访问 Resource API |
| Assistant | 只调用 assistantquery allowlist |
| Audit Exporter | Claim/Ack 审计 outbox、获取 Audit Lease |

证书轮换必须支持新旧 CA 短期重叠，不通过关闭校验完成升级。

Envoy 的客户端证书 SAN 绑定不可变 `instance_id`，xDS `node.id` 必须与之匹配；同一 instance 的新连接以递增 connection epoch 取代旧连接。AI companion 使用同一 instance ID 的独立服务证书，Controller 只把该节点所需的 AI 配置发送给匹配身份。伪造 node ID、跨节点领取 AI secret 或匿名连接均在握手阶段拒绝。

### 11.3 Secret

- 访问密钥只在创建或轮换响应中返回一次；持久化 HMAC/密码学摘要，不保存可逆明文。
- Certificate 私钥和模型 API Key 使用 envelope encryption 存储。
- KMS/KEK provider 是真实外部边界，定义在 Resource API 消费侧。
- Secret 解密后只存在于需要它的进程内存，禁止进入错误、日志、指标 label、审计 diff 和 Status。
- API 返回 `configured`、指纹或有效期，不回显 secret。
- 密钥轮换使用明确的 overlap 窗口和状态，不原地覆盖导致流量瞬断。

### 11.4 审计

资源变更和审计 outbox 在同一个 etcd transaction 中写入。审计 exporter 至少一次输出到企业审计目标；event ID 用于去重。

审计记录包含：actor、service identity、action、resource kind/ID、request ID、trace ID、结果、时间和脱敏字段级变更摘要。读取 secret、权限变化和 break-glass 登录必须单独标记。

## 12. 高可用和故障语义

### 12.1 部署角色

| 组件 | 企业形态 | 状态性质 |
| --- | --- | --- |
| Console | 2+ 无状态副本 | 会话使用签名 Cookie 或共享会话存储 |
| Admin API | 2+ 无状态副本 | Resource API 保证提交一致性 |
| Resource API | 3+ 无状态副本 | etcd 3 或 5 节点 |
| Controller | 2+ 热备，单活发布 | 配置只在内存，可重建 |
| Envoy + AI | N 个配对实例 | Envoy 保留最后接受配置 |
| Authz | 2+ 无状态副本 | Redis 共享计数，本地版本化配置 |
| ALS | 2+ 有状态副本 | 每实例独立 WAL 和持久卷 |
| Kafka | 3+ broker | 副本和 ISR 按企业策略配置 |
| Analytics ingest | 2+ consumer group | Kafka offset + ClickHouse 幂等 |
| Analytics query | 2+ 无状态副本 | ClickHouse 查询 |
| Assistant | 可选 2+ | MySQL/Redis 独立 |
| Audit Exporter | 2+ 热备，单活导出 | outbox 位于 Resource API |

### 12.2 Readiness

- Console：静态资源和 Admin 反向代理配置可用。
- Admin API：身份配置和 Resource API 可用；Analytics 不参与核心 readiness。
- Resource API：etcd 线性一致读写、KMS 和审计 outbox 容量正常。
- Controller：初始 Snapshot 已同步、当前 revision 编译完成且持有 Lease。
- Authz：已应用允许陈旧窗口内的配置，Redis 可用。
- AI：已应用允许陈旧窗口内的配置；外部模型供应商不参与 readiness。
- ALS：WAL 可写且剩余空间高于安全水位；Kafka 故障显示 degraded，不立即下线。
- Analytics ingest：Kafka 和 ClickHouse 写入可用。
- Analytics query：ClickHouse 查询可用。
- Audit Exporter：持有 Audit Lease 且导出目标可用；其 readiness 不控制 Resource API 读取。

Liveness 只表示进程没有失去自我恢复能力，不能用依赖暂时不可用触发无休止重启。

### 12.3 故障隔离矩阵

| 故障 | 必须继续工作 | 允许失败 |
| --- | --- | --- |
| Analytics 不可用 | 配置 CRUD、Envoy 转发、Authz、AI | 报表和日志查询 |
| Kafka 不可用 | Envoy 转发、ALS 在 WAL 容量内接收 | 实时分析延迟增加 |
| Redis 不可用 | 不使用动态鉴权/配额的 Route | 受保护 Route 失败关闭 |
| AI companion 不可用 | 普通 API Route | 该 Envoy 上的 AI Route |
| Resource API 不可用 | Envoy 使用最后配置继续转发 | 配置 CRUD 和新发布 |
| Controller 切换 | Envoy 使用最后配置、现有请求 | 短时间新配置发布 |
| 外部模型供应商不可用 | 普通 API、其他供应商线路 | 受影响模型线路 |
| 插件源不可用 | 已验证插件和普通配置 | 新插件同步或升级 |
| 审计目标不可用 | 查询和现有数据面流量 | outbox 达到上限后拒绝配置变更 |

启动编排不能把可选依赖写成所有流量的前置健康条件。失败语义由实际使用该能力的 Route 决定。

## 13. 观测与运维

### 13.1 统一上下文

- 管理请求产生 request ID 并贯穿 Console、Admin、Resource API 和 Analytics。
- 内部 RPC 和数据面调用传播 W3C Trace Context。
- Kafka 事件携带 event ID、trace ID、config revision 和匿名化资源 ID。
- 高基数字段不作为 Prometheus label。

### 13.2 必备指标

- Resource API：commit latency、revision conflict、watch lag、compaction resync、audit backlog。
- Controller：observed/published/stable revision、compile duration、diagnostic count、lease epoch。
- xDS：connected nodes、per-node applied revision、ACK/NACK、rollout duration、version skew。
- Authz：decision latency、deny reason、config age、Redis latency、fail-closed count。
- AI：active correlations、body bytes、provider latency、stream error、quota result、config age。
- ALS：WAL bytes/age、Kafka publish latency、retry、dropped events；正常情况下 dropped 必须为零。
- Analytics：consumer lag、batch size、ClickHouse latency、dedupe、query saturation。

### 13.3 日志规则

- 使用结构化 `slog`，稳定字段名在共享 internal 包中集中定义，但不包装所有日志方法。
- 健康检查、正常 Watch keepalive 和正常轮询不记录 INFO。
- 错误只在决定重试、降级、拒绝或返回调用方的责任边界记录。
- 每个日志事件有稳定 event name，禁止用完整错误文本作为聚合维度。

### 13.4 运维接口

每个进程分离 `/livez`、`/readyz` 和 `/metrics`。pprof 默认关闭；启用时使用独立管理监听、mTLS 和网络策略，不与业务端口共享。

## 14. 新仓库结构

```text
api/
  admin/v1/                  # 产品协议
  internal/resource/v1/      # Resource API
  internal/runtimeconfig/v1/ # Authz/AI 配置发布
  internal/controlplane/v1/  # Rollout 和数据面健康查询
  internal/audit/v1/         # 审计 outbox 领取与确认
  internal/analytics/v1/     # 分析查询
  internal/assistantquery/v1/

cmd/
  ingate-console/
  ingate-admin-api/
  ingate-apiserver/
  ingate-controller/
  ingate-authz/
  ingate-ai/
  ingate-als/
  ingate-analytics/
  ingate-plugin-reconciler/
  ingate-assistant/
  ingate-audit-exporter/

internal/
  adminapi/
    server/
    service/
    biz/
      admin/                  # 写入和查询用例
    data/
      resource/
      analytics/
      identity/
  domain/                     # Admin 与 Compiler 共用的纯模型和规则
    gateway/
    route/
    upstream/
    caller/
    policy/
  resourceapi/
    server/
    store/                    # 唯一 etcd 访问包
    secret/
    audit/
  controller/
    compiler/
    delivery/
    rollout/
    status/
    server/xds/
  authz/
    server/
    credential/
    policy/
    quota/
    config/
  ai/
    server/
    stream/
    provider/
    quota/
    config/
  als/
    server/
    event/
    wal/
    kafka/
  analytics/
    ingest/
    query/
    clickhouse/
  plugin/
    catalog/
    artifact/
    verify/
  assistant/
  auditexport/
  identity/
  requestid/
  telemetry/
  tlsconfig/

migrations/
deploy/
docs/
scripts/
```

不存在顶层 `pkg`：Ingate 不是提供公共 Go SDK 的库。进程间通信只使用 Proto；同一 module 内只有无 transport、存储和框架依赖的纯领域模型与规则可以由 Admin biz 和 Compiler 共同导入。

### 14.1 依赖方向

```text
cmd -> server -> service -> biz <- data

adminapi/biz -> domain <- controller/compiler

controller/server -> delivery -> compiler
resourceapi/server -> store
authz/server -> credential/policy/quota
als/server -> event/wal -> kafka
analytics/ingest -> clickhouse
analytics/query  -> clickhouse
```

- biz 定义自己消费的接口，data 实现接口。
- 构造函数返回具体类型；调用方按需要定义接口。
- 不允许 biz 导入 Kratos server、etcd、Redis、Kafka、ClickHouse 或 Envoy transport 包。
- Compiler 可以导入资源类型和官方 Envoy Proto，但不能导入 server、data 或网络客户端。
- 只有 `resourceapi/store` 导入 etcd client。
- 共享包必须能用一句话说明用途；禁止 `common`、`util`、`helper`、`base`。
- 避免 `init()`；配置、注册和副作用通过显式构造完成。

### 14.2 装配

每个 command 只有：解析配置、构造依赖、启动生命周期、等待信号、优雅关闭。退出只发生在 `main`。

不使用生成式依赖注入隐藏对象图。构造顺序保持显式；当装配超过一个屏幕时按真实生命周期拆成少量构造函数，而不是引入容器。

## 15. Go API 与命名规则

### 15.1 包和类型

- 包名使用短小单数名：`upstream`、`compiler`、`rollout`。
- 类型表达领域名词：`UpstreamDraft`、`ConfigRevision`、`Rollout`、`LeaseEpoch`。
- 不使用 `BaseService`、`GenericRepository`、`ResourceManager`、`ConfigEngine`、`TaskProcessor`。
- gRPC 生成的 `UpstreamService` 只表示 RPC service；产品对象始终叫 `Upstream`。
- 首字母缩写保持 Go 惯例：`ID`、`HTTP`、`TLS`、`URL`。

### 15.2 函数

名称同时表达成本和副作用：

```go
NormalizeUpstreamDraft(draft)
ValidateUpstreamDraft(draft, view)
Compile(view)
LoadSnapshot(ctx)
Commit(ctx, change)
Watch(ctx, revision)
Publish(snapshot)
Activate(revision)
Reject(revision, reason)
```

- `Find`、`Lookup` 表示纯内存读取。
- `Load`、`List`、`Fetch`、`Watch` 表示可能访问存储或网络。
- `Create`、`Replace`、`Delete`、`Publish` 表示状态改变。
- 禁止含义空泛的 `Process`、`Handle`、`Do`、`Execute`；transport 的 `Handler` 除外。
- 简单 accessor 不加 `Get`。
- `context.Context` 始终是第一个参数，不存入 struct。
- 纯函数不接受无意义的 context、logger 或 interface。

### 15.3 变量

- 名称长度与作用域成正比；小循环使用 `i`，跨分支状态使用 `publishedRevision`。
- 使用 `err` 表示当前错误；包装时补充操作语义。
- 使用 `got`、`want`、`diff` 表达测试比较。
- 不在名称中重复类型：`upstreams`，不是 `upstreamSlice`。
- 不遮蔽 `error`、`string`、`len`、`copy`、`new` 等预声明标识符。

### 15.4 接口

- 接口定义在消费者包中，只包含该消费者真实调用的方法。
- 单方法接口使用行为名：`SnapshotLoader`、`Publisher`、`Clock`。
- 不为每个 struct 创建同名接口，不使用 `I` 前缀。
- 不只为 mock 创建接口；测试替身实现生产调用方本来就需要的边界。
- 外部边界类型在构造时验证并复制；包内不可变值不重复防御。

### 15.5 注释和文档

- 导出包和标识符使用完整 Go doc，说明契约而不是重复名称。
- 代码注释使用中文解释领域约束、非显然并发和安全原因。
- Go 标识符、日志、错误 reason 和内部错误使用英文。
- 不为显而易见的赋值、循环和 getter 写注释。
- 重要决策写 ADR，代码注释链接 ADR 的稳定编号而不是聊天记录。

## 16. 错误、并发与生命周期

### 16.1 错误

- 包只定义调用者需要分支处理的 sentinel 或类型化错误。
- 跨包传播使用 `%w`；进入网络边界后映射为稳定 reason，内部 cause 到此终止。
- 不同时 log and return；选择有处理责任的一处。
- 重试只针对明确可恢复的错误，并受 context deadline、次数和抖动退避约束。
- panic 只用于不可能继续的程序不变量，不能处理外部输入。

### 16.2 并发所有权

- 启动 goroutine 的对象负责停止和等待它。
- 长生命周期循环使用 `errgroup.WithContext`，任何关键循环退出会取消同组任务。
- Channel 必须有唯一发送所有者、关闭所有者和容量理由。
- Delivery 和 Rollout 各由单一事件循环拥有可变状态；外部通过命令和只读快照交互。
- 读多写少配置使用构造后不可变对象加原子指针替换，不对共享 map 原地修改。
- 所有队列、WAL、请求 body、关联 map、重试和并发数都有明确上限。
- `time.Time` 和 `time.Duration` 表达时间；业务测试通过窄 `Clock` 边界控制时间。

### 16.3 关闭顺序

1. 从 readiness 移除实例。
2. 停止接受新请求或新流。
3. 等待在途请求到 deadline。
4. 刷新可持久化批次并提交 checkpoint。
5. 关闭下游连接和后台循环。
6. 返回 `run()`，由 `main` 决定退出码。

## 17. 测试策略

测试不是补充工作，而是架构交付物。

### 17.1 单元测试

- Normalize、Validate、引用解析、错误映射使用表格测试。
- Compiler 使用语义 golden tests，对 Proto 使用 `cmp` 和 `protocmp`，不比较脆弱 JSON。
- 所有资源排序和 map 输入使用随机顺序重复运行，证明输出确定。
- 测试失败信息包含函数、输入、got 和 want，不使用 assertion framework。
- 测试 helper 首行调用 `t.Helper()`，清理使用 `t.Cleanup()`。

### 17.2 状态机测试

Delivery、Rollout、Lease、WAL 和配置缓存以显式事件表覆盖全部合法状态转换，包括：

- ACK、晚到 ACK、NACK、重复 NACK、未知 nonce。
- 多 Envoy 部分成功、断线、重连、超时和回退。
- Leader 失去 Lease、旧 epoch 写入、热备接管。
- Watch revision 间断、重复事件、compaction 和完整重同步。
- Authz/AI 新旧 revision 并存、过期和原子切换。
- ALS append、Kafka ACK、checkpoint 各崩溃窗口。

每个状态转换表中的一行都必须对应测试；关键状态机不接受只覆盖 happy path 的覆盖率数字。

### 17.3 存储契约测试

对真实 etcd 运行 Resource API 契约测试：

- 两个 Admin 副本基于同一 revision 提交时只有一个成功。
- 资源变更、revision、事件和审计 outbox 要么全部提交，要么全部不提交。
- Status 不增加 config revision。
- Watch 不丢失、不拆分事务，compaction 后要求重新同步。
- Secret 密文无法从 etcd 直接恢复，AAD 不匹配时解密失败。

Redis、Kafka、ClickHouse 和 MySQL 也使用真实组件进行边界测试，不用 mock 假装协议行为。

### 17.4 Fuzz、竞态和故障测试

- Fuzz hostname、path、header、cursor、资源解码和流式 AI 帧。
- 所有 Go 包执行 `go test -race ./...`。
- ALS 和 Analytics 注入断电、重复投递、网络超时和磁盘满。
- Authz 注入 Redis 超时、配置陈旧、凭据撤销和重复 Key。
- AI 注入半关闭流、上游重试、超大正文、客户端取消和供应商异常帧。
- Resource API 注入 etcd leader 切换和 compaction。

### 17.5 集成和端到端

最低端到端链路：

```text
Create Upstream
  -> Create Gateway and Route
  -> Resource Commit
  -> Controller Compile
  -> xDS Publish
  -> Envoy ACK
  -> real HTTP request
  -> ALS WAL
  -> Kafka
  -> ClickHouse
  -> Admin query
```

必须另有失败链路覆盖无效引用、Envoy NACK、Authz fail-closed、AI stream cancel 和 Kafka 恢复。

## 18. 合并和发布门禁

每个变更必须通过：

1. `gofmt`、`goimports`、`go vet`、`golangci-lint`。
2. `go test ./...` 和 `go test -race ./...`。
3. Proto lint、生成代码一致性和 `buf breaking`。
4. 受影响组件的契约和集成测试。
5. Compiler 或发布链路变更的 Envoy 端到端测试。
6. `govulncheck`、依赖许可证检查、SBOM 和镜像漏洞扫描。
7. 数据库迁移的 expand/contract 与滚动升级验证。
8. 文档、配置样例和实际默认值一致性检查。

覆盖率作为发现缺口的信号：纯领域、Compiler、状态机和存储事务包不得低于 90% statement coverage，但达到数字不能替代决策表、故障和并发测试。

代码评审必须回答：

- 新类型或接口为什么必须存在？
- 状态由谁拥有，能否被两个 goroutine 或组件同时修改？
- 失败后调用方看到什么，是否会重复记录或泄露信息？
- 是否引入无界内存、队列、重试或 goroutine？
- 名称是否准确表达领域、成本和副作用？
- 行为由哪项测试证明？

任何问题没有具体答案，变更不得合并。

## 19. 兼容、迁移与发布

- 产品 Proto 从 v1 开始执行兼容检查；新仓库没有理由发布一个随意破坏的 v1。
- 内部协议支持 N/N-1 minor skew，滚动升级顺序写入发布手册。
- Resource 存储使用显式 storage version；转换函数是纯函数并有双向 golden tests。
- MySQL 和 ClickHouse 使用 expand/contract migration，旧实例停止后才删除旧列。
- Controller 声明支持的 Envoy 最小和最大版本，升级前运行配置兼容套件。
- 镜像使用不可变 digest、生成 SBOM 并签名；一个发布版本不要求所有可选组件同时启用。
- etcd、ClickHouse、Kafka、MySQL 和 ALS WAL 分别定义备份、恢复和恢复演练，不把 Compose volume copy 当成企业备份。

## 20. 部署形态

### 20.1 Compact

用于本地开发、演示和小规模非 HA 环境：

- 每个启用组件一个副本。
- 单节点 etcd、Redis、Kafka 和 ClickHouse。
- Envoy 与 AI companion 配对。
- Analytics、Assistant、插件能力可以整体关闭。
- Audit Exporter 默认启用；开发环境可以输出到本地不可变文件，但仍走 outbox 和 ACK 协议。
- UI 和文档明确标注非 HA，不隐藏单节点风险。

Compact 不使用绕过 Kafka、绕过 Resource API 或关闭 TLS 校验的另一套代码路径。可以使用开发 CA 和单节点基础设施，但协议和安全判断保持一致。

### 20.2 Enterprise

- Console、Admin、Resource API、Authz、Analytics query 多副本。
- Controller 至少两个热备，Lease 单活发布。
- etcd 3/5 节点，Kafka 3+ broker，Redis 和 ClickHouse 使用受支持的 HA 形态。
- ALS 使用稳定身份和独立持久卷。
- Envoy 与 AI companion 按数据面容量水平扩展。
- NetworkPolicy、mTLS、独立服务账号、KMS、审计导出和备份演练默认启用。

## 21. 实施顺序

### 阶段 0：工程基线

- 初始化新 module、工具版本锁定、生成流程、CI 和许可证。
- 建立 Proto lint/breaking、Go lint/race、依赖和镜像门禁。
- 只创建第一条垂直链路需要的目录，不批量生成空包。

### 阶段 1：Resource API

- 实现 Snapshot、Commit、Watch、Status、Lease、encryption、audit outbox 和 Audit Exporter。
- 使用真实 etcd 完成并发、崩溃和 compaction 契约测试。
- 在此阶段证明多 Admin 副本写入不会破坏不变量。

### 阶段 2：Admin Upstream

- 实现身份、RBAC、错误协议和 Upstream CRUD。
- 打通 Console/CLI -> Admin -> Resource API。
- 固化 Draft、expected version、secret mutation 和命名规则。

### 阶段 3：Gateway、Route 和 xDS

- 实现 Gateway、Certificate、Route 领域规则。
- 完成纯 Compiler、单 Envoy xDS 和资源 Status。
- 真实请求通过 Envoy 到达 Upstream。

### 阶段 4：发布状态机和 HA

- 多 Envoy ACK/NACK、回退、超时、Lease 和热备切换。
- 建立 per-node applied revision 指标和运维查询。

### 阶段 5：实时能力

- Authz 配置发布、Caller、访问密钥和 RateLimit。
- AI companion、模型协议、TokenQuota 和版本关联。
- 每种能力在未启用时不成为普通 Route 的依赖。

### 阶段 6：观测

- ALS WAL、Kafka、Analytics ingest/query 和 ClickHouse。
- 完成至少一次、去重、消费延迟和故障恢复验证。

### 阶段 7：可选能力

- Plugin Reconciler 和强类型 Policy 扩展。
- Assistant 的只读能力和独立信任边界。

每个阶段都必须形成可运行、可观察、可测试的垂直切片。不得先铺满所有 resource/service/repository 文件再统一补实现和测试。

## 22. 明确拒绝的方案

### 合并 Console、Admin、Resource API、Controller 和 xDS

这些组件在身份、权限、HA、资源消耗和故障恢复上有真实边界。进程合并会使管理入口、配置存储和发布故障互相放大，也无法独立扩缩容。

### 将 Resource API 替换成 Admin 的 data 包

这会让每个 Admin 副本直接访问 etcd，破坏单一所有权，并使 Controller、插件和声明式客户端缺少稳定协议。

### 继续采用 Kubernetes Generic API Server

没有 Kubernetes 生态集成需求时，其认证、授权、admission、对象模型和依赖成本大于收益。实现范围明确的 Resource API，并用真实 etcd 契约测试验证 List/Watch/CAS，是更小且可审计的边界。

### 移除 Kafka 并让 ALS 直接写 ClickHouse

这会把请求接收、磁盘恢复、ClickHouse 批量写入和查询扩缩容重新绑在一起。Kafka 是观测事实的持久传输边界，企业版保留。

### 让 Authz 和 AI 直接 Watch 全部资源

它扩大权限和敏感数据暴露，并使同一请求可能使用不同 revision。Controller 只发布能力所需的最小运行配置。

### 每个资源生成一套 Service、UseCase 和 Repository

CRUD 形状相似不构成领域抽象。Admin 使用一个清晰的应用用例入口和按领域拆分的纯规则包；只有真实外部边界定义消费者侧接口。

### 自动把 Envoy NACK 写回用户配置

NACK 是派生配置的应用事实，不是修改用户声明的授权。系统可以回退内存 Snapshot，但必须保留声明并报告诊断。

### 用全局覆盖率代表代码质量

覆盖率无法证明竞态、崩溃窗口、状态转换和协议兼容。关键行为清单、真实依赖契约测试和故障注入是完成条件。

## 23. 设计验收标准

开始大规模编码前，本设计必须能够明确回答：

1. 每个资源和运行状态由谁写入？
2. 两个 Admin 副本同时修改时如何保证规则不被竞争破坏？
3. Controller 重启、Watch 断档和 etcd compaction 后如何恢复？
4. 多个 Envoy 分别 ACK/NACK 时页面和指标显示什么？
5. Authz、AI 和 Envoy 如何证明使用同一配置 revision？
6. Kafka、ClickHouse、Redis、AI 或 Assistant 故障会影响哪些请求？
7. 哪些进程能够读取哪类 secret，凭什么身份？
8. 每项核心不变量由哪个自动化测试证明？
9. 新增包、接口和 goroutine 的所有权如何解释？
10. N/N-1 升级和失败回退如何完成而不关闭安全校验？

任何问题只能用“以后实现时决定”回答，都表示设计仍不完整。数值型容量和 SLO 目标可以在压测后填写，但所有度量点、限额位置和失败策略必须在实现前确定。

## 24. 开源项目的取舍

- 从 NSQ 学习把可变状态和 goroutine 生命周期放在同一个明确所有者中，并围绕状态转换测试。
- 从 Pigo 学习把可替换策略留在真实调用边界，同时用大量行为测试保护主流程。
- 从 go-zero 学习工具链、生成、可观测和工程自动化，但不把框架级通用抽象复制进单一产品。
- 从 miniblog 学习小项目的可读分层，但不把教学型 CRUD 结构当作企业控制面的完成标准。

新仓库吸收的是可验证的设计原则，不复制任何参考仓库的目录数量、类型名称或框架习惯。
