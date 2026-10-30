# Ingate 技术文档

当前实现的阅读顺序是架构、资源、配置、请求与启动。设计决定、实现范围和验证证据分别阅读，
历史记录不作为当前端到端验收结果。

## 当前架构与运行契约

| 文档 | 内容 |
| --- | --- |
| [架构与流程图](diagrams/ai-gateway.html) | 当前配置流、资源关系、请求链和 Delta 同步，可离线打开 |
| [控制面增量编译与配置分发](design/control-plane-incremental-design.md) | krt 依赖图、转换缓存、两路 Delta ADS，以及仍需完整遍历的步骤 |
| [网关资源契约](design/ai-gateway-resource-contract.md) | 六种资源、Namespace、同空间引用、匹配与后端连接语义 |
| [AI 编译配置契约](design/ai-compiled-config-contract.md) | 独立 protobuf 资源、命名、版本、批次应用、删除和重连 |
| [AI Proxy 操作与部署](design/ai-model-operations-and-deployment.md) | 两个模型创建接口、MCP、UDS、错误、SSE、TLS 与未实现范围 |
| [控制面与数据面启动](control-plane-initialization.md) | 组件依赖、参数、实例标识、共享数据面和就绪边界 |
| [本地开发运行](development.md) | Compose 依赖、浏览器登录、Envoy/AI Proxy 数据面、一键启动与单组件 IDE 接管 |
| [AI 网关上游设计对照](design/ai-gateway-upstream-alignment.md) | 已核对的上游版本、采用范围和明确差异 |
| [AI Proxy 包布局](decisions/0015-ai-proxy-package-layout.md) | 应用层与能力包的所有权、依赖方向和资源清理 |

实现入口为 [Gateway Controller](../internal/controllermanager/controller/gateway/controller.go)、
[Envoy 适配器](../internal/controllermanager/dataplane/envoy/publisher.go)、
[共享 ADS 发布器](../internal/controllermanager/dataplane/xds/publisher.go)和
[AI Proxy Loader](../internal/aiproxy/pkg/aiconfig/loader.go)。

当前不再通过 ConfigMap 下发 AI 配置；原生 ConfigMap API 继续保留。
Envoy 与 AI Proxy 分别确认资源，不提供跨进程原子切换。
本轮真实组件验证范围及修复见 [2026-10-29 本地功能验证](functional-testing-2026-10-29.md)，
其中已通过项不替代尚未覆盖的部署和容量验收。

## 尚未实施与验证计划

- [AI 凭据与运行配置草案](design/ai-credentials-and-runtime-config.md)：Secret、权限、轮转与撤销尚未实施；不作为当前启动前提。
- [HTTP 主链路隔离验证方案](design/http-gateway-validation-plan.md)：验证计划；本轮已执行范围见上述功能记录，未覆盖项继续保留。
- [重构审查记录](refactoring-review.md)：各轮范围、修复和实际检查；带日期的历史结果不推定为当前结果。
- [API 网关完成度快照](api-gateway-readiness.md)：2026-10-10 的历史评估，不描述当前 AI/MCP 完成度。
- [图表维护](diagrams/README.md)：维护源、渲染方式与图形验证边界。

## 设计决策

| 决策 | 主题 |
| --- | --- |
| [0001](decisions/0001-gateway-first-slice.md) | 首个 Gateway、Route、Upstream 切片，资源名称已被后续契约替代 |
| [0002](decisions/0002-shared-data-plane-controller.md) | 共享数据面的协调与冲突裁决；全量计算和发布已演进为增量方案 |
| [0003](decisions/0003-generated-clients.md) | Kubernetes 生成客户端 |
| [0004](decisions/0004-admin-authentication.md) | 管理端账户和 Session |
| [0005](decisions/0005-admin-resource-management.md) | 管理端资源操作与异步查询投影 |
| [0006](decisions/0006-mysql-client-lifecycle.md) | MySQL 客户端生命周期 |
| [0007](decisions/0007-redis-client-lifecycle.md) | Redis UniversalClient |
| [0008](decisions/0008-build-version.md) | 构建与版本 |
| [0009](decisions/0009-logging-initialization.md) | 日志初始化 |
| [0010](decisions/0010-unified-ai-proxy-config-delivery.md) | 统一 Go AI Proxy；API Server 下发部分已由 Delta ADS 替代 |
| [0011](decisions/0011-http-resource-contract.md) | HTTPRoute/Backend 替换及标准行为子集 |
| [0012](decisions/0012-resource-names-and-query-schema.md) | 资源名称、搜索和查询模型 |
| [0013](decisions/0013-namespaced-resources.md) | 显式 Namespace、同空间引用与共享数据面 |
| [0014](decisions/0014-upstream-core-resources.md) | 直接复用原生 Namespace/ConfigMap |
| [0015](decisions/0015-ai-proxy-package-layout.md) | AI Proxy 应用分层与能力包 |
| [0016](decisions/0016-gocanon-template-alignment.md) | GoCanon 对齐与既有验证记录 |
| [0017](decisions/0017-client-startup-probes.md) | 客户端启动探测 |

开发和代码评审以 [AGENTS.md](../AGENTS.md) 为入口。
