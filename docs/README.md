# Ingate 技术文档

阅读顺序：先看当前能力和主链路，再看 AI 网关的已确认方向与待评审设计。

- [控制面增量编译与配置分发设计](design/control-plane-incremental-design.md)：方案选型、总体架构、资源结构、增量计算和 Delta 分发；结合 Envoy Gateway、Istio、Agentgateway 的设计说明采用理由及差异。
  六类资源统一接入 krt；Backend 网络结果、三类 Route 匹配和 AIBackend 协议声明按对象派生，AIBackend 通过按键 Fetch 关联网络后端。
  协调器读取编译集合，不再逐次 List 六类原对象并重做端点、匹配和协议转换。Gateway 与 HTTPRoute 为状态协调保留实例身份和 Generation，写回前核对当前对象。
  Gateway 变化驱动集中入口冲突裁决；三类 Route 通过父引用建立附着关系，再按入口索引聚合，匹配变化只重算相关入口，端点变化不重算路由表。
  发布边界核对异步集合的结果是否收敛，合并入口缓存并从唯一 Backend 集合填充网络结果。最终一致性核对、配置组装和快照发布仍遍历完整结果；Envoy 接入原生 Delta ADS，Go AI Proxy 暂时继续消费完整 ConfigMap。
  Envoy 快照使用 go-control-plane 的原生资源版本表，按类型和名称复用内容未变的消息，撤销时同时移除资源及版本；类型级聚合版本仅作 Delta 响应诊断，差异按逐资源版本计算。ADS 不保留 SotW 入口，Envoy 启动配置须使用 `DELTA_GRPC`。
  HTTP protobuf 按端口缓存 LDS/RDS，按 Backend 身份缓存 CDS/EDS；AI 入口按端口与 Host 缓存固定模型接口和 MCP 路径，合入公共 RDS 的结果按端口缓存。AI 出口按内容寻址目标复用 CDS/EDS 与路由，目标列表未变时复用出口 RDS；UDS 资源独立复用，无入口或出口时清理。
  缓存只保存当前有效键，校验失败保旧，通知失败保留新期望并重试。最终配置扫描、目标哈希、快照序列化和版本计算仍遍历全量。Delta 的订阅、变化、删除和已知版本重连已由原生 gRPC 客户端验证，尚未进行真实 Envoy 联调。
  Go 独立资源已定义为 [protobuf 契约](../api/aidataplane/v1/resource.proto)，并提供[控制面纯转换](../internal/controllermanager/controller/gateway/ai_resources.go)：按入口、模型规则和 MCPRoute 输出，保留来源与拒绝语义，复用 Envoy 的内容寻址出口。当前尚未接入 LinearCache、ADS 分流或 Go 客户端，运行时仍使用 ConfigMap；转换阶段仍扫描完整候选。

- [GoCanon 模板对齐](decisions/0016-gocanon-template-alignment.md)：本轮规则版本、重构边界、实施顺序与验证进度；不把参考示例覆盖到产品业务。

