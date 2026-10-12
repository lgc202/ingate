# AI 网关上游设计对照：采用、裁剪与必要差异

- 状态：六种资源关系、HTTP 子集及 2026-10-12 模型/MCP 基础字段已确认；AIBackend、ModelRoute 声明已提交，MCPRoute 声明与存储、AIBackend 管理与投影待用户 review；不授权新服务、安装组件或新增依赖
- 最近核对日期：2026-10-12；通用 HTTP/TLS 与 MCP 运行行为的此前核对日期为 2026-10-10
- 当前 Ingate 基线：`3519b189`（AIBackend、ModelRoute）；HTTP 替换前基线为 `56205b84321c60b9e6e73d2b9f2d5664575ea275`
- 对应设计：[资源契约与替换计划](ai-gateway-resource-contract.md)

## 结论与来源选择

已确认的六种资源关系不变。字段及通用行为优先采用现有契约；只有统一 Go 运行方式、现有平台作用域和消费者边界确实需要适配时，才保留项目差异。与标准不同的行为必须在实现前确认，不靠「新项目」或参考 YAML 自行批准。

本轮以 Gateway API、Agent Router、Envoy Gateway 为主要依据，三者分别负责通用附着/匹配、模型/MCP 服务语义、网络目标和 TLS。选择这一组合，是因为 Agent Router 的固定版本实际依赖后两者，可以核对连续实现链；不把多个品牌的配置字段拼成一套自有网关概念。此前查阅的 Kong、APISIX 仅作为旁证，不决定本稿字段。

| 来源 | 固定版本/提交 | 本轮使用范围 |
| --- | --- | --- |
| Gateway API | v1.6.1，`8bb74df00e56ec8f944d48c25e6c1c9c2f6848e3` | HTTPRoute、ParentReference、Conditions 及失效后端 conformance 测试定义 |
| Agent Router | v1.2.0，`daa9f891a8afcb18576d4593ad18870d1d18abac` | v1beta1 资源、模型 Controller、配置编译、引用撤销测试、MCP 请求主入口与发布说明 |
| Envoy Gateway | v1.9.2，`f9f92a112a4033abed5c8d43a5f7e37da257403c` | Backend 内联 TLS、HTTP 翻译器及缺少自定义后端的测试预期输出 |
| MCP | 2026-07-28 协议文档；Go SDK v1.7.0 标签 | 协议变化、SDK 版本定义；不据此选定 Ingate 首个运行版本 |

