# Ingate 技术文档

阅读顺序：先看当前能力和主链路，再看 AI 网关的已确认方向与待评审设计。

- [API 网关完成度与 AI 网关开发条件](api-gateway-readiness.md)：2026-10-10 的源码与验证快照。
- [HTTP 主链路隔离验证方案](design/http-gateway-validation-plan.md)：留待后续统一测试的实库、Manager/Envoy 转发和 ACK/NACK 清单；不作为当前开发前置条件。
- [统一 Go AI Proxy 与配置下发](decisions/0010-unified-ai-proxy-config-delivery.md)：已确认运行组件、配置下发和资源关系，及待评审契约与实施建议。
- [AI 网关上游设计对照](design/ai-gateway-upstream-alignment.md)：固定版本的契约、源码、反例测试，以及采用、裁剪和必要差异。
- [AI 网关资源契约设计稿](design/ai-gateway-resource-contract.md)：六种主资源、策略附着建议、内部模型路由表、引用/冲突/删除语义与完整替换计划；HTTP 子集设计已通过，用户已授权提交阶段 B 当前实现，后续 AI/MCP 契约仍需确认。
- [AI 模型操作与数据面部署设计](design/ai-model-operations-and-deployment.md)：参考官方的首批 OpenAI 操作与共部署建议；待 review，不代表已实现或完整 API 兼容。
- [AI 凭据与运行配置设计](design/ai-credentials-and-runtime-config.md)：Secret 引用、派生配置、版本绑定、本地就绪及轮转/撤销建议；待 review，包含集中托管的基础设施代价和未定契约。
- [架构与流程图](diagrams/ai-gateway.html)：当前架构、目标资源关系、目标架构、请求流程、配置发布流程；可离线打开。
- [HTTP 资源替换实现记录](decisions/0011-http-resource-contract.md)：阶段 B 范围、验证与未覆盖项。
- [Namespace 与同空间引用](decisions/0013-namespaced-resources.md)：编译身份、Namespace 存储、OpenAPI、原生 HTTP API 和本地 NamespaceLifecycle 已提交；本轮接入删除内核、Manager Watch 与重试调度，待 review；业务存储与管理链路仍待切换。
- [重构审查记录](refactoring-review.md)：此前重构的验证结果与保留缺口，不代表当前端到端验收。

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

各文档保留自己的状态；加入索引不表示草案或历史待评审项已经获得确认。