- [控制面 Namespace 初始化](control-plane-initialization.md)：通过 Ingate API 创建配置空间的清单与手动安装顺序，已提交为 `6979b0c5`，未执行真实安装；Controller 不自动创建 Namespace，不提前创建共享 ConfigMap。
- [API 网关完成度与 AI 网关开发条件](api-gateway-readiness.md)：2026-10-10 的源码与验证快照。
- [HTTP 主链路隔离验证方案](design/http-gateway-validation-plan.md)：留待后续统一测试的实库、Manager/Envoy 转发和 ACK/NACK 清单；不作为当前开发前置条件。
- [统一 Go AI Proxy 与配置下发](decisions/0010-unified-ai-proxy-config-delivery.md)：已确认运行组件、配置下发和资源关系，及待评审契约与实施建议。
- [AI Proxy 应用分层与能力包布局](decisions/0015-ai-proxy-package-layout.md)：组件内 pkg 归属及协议能力职责；完整请求编排迁入 modelproxy.Service 的重构与 HTTP 装配已通过 review，并获准本地提交。
- [AI 网关上游设计对照](design/ai-gateway-upstream-alignment.md)：固定版本的契约、源码、反例测试，以及采用、裁剪和必要差异。
- [AI 网关资源契约设计稿](design/ai-gateway-resource-contract.md)：六种主资源、策略附着建议、内部模型路由表、引用/冲突/删除语义与完整替换计划；HTTP 子集设计已通过，用户已授权提交阶段 B 当前实现，后续 AI/MCP 契约仍需确认。
- [AI 模型操作与数据面部署设计](design/ai-model-operations-and-deployment.md)：启动配置已提交为 `0b0eea18`；AI Proxy 命令、Run 与有界退出已通过 review 并获准本地提交，尚未进行真实 Envoy/模型联调。
- [AI 凭据与运行配置设计](design/ai-credentials-and-runtime-config.md)：Secret 引用与轮转/撤销草案；凭据部分待 review，按整份版本准入与双版本发布已标为未采用的历史候选。
- [AI 编译配置契约](design/ai-compiled-config-contract.md)：编译契约已确认，协调接线已提交为 `fcc9650b`，原生 Factory 加载器已提交为 `5bf59e98`；AI/MCP 引用说明已对齐同 Namespace 契约并通过 review，当前模型请求和运行装配进度见模型操作与部署设计。
- [架构与流程图](diagrams/ai-gateway.html)：当前架构、目标资源关系、目标架构、请求流程、配置发布流程；可离线打开。
- [HTTP 资源替换实现记录](decisions/0011-http-resource-contract.md)：阶段 B 范围、验证与未覆盖项。
- [Namespace 与同空间引用](decisions/0013-namespaced-resources.md)：编译身份、Namespace 存储、OpenAPI、原生 HTTP API、本地 NamespaceLifecycle 和删除控制器已提交；Backend 默认存储筛选重构已提交；六种资源的存储、管理 API 与查询投影已一起切换 Namespace，并经用户 review 提交为 `cda0478e`。
- [重构审查记录](refactoring-review.md)：此前重构的验证结果与保留缺口，不代表当前端到端验收。
- [原生核心资源直接复用](decisions/0014-upstream-core-resources.md)：参考 OneX 接入 Kubernetes Namespace/ConfigMap 类型与存储；实现已通过用户 review 并获准本地提交，不扩展原生 API 集合。

## 已有决策

| 决策 | 主题 |
| --- | --- |
| [0001](decisions/0001-gateway-first-slice.md) | Gateway、Route、Upstream 首个功能切片 |
| [0002](decisions/0002-shared-data-plane-controller.md) | 共享数据面汇总、裁决与 xDS 发布 |
| [0003](decisions/0003-generated-clients.md) | Kubernetes 生成客户端 |
| [0004](decisions/0004-admin-authentication.md) | 管理端账户和 Session |
| [0005](decisions/0005-admin-resource-management.md) | 管理端资源操作与异步查询投影 |
| [0006](decisions/0006-mysql-client-lifecycle.md) | MySQL 客户端生命周期 |
| [0007](decisions/0007-redis-client-lifecycle.md) | Redis UniversalClient |
| [0008](decisions/0008-build-version.md) | 构建与版本 |
| [0009](decisions/0009-logging-initialization.md) | 日志初始化 |
| [0010](decisions/0010-unified-ai-proxy-config-delivery.md) | 统一 AI Proxy、配置下发与目标资源关系 |
| [0011](decisions/0011-http-resource-contract.md) | HTTPRoute/Backend 替换及标准行为子集 |
| [0012](decisions/0012-resource-names-and-query-schema.md) | 资源名称、搜索和查询模型修正 |
| [0013](decisions/0013-namespaced-resources.md) | 显式 Namespace、同空间引用与共享数据面 |
| [0014](decisions/0014-upstream-core-resources.md) | 原生 Namespace/ConfigMap 的上游直接复用 |

各文档保留自己的状态；加入索引不表示草案或历史待评审项已经获得确认。