GitHub 标签的提交已只读解析；Envoy Gateway 使用 annotated tag，表中记录的是剥离后的 commit，不是 tag 对象。Agent Router go.mod 与[发布说明](https://github.com/theagentrouter/agent-router/releases/tag/v1.2.0)记录 Gateway API v1.6.1、Envoy Gateway v1.9.2、MCP Go SDK v1.7.0。这里是研究基线，不是给 Ingate 添加这些依赖的决定，也不是 Envoy 可执行程序兼容性证明。

## 通用路由：采用标准语义，不增加同义规则

| 事项 | 上游证据 | 修订后的建议与影响 |
| --- | --- | --- |
| sectionName | [ParentReference](https://github.com/kubernetes-sigs/gateway-api/blob/8bb74df00e56ec8f944d48c25e6c1c9c2f6848e3/apis/v1/shared_types.go#L92)允许省略，引用整个 parent；至少一个合格 Listener 接纳即附着成功 | 撤掉必填要求。省略时后续新增 Listener 可能接纳，管理端显示有效附着范围 |
| 规则 name | [HTTPRouteRule](https://github.com/kubernetes-sigs/gateway-api/blob/8bb74df00e56ec8f944d48c25e6c1c9c2f6848e3/apis/v1/httproute_types.go#L148)可选 | 撤掉强制名字及按规则名排序；未命名规则使用内部列表索引，不产生公共规则 ID |
| 缺省 matches/path | [匹配定义](https://github.com/kubernetes-sigs/gateway-api/blob/8bb74df00e56ec8f944d48c25e6c1c9c2f6848e3/apis/v1/httproute_types.go#L154)采用 PathPrefix `/` | 撤掉「无 matches 拒绝」。这是扩大匹配范围的显式新契约，不能在界面中隐藏有效默认值 |
| 匹配优先级 | [优先级定义](https://github.com/kubernetes-sigs/gateway-api/blob/8bb74df00e56ec8f944d48c25e6c1c9c2f6848e3/apis/v1/httproute_types.go#L186)先具体匹配，再跨资源创建时间/namespace-name，同资源最终按第一条规则 | 已实现路径条件采用 Exact、最长路径段前缀；集群级资源省略 namespace 比较。相同 HTTP match 保留候选而非整体删除资源 |
| 引用失效 | [HTTPBackendRef 语义](https://github.com/kubernetes-sigs/gateway-api/blob/8bb74df00e56ec8f944d48c25e6c1c9c2f6848e3/apis/v1/httproute_types.go#L269)要求命中无效目标返回错误，不自动把其流量分配给有效目标 | 保留匹配，生成错误动作；不能删除规则后落到 `/` 兜底。当前 Ingate 排除缺少引用 Route 的行为会改变 |
| 路由状态 | [RouteParentStatus](https://github.com/kubernetes-sigs/gateway-api/blob/8bb74df00e56ec8f944d48c25e6c1c9c2f6848e3/apis/v1/shared_types.go#L481)及标准 Conditions | 采用 parentRef/controllerName/conditions；Accepted 与 ResolvedRefs 分开，不自造逐规则 status 协议或 BackendMissing 同义 Reason |

Header、Query、Method、filters、跨 namespace 引用等未批准能力不能因上游类型存在就自动开放。需要这些能力时采用对应原义并补反例，不创建任意条件字典。自有 Group、cluster scope、单目标切片和必需 parentRefs 意味着这是明确子集，不是 Gateway API 全量一致性实现。

### 同一条失效规则的三层证据

标准、翻译器和反例必须能对应同一个行为，不能只找到一个同名字段。

1. Gateway API [HTTPRouteInvalidNonExistentBackendRef](https://github.com/kubernetes-sigs/gateway-api/blob/8bb74df00e56ec8f944d48c25e6c1c9c2f6848e3/conformance/tests/httproute-invalid-nonexistent-backendref.go#L36)要求 Route 被接纳，ResolvedRefs=False/BackendNotFound，并通过 HTTP 请求断言 500。说明「接纳」不是「目标可用」。
2. Envoy Gateway [processHTTPRouteRules](https://github.com/envoyproxy/gateway/blob/f9f92a112a4033abed5c8d43a5f7e37da257403c/internal/gatewayapi/route.go#L379)在没有有效 destination 时，把已有匹配的 IR route 改为 500 directResponse，而不是删除匹配。
3. [缺少自定义 Backend 的预期输出](https://github.com/envoyproxy/gateway/blob/f9f92a112a4033abed5c8d43a5f7e37da257403c/internal/gatewayapi/testdata/extensions/httproute-with-custom-backend-not-found.out.yaml#L72)同时包含 Accepted=True、ResolvedRefs=False 和保留路径的 500 动作。它将网络后端扩展与通用错误语义连接起来。

Ingate 应增加「更具体的失效路由 + 有效 `/` 兜底」反例：请求必须失败，不能到兜底后端；补齐引用后再恢复。未来若批准多目标，还必须验证无效目标的权重不会被偷偷归一化给有效目标。连接拒绝、超时、无就绪端点与配置引用错误分开处理，不能一律叫 BackendNotFound。

以上核对了源码与测试定义，未执行上游 conformance 或真实 Envoy 测试。

## 模型服务：沿用 schema、覆盖字段与策略边界

Agent Router [AIServiceBackendSpec](https://github.com/theagentrouter/agent-router/blob/daa9f891a8afcb18576d4593ad18870d1d18abac/api/v1beta1/ai_service_backend.go#L54)拥有 schema 与网络 backendRef；[VersionedAPISchema](https://github.com/theagentrouter/agent-router/blob/daa9f891a8afcb18576d4593ad18870d1d18abac/api/v1beta1/shared_types.go#L12)表达 name/version/prefix。模型绑定的 [modelNameOverride](https://github.com/theagentrouter/agent-router/blob/daa9f891a8afcb18576d4593ad18870d1d18abac/api/v1beta1/ai_gateway_route.go#L364)可选；[Gateway 配置编译](https://github.com/theagentrouter/agent-router/blob/daa9f891a8afcb18576d4593ad18870d1d18abac/internal/controller/gateway.go#L481)把它写入后端运行配置。这不是网络 Backend 的模型属性。

因此本稿采用 `schema.name: OpenAI`、`schema.prefix`、`modelNameOverride`，撤掉 OpenAIChatCompletions、独立 basePath 和必需上游 model 字段。操作能力与 schema 分开声明：第一切片即使只实现 Chat Completions，也不能重新发明 schema 枚举或宣称所有 OpenAI 操作都支持。省略 override 时保留请求模型名。

模型认证参考 [BackendSecurityPolicySpec](https://github.com/theagentrouter/agent-router/blob/daa9f891a8afcb18576d4593ad18870d1d18abac/api/v1beta1/backendsecurity_policy.go#L47)的 targetRefs 附着，凭据由受保护引用获取。Ingate 的建议目标是已确认的 AIBackend；这是名称/目标适配，不是新策略框架。认证权限、轮转及撤销有独立生命周期，建议独立资源，而不是写死进模型 API schema。新增策略 API 与凭据能力仍须 review。

### 2026-10-12 重新核对：资源定义必须连到实际消费者

本次只读取得表中固定提交的类型、Controller、ExtProc 入口与示例，未切换到 main，未运行上游组件。旧稿的模型字段并非官方原样契约；用户于 2026-10-12 确认基础字段修订，运行入口编译和安全边界仍另行 review。

| 关注点 | 本次读取的官方证据 | Ingate 修订建议与行为影响 |
| --- | --- | --- |
| route 级 path/schema | [AIGatewayRouteSpec](https://github.com/theagentrouter/agent-router/blob/daa9f891a8afcb18576d4593ad18870d1d18abac/api/v1beta1/ai_gateway_route.go#L61)与 [OpenAI 示例](https://github.com/theagentrouter/agent-router/blob/daa9f891a8afcb18576d4593ad18870d1d18abac/examples/basic/openai.yaml)均无这两个字段 | 撤掉 ModelRoute.path/schema；同一组模型规则适用于已开放操作，不再复制每操作资源 |
| 客户端格式选择 | [操作注册](https://github.com/theagentrouter/agent-router/blob/daa9f891a8afcb18576d4593ad18870d1d18abac/cmd/extproc/mainlib/main.go#L328) + [processorForPath](https://github.com/theagentrouter/agent-router/blob/daa9f891a8afcb18576d4593ad18870d1d18abac/internal/extproc/server.go#L98)按标准路径选择处理器 | 操作属于运行实现范围；不新增 operation 字段，也不因资源存在就开放全部接口 |
| 模型选择 | [AIGatewayRouteRuleMatch](https://github.com/theagentrouter/agent-router/blob/daa9f891a8afcb18576d4593ad18870d1d18abac/api/v1beta1/ai_gateway_route.go#L424)使用 headers，模型键为 `x-ai-eg-model`；[ProcessRequestBody](https://github.com/theagentrouter/agent-router/blob/daa9f891a8afcb18576d4593ad18870d1d18abac/internal/extproc/processor_impl.go#L287)从请求内容赋值并生成 Header mutation | 撤掉 matches.model，沿用官方 headers 形状和提取后匹配的语义；Go 可直接匹配提取值，不能信任客户端伪造 Header |
| 空 matches | 官方 matches 可省略，派生 [HTTPRoute](https://github.com/theagentrouter/agent-router/blob/daa9f891a8afcb18576d4593ad18870d1d18abac/internal/controller/ai_gateway_route.go#L341)沿用 HTTPRoute 的缺省匹配 | 撤掉强制每规则填写模型的额外限制；空值为 catch-all，管理端须明确显示「所有模型」；未知模型未必由网关拒绝 |
| 上游 prefix 默认值 | [schemaToFilterAPI](https://github.com/theagentrouter/agent-router/blob/daa9f891a8afcb18576d4593ad18870d1d18abac/internal/controller/gateway.go#L175)对 OpenAI 用 `cmp.Or(prefix, "v1")` | 省略及空字符串的有效前缀为 `/v1`，不能解释成根路径。schema 类型注释与实际编译行为有差异，以消费者为依据 |
| schema.version | [VersionedAPISchema](https://github.com/theagentrouter/agent-router/blob/daa9f891a8afcb18576d4593ad18870d1d18abac/api/v1beta1/shared_types.go#L18)明确 OpenAI 忽略 version；实际编译也未复制该值 | 首版只有 OpenAI，建议不开放无效 version 字段；增加确实需要版本的提供商时采用上游原义 |
| 模型入口生成 | [newHTTPRoute](https://github.com/theagentrouter/agent-router/blob/daa9f891a8afcb18576d4593ad18870d1d18abac/internal/controller/ai_gateway_route.go#L341)使用 Controller 的 rootPrefix；[启动参数](https://github.com/theagentrouter/agent-router/blob/daa9f891a8afcb18576d4593ad18870d1d18abac/cmd/controller/main.go#L235)默认 `/` | 不复制官方覆盖根前缀的网络入口；统一 Go Proxy 的已批准操作拟生成精确入口，避免接管普通 HTTP/MCP。这是运行编译适配，不是新增资源字段 |

上述依据不能由过时注释替代。AIServiceBackendSpec 的 APISchema 注释仍提及 AIGatewayRouteSpec.APISchema，但该版本实际 route 定义不存在该字段；shared_types 的省略 prefix 描述也不能替代 Controller 的默认处理。

名称变化、cluster scope、displayName、单目标、只开放模型 Exact 匹配以及标准 parent Conditions，仍是明确的 Ingate 子集。沿用结构不等于完整实现官方 Header 路由、模型发现、InferencePool、多目标重试、转换或计费。后台生成 name、重复 displayName 和必要时显示资源 ID 沿用本项目已确认契约，不重新发明身份系统。

### 不照搬实现中的每一处行为

官方 [newHTTPRoute](https://github.com/theagentrouter/agent-router/blob/daa9f891a8afcb18576d4593ad18870d1d18abac/internal/controller/ai_gateway_route.go#L249)会跳过无法取得或未经授权的 AIServiceBackend 引用，继续构建其余配置并汇总错误；[撤销 ReferenceGrant 的测试](https://github.com/theagentrouter/agent-router/blob/daa9f891a8afcb18576d4593ad18870d1d18abac/internal/controller/ai_gateway_route_test.go#L1060)断言派生 HTTPRoute 清掉该目标，并通知 Gateway 重建配置，不能只改 status 却继续使用旧账号。

这证明必须清除失效/未授权的转发目标，不证明可以删掉匹配、继承其他账号，或为无效目标重新分配全部流量。官方模型层也没有原样采用 Gateway API 的 parent Conditions。本稿的通用路由状态优先采用 Gateway API，模型/MCP 错误动作按各协议定稿，差异明确记录，不将官方当前实现全称为标准。

## 网络 TLS：内联有上游依据，不再套自有 transport

Envoy Gateway v1.9.2 的 [BackendSpec](https://github.com/envoyproxy/gateway/blob/f9f92a112a4033abed5c8d43a5f7e37da257403c/api/v1alpha1/backend_types.go#L137)已提供 `tls`；[BackendTLSSettings](https://github.com/envoyproxy/gateway/blob/f9f92a112a4033abed5c8d43a5f7e37da257403c/api/v1alpha1/backend_types.go#L175)提供信任根、SNI 与自动派生等配置。稳定 BackendTLSPolicy 是另一个已有附着方案；不能因为官方 basic 示例使用策略，就断言内联 TLS 是自造。

本稿拟使用内联 `tls.sni` 与 `tls.wellKnownCACertificates: System` 子集，不增加 `transport.protocol/authority/tls.trust` 包装。暂不同时开放独立 TLS 策略与合并覆盖，不开放 skip-verify、私有 CA 或 mTLS。显式 SNI 与相同 DNS 身份证书校验属于本项目限制，较上游更窄；它避免从任意客户端 Host 选择上游身份，应在 TLS review 中验证必要性。

HTTP Host 重写独立于 TLS。Agent Router [newHTTPRoute](https://github.com/theagentrouter/agent-router/blob/daa9f891a8afcb18576d4593ad18870d1d18abac/internal/controller/ai_gateway_route.go#L255)引用自动 hostname rewrite filter，说明需要由实际出口配置处理；不强制用户填一个同义网络 authority 字段。多端点是否共用证书身份、Host 如何生成，必须在首个实际服务中检验。

Gateway API [XBackend](https://gateway-api.sigs.k8s.io/reference/api-types/backend/)仍属 Experimental，主要表达外部 hostname，不作为已稳定统一网络池的证明。Ingate 保留已有 IP/DNS/端口/正端点权重，是现有网络池能力，而非宣称逐字段等同 Envoy Gateway 的端点结构。原稿额外的 DNS/IP 规范化和 connectTimeout API 不作为替换条件。

## MCP：复用协议实现，不把源码分支当端到端能力

Agent Router [MCPRouteSpec](https://github.com/theagentrouter/agent-router/blob/daa9f891a8afcb18576d4593ad18870d1d18abac/api/v1beta1/mcp_route.go#L71)直接引用网络目标；[backendRefs](https://github.com/theagentrouter/agent-router/blob/daa9f891a8afcb18576d4593ad18870d1d18abac/api/v1beta1/mcp_route.go#L182)携带 path 与 securityPolicy。入口/上游路径均默认 `/mcp`。本稿沿用这些名称及附着边界，不再自造 authentication 包装或每路由 protocol.versions；首个切片拟限定远程单后端 Streamable HTTP，仍须 review。

本次重读了这些类型及 [MCP 实例配置](https://github.com/theagentrouter/agent-router/blob/daa9f891a8afcb18576d4593ad18870d1d18abac/examples/mcp/mcp_example.yaml)：path 是可选指针，两个路径默认值及 1024 长度限制一致。建议基础资源只接入 parentRefs、path、单个 Backend 引用及其 path；认证未定时不创建空 securityPolicy。拒绝显式空 path 是项目的路径有效性子集，不能冒充官方已写出的字段约束。协议版本、会话与 SDK 行为本次没有重新验证，不影响先做非敏感资源 CRUD。

官方此版本主要提供多后端聚合；Ingate 的单后端、不重命名工具、不引入共享会话存储是范围裁剪，不是官方聚合器等价实现。完整方法、通知、请求取消和合法流式交互仍必须通过选定版本验证，不能拿 tools/call 成功代替。

有一个容易误读的发布边界：SDK v1.7.0 的 [协议版本定义](https://github.com/modelcontextprotocol/go-sdk/blob/v1.7.0/mcp/shared.go#L44)包含 2026-07-28，Agent Router 也含 modern 处理代码，但固定版本 [servePOST](https://github.com/theagentrouter/agent-router/blob/daa9f891a8afcb18576d4593ad18870d1d18abac/internal/mcpproxy/handlers.go#L65)仍保留 era 分派 TODO，只调用 serveLegacyPOST。[发布说明](https://github.com/theagentrouter/agent-router/releases/tag/v1.2.0)也明确 modern 端到端支持尚未启用。因此不能把最新协议直接填进示例便声称可互操作。

首个协议版本取决于实际目标客户端/后端。优先使用官方 Go MCP SDK 的 JSON-RPC 与传输能力，但 SDK 并不替我们完成网关的可信身份、网络出口和配置切换。没有完整聚合需求时不复制官方 fan-out、工具前缀、会话加密和 Redis 存储；将来确有需求再按该版本契约设计。

## 需要保留的差异及其确认条件

| 项目差异 | 必要性/代价 | 是否已经确认 |
| --- | --- | --- |
| 一个 Go AI Proxy，模型与 MCP 模块分开 | 用户选择统一运行/配置生命周期；模型不再原样使用官方 ExtProc，需要验证新增 HTTP 转发的流、取消和背压 | 方向已确认，出口拓扑与运行参数未确认 |
| API Server 下发编译结果 | 已有 mini API 机制，没有 kubelet 卷投射；复用 client-go List/Watch，不实现宿主编排替代品 | 下发方向已确认，运行时对象/权限/激活未确认 |
| ModelRoute、AIBackend 名称 | 用户已确认职责关系；对应官方 AIGatewayRoute、AIServiceBackend，不增加公共模型实体 | 已确认 |
| 自有 Group、cluster scope | 沿用现有管理边界；没有 Namespace/租户契约。不具备租户自治隔离，未来增加该能力前必须重审 | 阶段 B 的 HTTP 资源已确认并实现；未来租户/作用域演进仍须重审 |
| ModelRoute 沿用 headers 匹配，无 route 级 path/schema | 统一 Go 运行不构成更改资源字段的必要理由；保留请求内容提取与匹配原义 | 本次基础字段已确认，替代旧稿的 matches.model/单操作路由建议 |
| 精确操作入口与跨 HTTP/模型/MCP 入口裁决 | 普通 HTTP 仍直达 Envoy，不能机械复制官方 `/` 根前缀；冲突行为属于运行编译边界 | 另行 review；本次不发布入口或预建路由编排框架 |
| 保留当前网络池、Admin HTTP/gRPC 与 MySQL 投影 | 属于现有功能及消费者，不因采用上游术语删除 | 阶段 B 当前实现及 00002 基线修正已获用户授权提交；未执行实库迁移 |

以上差异不能仅以「我们更灵活」为依据。若无需差异就能满足消费者，应选上游原义；发现新共享需求后再提炼公共抽象，不在第一切片预建 Provider/Processor 插件、策略 DSL、通用实体表、空接口或新 Watch 协议。

## 实现复用与验证边界

基础 API 生命周期沿用 Kubernetes metadata、Strategy、标准错误、生成工具与现有 client-go；xDS 使用现有 go-control-plane。协议能力优先评估官方 MCP SDK、提供商 SDK 和 Agent Router 已有转换实现。实际采用包、版本或移植代码前，核对公开接口、许可证、internal 包限制、运行模式及依赖影响；没有可复用接口时才实现小范围适配，不直接复制整套 Controller 或升级全部依赖。

此前上游对照阶段仅修改文档。已核对固定版本资源契约、关键协调/翻译路径和指定反例测试定义，不代表审查了上游全部代码，也未运行上游测试、SDK 互操作、真实 Envoy/提供商/MCP/凭据/实库验证。纯文档检查不替代这些证据。上表中的自有 Group、cluster scope 及 HTTP 管理/投影替换已随阶段 B 确认；模型、MCP、安全和跨协议裁决仍保留各自待确认边界。

[契约稿](ai-gateway-resource-contract.md)的六种资源关系、HTTP 子集及本次模型/MCP 基础字段已通过 review，HTTP 实现及验证见 [0011](../decisions/0011-http-resource-contract.md)。AIBackend、ModelRoute 声明已提交为 `3519b189`；MCPRoute 声明与存储、AIBackend Admin/查询投影已实现，待用户 review。后续按已确认计划继续 ModelRoute、MCPRoute 管理与投影，不在此开发新服务。MCPRoute 采用已确认的单 Backend、可选路径及默认值子集；这不表示已实现官方聚合器或协议转发。每阶段审查全部手写改动；提交前仍等用户 review。
