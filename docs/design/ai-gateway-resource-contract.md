# 三类路由复用 Backend，以 AIBackend 定义模型服务访问契约

- 状态：六种资源、Namespace、管理与投影、模型/MCP 代理、上游 TLS 子集及增量分发已实现；真实组件验收尚未完成。
- 作者：Ingate 开发协作
- 更新日期：2026-10-20
- 讨论入口：本仓库文档 review，依据 [0010 决策](../decisions/0010-unified-ai-proxy-config-delivery.md)及用户确认的资源关系
- 范围：当前资源契约、裁决边界、管理接口和运行限制；安全策略建议单独标注，不作为已实现能力。

## 摘要

我们按已确认关系设计 Gateway、HTTPRoute、ModelRoute、AIBackend、MCPRoute、Backend。三种路由通过 `parentRefs` 绑定 Gateway；HTTPRoute 和 MCPRoute 直接引用 Backend，ModelRoute 引用 AIBackend，AIBackend 再引用 Backend。Backend 管理网络目标，AIBackend 管理模型服务契约，ModelRoute 管理模型入口与目标映射。

项目没有旧方案兼容压力，Route、Upstream 已替换为 HTTPRoute、Backend，不保留同义的兼容 API。普通 HTTP 路径匹配、端点权重、资源版本并发控制、异步管理查询继续保留。模型/MCP 的资源、管理、编译与代理已接通；共用网络层支持上游 TLS，配置通过资源级 Delta ADS 下发。

资源字段参考 Agent Router v1.2.0 固定提交的类型、Controller、请求入口注册和实际 YAML，具体依据见[上游对照](ai-gateway-upstream-alignment.md)。已采用下文基础字段，不保留旧稿的 `ModelRoute.path`、`ModelRoute.schema`、`matches.model`。统一 Go Proxy 不要求改变官方资源形状；凭据与安全策略仍待独立评审。

![已确认的资源引用关系](../diagrams/ai-resource-model.svg)

## 背景：现有契约能表达网络转发，不能表达模型服务

基线版本的 RouteSpec 只有 `gatewayName`、`listenerName`、`path`、`upstreamName`，UpstreamSpec 只有端点集合。它们没有客户端模型名、上游模型映射、协议 schema 或后端账号语义。阶段 B 已替换为 [HTTPRouteSpec](../../pkg/apis/gateway/v1alpha1/httproute.go)与 [BackendSpec](../../pkg/apis/gateway/v1alpha1/backend.go)；模型/MCP 协议层的当前范围见文末。

基线裁决按 Gateway/Listener 和规范化路径判断冲突；阶段 B 的 [HTTPRoute 裁决](../../internal/controllermanager/controller/gateway/httproute.go)已保留同路径候选，按父引用独立附着。[Envoy 适配层](../../internal/controllermanager/dataplane/envoy/resources.go)按 Exact 优先、长 PathPrefix 优先稳定排序，IP/DNS 后端采用 EDS/STRICT_DNS。后端连接超时仍为 5 秒；上游 TLS 的当前实现边界见文末，不能由资源改名推断其部署已验收。

当前 Admin 接口保留 [0005](../decisions/0005-admin-resource-management.md)的 API Server 写入、MySQL 异步查询分工；[资源存储映射](../../internal/apiserver/registry/gateway/storage.go)已有六种资源及其 status。D2 已接入 AIBackend、ModelRoute、MCPRoute 管理与查询投影，均已按用户指示提交，未推送。Namespace 原生 API 与生命周期已接入；六种业务资源的 Namespace 全链路切换已提交。派生 AI 配置通过 Delta ADS 下发，不再写入 ConfigMap；原生 ConfigMap API 保留，没有 Secret 或租约资源。本文不假设完整 Kubernetes 已存在。

本稿输入是已确认讨论和现有代码，没有另建 PRD，也没有新增计费、配额、Agent、工具聚合、插件市场或多租户业务需求。

## 设计：先固定资源身份和引用边界

### API 沿用项目自己的 Group，不冒充 Gateway API 实现

采用 `gateway.ingate.io/v1alpha1`，保留无版本内部类型、版本化外部类型、生成 Conversion/DeepCopy/OpenAPI/Clientset 的现有形式。六种主资源使用 Kubernetes metadata、spec、status 结构。

| Kind | API Server 资源名 | 当前管理端集合路径 |
| --- | --- | --- |
| Gateway | gateways | /api/admin/v1/namespaces/{namespace}/gateways |
| HTTPRoute | httproutes | /api/admin/v1/namespaces/{namespace}/http-routes |
| ModelRoute | modelroutes | /api/admin/v1/namespaces/{namespace}/model-routes |
| AIBackend | aibackends | /api/admin/v1/namespaces/{namespace}/ai-backends |
| MCPRoute | mcproutes | /api/admin/v1/namespaces/{namespace}/mcp-routes |
| Backend | backends | /api/admin/v1/namespaces/{namespace}/backends |

用户已确认改用 Namespace 作用域：管理请求显式指定空间，界面可默认选中一个空间，后端不固定空间；引用只在源对象的空间内解析。名称在同一 Kind、同一 Namespace 内唯一，不同空间可同名。管理路径为 `/api/admin/v1/namespaces/{namespace}/<resources>`，不保留旧 CRUD 兼容入口；上表对应当前已实现接口。具体范围和分步状态见 [0013](../decisions/0013-namespaced-resources.md)。Namespace 不是权限或网络隔离的替代品，本次不增加租户、ReferenceGrant 或跨空间授权。

资源名称沿用当前 DNS subdomain 规则；Listener 名称沿用 DNS label 规则。端点集合采用现有确定排序；路由规则列表保留声明顺序，因为它是 Gateway API 规定的最终同资源匹配裁决因素。更新仍使用原样的 `metadata.resourceVersion`，不把它解释成跨资源业务发布版本。

### parentRefs 沿用 Gateway 与 Listener 的附着语义

三种路由统一使用以下引用形状；下列是字段声明示意，不是 Go 类型或已安装 API：

```yaml
parentRefs:
  - name: public
    sectionName: api
```

`name` 引用 Gateway，必填；`sectionName` 可选，设置时引用其中的 Listener，省略时引用整个 Gateway，由其符合协议及附着要求的 Listener 接纳。至少一个 Listener 接纳即表示该 parent 附着成功，不表示所有 Listener 均成功。沿用 Gateway API 的语义，不再要求额外填写 sectionName 才能使用。

引用限定到同一 Namespace 的 Gateway，不接受任意 Group/Kind，不增加跨空间 namespace 字段或按端口绑定。Gateway 改 Listener 端口时，显式 Listener 引用仍稳定；删除或改 Listener 名称时，该绑定失效。省略 sectionName 会使后续新增的合格 Listener 也可能接纳该路由，管理端须展示此影响；需要固定入口时显式填写。

`parentRefs` 是非空集合，重复的 `(name, sectionName)` 在写入时拒绝。同一 Gateway 的多条引用必须都显式指定不同 Listener，不能同时声明整个 Gateway 和其某个 Listener；该约束沿用上游以避免重叠引用。拟允许一个路由绑定多个入口，各绑定独立接纳和报告状态。非空要求是本项目只管理有入口路由的范围限制；Gateway API HTTPRoute 本身允许无 parent 的对象，本文不宣称完整一致性。

### Gateway 继续只拥有入口配置

| 字段 | 当前行为 |
| --- | --- |
| spec.displayName | 可修改、允许重复的展示名称，不参与引用和匹配 |
| spec.listeners[].name | Gateway 内唯一，供 sectionName 引用 |
| spec.listeners[].protocol | 本次 HTTP 资源替换阶段只接受现有 HTTP；HTTPS 需完成证书与权限契约后接入 |
| spec.listeners[].port | 1..65535 |
| spec.listeners[].hostname | 保留空值或精确 DNS 名称；空值表示该端口任意 Host |

模型和 MCP 是 HTTP 之上的业务协议，不把 `Model` 或 `MCP` 添加为 Listener 网络协议。公网入站 TLS、AI 客户端认证和 Listener 的路由附着授权需要单独完成安全设计；当前管理员 Session 不承担这些职责。

Gateway、HTTPRoute、Backend 的 Admin 创建请求不接收技术名称；后台生成不可修改的 `metadata.name`，供引用、查询和必要时展示“资源 ID”。三种资源均提供可修改且允许重复的 `spec.displayName`；列表参数 `display_name` 使用包含匹配，不限制为前缀。原生 API 保留 name / generateName 与 metadata.uid，MySQL 不额外投影 UID。详见 [名称契约](../decisions/0012-resource-names-and-query-schema.md)。

新增三种资源沿用同一身份和展示名称契约：Admin 生成 name，用户编辑 displayName，引用仍使用 name。不新增 uid 字段、展示名称唯一约束或前缀索引字段；三种资源的 Admin 入口均已实现。

跨 Gateway 的 Listener 冲突仍在 [0002](../decisions/0002-shared-data-plane-controller.md)的共享数据面范围内裁决，包括不同 Namespace。按创建时间、Namespace、name 稳定排序；相同物理入口不会因资源分属不同空间而放行。暂不引入 wildcard Host、GatewayClass、部署编排或流量管理策略资源。

### Backend 管理网络目标，不管理模型账号

| 字段 | 当前行为与依据 |
| --- | --- |
| spec.endpoints[].address | IP 或 DNS 名称，不允许 URL、路径、查询和用户信息；保留现有端点能力 |
| spec.endpoints[].port | 明确端口，1..65535，不根据协议悄悄覆盖 |
| spec.endpoints[].weight | 正 int32，保留当前端点相对权重；不是模型路由分流权重 |
| spec.tls | 可选；设置即要求加密上游连接，省略保留当前明文 HTTP 行为。沿用 Envoy Gateway Backend 的内联 TLS 形状，不另造 transport 包装 |
| spec.tls.sni | 首个 TLS 切片要求显式固定 DNS 身份，用于 SNI，并以同一 DNS 身份验证证书；不从不可信客户端 Host 推导身份。这是明确标注的安全子集，上游允许省略或自动派生 |
| spec.tls.wellKnownCACertificates | 首个 TLS 切片仅支持 System，使用运行环境系统信任根；名称沿用上游，不另造 trust 枚举 |

端点集合必须非空，沿用当前地址/端口重复校验及确定排序。原稿额外提出的 IP 文本归一化、DNS 小写与根点改写不作为本次资源替换条件；它们不是新增协议能力所必需。若后续发现真实重复目标问题，再结合网络解析语义与 Admin/API Server 一致性测试评审，不借重构扩大校验范围。

一个 Backend 的端点是同一服务的可替换网络副本，TLS 身份和连接策略一致。不能把具有不同 API 契约、账号或证书身份的提供商地址随意塞进一个端点池。相同域名也不强制合并 Backend；需要连接、健康或安全隔离时使用不同 Backend 对象。

连接超时暂保留现有内部 5s，不为了参考项目有流量策略就新增 connectTimeout API。连接超时不约束整个模型响应或流空闲；后者随首个协议运行切片另行定稿。

HTTP Host/authority 与 TLS SNI 是不同语义，不能因为使用 TLS 就强制新增 authority 字段。普通 HTTP 保留当前 Host 转发；模型出口的 Host 重写参考 Agent Router 的自动 hostname rewrite，待出口拓扑确定后在编译/网络适配层验收，不凭客户端路径或 Host 决定上游身份。

访问 HTTPS 模型提供商必须先实现并验收 TLS。TLS 构建或验证失败不能降级为明文。用户已确认：完整更新主动省略或清空 tls 会恢复明文期望，这是显式配置修改，首版不禁止主动删除、不增加授权开关。连接或证书失败不改变期望，不重试明文；旧流撤销时限不由本切片承诺。首个切片不同时支持内联 TLS 与独立 BackendTLSPolicy，避免先引入尚无需求的合并优先级；私有 CA、mTLS、跳过证书校验和其他网络治理能力不随上游字段一并开放。

### HTTPRoute 保留现有普通转发功能

采用 `spec.parentRefs` 和 `spec.rules[]`。规则的 `name` 可选，填写时在资源内唯一；不以规则名决定匹配优先级。第一实现切片每个规则只接受一个 Backend 引用；集合形状用于明确规则与目标绑定，不意味着已支持多个目标或故障转移。未支持的输入拒绝，不静默取第一项。

```yaml
rules:
  - name: orders
    matches:
      - path:
          type: PathPrefix
          value: /orders
    backendRefs:
      - name: orders-service
```

`matches` 内是 OR；同一个 match 内各条件是 AND。首个切片只定义已有的 Exact/PathPrefix 路径条件，不加入 Header、Query、正则、重定向或任意 filters。路径必须为绝对路径且不含 query、fragment 或连续斜杠；PathPrefix 去尾斜杠，根路径除外，按路径段匹配 `/api` 而非 `/apix`。Exact 保留尾斜杠。

`matches` 省略或为空时，按 Gateway API 解释为 PathPrefix `/`；match 中省略 path 也采用该默认值。管理端必须显示这个有效匹配，不能展示成「没有路由」。不另造强制 matches 的规则；采用该默认值会扩大新对象的匹配范围，需纳入评审和真实 Envoy 反例。

### AIBackend 定义服务 API，身份策略独立附着

| 字段 | 拟定行为 |
| --- | --- |
| spec.displayName | 沿用已有资源的可修改、可重复展示名称 |
| spec.backendRef.name | 引用一个 Backend，网络地址只在 Backend 维护 |
| spec.schema.name | 必填；首版只接受官方枚举值 OpenAI，不把 Chat Completions 或 Responses 操作再造为 schema |
| spec.schema.prefix | 可选的上游 API 路径前缀。省略或空字符串时有效值为 `/v1`；接受有或没有前导 `/` 的路径前缀，沿用官方形式，不含域名、query、fragment |

字段来源是官方 [AIServiceBackendSpec](https://github.com/theagentrouter/agent-router/blob/daa9f891a8afcb18576d4593ad18870d1d18abac/api/v1beta1/ai_service_backend.go#L54)和 [VersionedAPISchema](https://github.com/theagentrouter/agent-router/blob/daa9f891a8afcb18576d4593ad18870d1d18abac/api/v1beta1/shared_types.go#L18)。这里有一个源码与注释差异：shared_types 的注释暗示省略 prefix 可不使用前缀，但 [schemaToFilterAPI](https://github.com/theagentrouter/agent-router/blob/daa9f891a8afcb18576d4593ad18870d1d18abac/internal/controller/gateway.go#L175)对 OpenAI 的省略值及空字符串实际采用 `v1`。本稿按实际编译行为取默认值，显式 `/` 才表达根路径；不把空值默默解释成另一种访问路径。

官方 version 对 OpenAI 被忽略。首版不开放 `schema.version`，避免保存一个不生效的配置；实际增加 AzureOpenAI 等提供商时再采用它的原义，不用 `/v1` 充当提供商版本。也不加入 header/body mutation、providerOptions 或模型列表字段。

schema 定义上游 API 格式，客户端操作由运行实现的标准路径决定。资源能被保存不表示请求已经能转发；模型创建与 MCP 单后端代理已接入数据面，具体操作范围见运行设计；真实服务的操作能力、认证与 TLS 部署仍须单独核对。

拟参考官方 BackendSecurityPolicy，通过 targetRefs 附着 AIBackend，分别管理 API 契约和访问身份。它不是第四类路由，也不改变已确认引用关系。凭据权限、轮转和撤销本就具有独立生命周期，因此不再以「暂时减少资源」为理由把认证写死进 AIBackend。

这是新增待评审的策略建议，不是已批准的第七种 API：目标类型、冲突规则、凭据存储和首个认证形式通过独立 review 后才实现。字段沿用实际选定上游认证类型，不提前做通用注入 DSL、空凭据 resolver 或 providerOptions 字典；不允许明文密钥进入公开配置。

AIBackend 可被多条 ModelRoute 使用，也可访问该服务提供的多个模型。共享账号轮转不要求重写所有模型路由；路由切换模型不要求重建 Backend。认证或 TLS 的必需引用不可用时关闭该访问路径，不回退到无认证、客户端 Authorization 或其他账号。

### ModelRoute 沿用官方规则形状，不另建单操作路由

| 字段 | 拟定行为 |
| --- | --- |
| spec.displayName | 沿用已有资源的展示名称契约 |
| spec.parentRefs | 入口绑定 |
| spec.rules[].name | 可选，设置时资源内唯一；省略不影响使用，最终同资源匹配按规则列表顺序 |
| spec.rules[].matches[].headers[].name | 首版只支持官方模型匹配键 `x-ai-eg-model`；大小写按 HTTP Header 名处理。它指向网关提取的模型名，不信任客户端同名 Header |
| spec.rules[].matches[].headers[].type | 省略为 Exact；首版只支持 Exact，不开放正则或其他 Header 条件 |
| spec.rules[].matches[].headers[].value | 模型名按原值精确匹配，不 trim、不改大小写、不根据资源名推断别名 |
| spec.rules[].backendRefs[].name | 引用 AIBackend；首版每规则恰好一个目标，不支持权重、优先级或自动故障转移 |
| spec.rules[].backendRefs[].modelNameOverride | 可选；沿用官方名称，设置时覆盖请求模型名，省略时保留原值，不从资源名猜测 |

官方 [AIGatewayRouteSpec / Rule / Match](https://github.com/theagentrouter/agent-router/blob/daa9f891a8afcb18576d4593ad18870d1d18abac/api/v1beta1/ai_gateway_route.go#L61)及 [OpenAI 示例](https://github.com/theagentrouter/agent-router/blob/daa9f891a8afcb18576d4593ad18870d1d18abac/examples/basic/openai.yaml)采用以上规则层次，并没有 route 级 path 或 schema。后端类型的历史注释仍提到 AIGatewayRouteSpec.APISchema，但该版本的实际 route 类型和示例已经没有这个字段，不能据旧注释复制。

`matches` 是 OR，match 内的 headers 是 AND。省略或空 matches、或空 match 都表示不限制模型，沿用官方及 HTTPRoute 的缺省匹配语义；有模型条件的规则优先于 catch-all，同级最终按创建时间、Namespace、资源名、规则声明顺序裁决。它会允许任意请求模型名进入指定 AIBackend，而不是自动发现模型；管理界面必须显示「所有模型」，不得显示成「未配置」。catch-all 的声明、编译和运行时匹配均已实现，未知模型只有在没有匹配规则及 catch-all 时才由网关拒绝。

保留官方匹配键不要求 Go 把它实际塞入 HTTP 请求：Go 可以直接用请求体的模型提取结果执行等价匹配。后续请求处理必须清除或覆盖客户端伪造的同名 Header，不能让它选择账号。当前只有模型选择需求，其他 Header、hostnames、InferencePool、filters 和模型发现不随结构一并开放。

客户端格式按已注册的标准操作路径确定。官方 [操作注册](https://github.com/theagentrouter/agent-router/blob/daa9f891a8afcb18576d4593ad18870d1d18abac/cmd/extproc/mainlib/main.go#L328)分别注册 Chat、Responses、Messages 等处理器；[processorForPath](https://github.com/theagentrouter/agent-router/blob/daa9f891a8afcb18576d4593ad18870d1d18abac/internal/extproc/server.go#L98)按路径选择，AIBackend.schema 再决定上游格式。建议同一组模型规则适用于已开放的标准操作，不为每个操作复制一份 ModelRoute；新增运行操作无需改变资源字段。首批开放哪些操作仍按独立协议稿 review，不因保存资源就开放官方全部接口。

入口与上游路径仍独立。例如 Chat 操作后缀 `/chat/completions` 与 AIBackend 的有效 prefix `/v1` 组成上游路径。统一 Go Proxy 拟只接管已批准操作的精确路径，不照搬官方覆盖 `/` 的派生 HTTPRoute，也不新增 rootPrefix/operation 配置；这项入口编译差异须在运行切片 review，避免抢占已有 HTTP/MCP 入口。资源阶段不发布这些入口。

本稿的模型映射只改变出站请求的目标模型。响应中的模型身份默认保留上游 API 语义，不自动按输入别名逆向改写；如果客户端契约要求隐藏或返回别名，需要在首个 codec review 中明确。日志和诊断需区分请求别名、实际模型及服务目标，但不记录原始提示词和密钥。

模型操作确定后，只接受该操作支持的方法。非法请求体、缺失模型、未匹配且没有 catch-all 的模型，以及不支持的操作按该输入 API 的错误格式返回，不伪造一套所有提供商通用的响应 body。错误码、usage、工具调用和 SSE 事件需随协议实现一起定稿。

### MCPRoute 直接引用 Backend，并在引用上定义服务访问参数

| 字段 | 拟定行为 |
| --- | --- |
| spec.displayName | 沿用已有资源的展示名称契约 |
| spec.parentRefs | Gateway/Listener 绑定 |
| spec.path | MCP 的精确入口路径，省略为 `/mcp`，沿用官方默认值 |
| spec.backendRefs[].name | 第一切片只引用一个 Backend，采用单后端代理模式 |
| spec.backendRefs[].path | MCP 上游端点路径，省略为 `/mcp`；沿用官方默认值，不从客户端拼接任意目标 URL |

安全设计确认后采用官方引用内的 `securityPolicy` 位置；本次基础资源字段不开放该字段，也不放置未实现的空安全对象。路由和后端路径采用可选字段：只对省略值默认 `/mcp`，显式空字符串拒绝；路径必须是绝对路径、不含 query/fragment，最大 1024 字符沿用官方 MCP 字段限制。入口采用精确匹配，不对路径做未经确认的尾斜杠改写。

MCPRoute 不经过 AIBackend，因为它不需要模型 API schema 转换。首个切片限定远程 Streamable HTTP，不启动 stdio 命令；协议版本是 SDK/客户端/后端的运行兼容契约，暂不新增每路由 protocol.transport/versions 配置。单后端透明代理也要覆盖所选版本的正常请求、通知、流式响应、取消与协议错误，不能仅实现 tools/call 或丢弃自己暂时不解析的合法方法。

这里的透明指保留所选版本的协议交互，不表示透传客户端身份。Origin 的允许范围、协议版本协商、对应版本要求的元数据一致性校验和必要 Header 的转发属于 MCP 切片的强制契约；在协议与客户端选择后逐项确定，不用一个普通 HTTP 反向代理测试替代。

不在本稿加入 MCPBackend、工具聚合、工具前缀重命名、工具过滤、跨实例会话存储或 Redis 状态。当前只有一个实际 MCP 后端选择单元；未来若有独立的 MCP 服务复用与治理需求，再评审它的独立资源职责。

MCP 实现版本必须在开发前明确选择，并记录能力矩阵。本轮查阅的 [2026-07-28 Streamable HTTP](https://modelcontextprotocol.io/specification/2026-07-28/basic/transports/streamable-http)已取消协议级会话和 GET 流端点；但 Agent Router v1.2.0 的 POST 主入口仍只调用 legacy 分支，modern 代码尚未接通，不能把 SDK 含有类型或分支当成发布网关端到端支持。没有旧 Ingate API 兼容压力，不等于外部客户端版本可以忽略；当前代码已选择 2026-07-28 单后端 POST 子集，具体约束见[运行契约](ai-model-operations-and-deployment.md#首版-mcpstreamable-http2026-07-28)；客户端与后端互操作仍需验证。

### 首个资源切片的校验与状态边界

以下基础字段与约束已由用户于 2026-10-12 确认。它们不依赖 AI Proxy 已经存在；跨对象引用解析和运行确认不塞进 API 写入路径。

| 对象/字段 | 写入时的约束 | 来源或明确差异 |
| --- | --- | --- |
| 三种新资源的 metadata / displayName | 沿用已有资源的名称、更新版本、Spec/Status 隔离和展示名称规则 | 当前 Ingate 契约，不再加第二套身份 |
| ModelRoute/MCPRoute.parentRefs | 非空，最多 16 项；名称格式、重复及整个 Gateway/section 重叠校验沿用现有 ParentReference | 上界沿用官方；非空模型路由是项目子集，官方 AIGatewayRoute 可无 parent |
| AIBackend.backendRef.name | 必填，符合已有 Backend 引用名称规则；允许引用尚未创建的对象 | 单对象格式与跨对象解析分离 |
| AIBackend.schema | 必填；name 仅 OpenAI；prefix 为合法路径前缀，省略或空值的有效值为 `/v1` | 官方字段与实际默认值；其他 schema、version 不在首版开放 |
| ModelRoute.rules | 1..15 项；可选 name 沿用现有规则名称校验，填写时唯一 | 非空沿用当前 HTTPRoute 子集，上界沿用官方发布 API；不复制其内部保留规则名 |
| ModelRoute.rules[].matches | 可省略或为空；最多 128 项，列表保留顺序 | 沿用官方形状及上界；空值为 catch-all |
| ModelRoute 的单个 match | 可为空；非空时只有一个 `x-ai-eg-model` Exact Header 条件，value 非空 | 首版只支持模型精确匹配，不因沿用 headers 而开放任意 Header 或正则 |
| ModelRoute.rules[].backendRefs | 恰好一个 AIBackend 名称；override 省略或空值表示保留请求模型名 | 官方覆盖语义；单目标是已选范围，不把 Backend 端点权重搬到这里 |
| MCPRoute.backendRefs | 恰好一个 Backend 名称；入口及上游 path 省略值为 `/mcp`，长度上界 1024 | 官方字段及默认值；单目标和显式空路径拒绝是项目子集 |

对已声明字段的未支持枚举或集合数量必须返回明确校验错误，不能接受后只取第一项或忽略。未声明的字段另按原生 API 与 Admin 协议的实际解码契约处理，并验证客户端的严格校验方式；不在本次资源追加中悄悄改变整个 API Server 的未知字段策略，也不把 OpenAPI 描述存在当成严格校验已经生效。

ModelRoute/MCPRoute 的 status 复用现有 `RouteParentStatus`；AIBackend 使用与 Backend 一样的 `conditions`。只声明可用状态结构，不在 Create/Update 中填 Accepted，不新增全局 Ready 或逐规则状态。资源存储切片尚无对应协调器时 status 为空，不能为了让界面「成功」返回伪造的生效状态。

## 示例：同一个模型服务提供两个对外模型

以下是目标字段的构造示例，仅解释资源归属；其中 Backend.tls 已按文末首版边界接入本地源码，尚未进行真实 Envoy 验证，不能把整段 YAML 当作已验收的部署配置。四个对象显式属于 `team-a`，引用在该空间内解析；应用这些资源前需通过原生 Namespace API 创建这个空间，后端不会自动创建。当前 API Server 已接入 AIBackend、ModelRoute、MCPRoute 的声明与存储；模型和 MCP 请求链路已接入，运行边界见文末。域名和模型均为示意，没有访问真实提供商，没有创建凭据。

```yaml
apiVersion: gateway.ingate.io/v1alpha1
kind: Gateway
metadata:
  namespace: team-a
  name: public
spec:
  listeners:
    - name: api
      protocol: HTTP
      port: 8080
      hostname: gateway.example.com
---
apiVersion: gateway.ingate.io/v1alpha1
kind: Backend
metadata:
  namespace: team-a
  name: model-network
spec:
  endpoints:
    - address: model.example.com
      port: 443
      weight: 1
  tls:
    sni: model.example.com
    wellKnownCACertificates: System
---
apiVersion: gateway.ingate.io/v1alpha1
kind: AIBackend
metadata:
  namespace: team-a
  name: model-service
spec:
  backendRef:
    name: model-network
  schema:
    name: OpenAI
    prefix: /v1
---
apiVersion: gateway.ingate.io/v1alpha1
kind: ModelRoute
metadata:
  namespace: team-a
  name: public-models
spec:
  parentRefs:
    - name: public
      sectionName: api
  rules:
    - name: coding
      matches:
        - headers:
            - name: x-ai-eg-model
              type: Exact
              value: coding
      backendRefs:
        - name: model-service
          modelNameOverride: code-v2
    - name: summary
      matches:
        - headers:
            - name: x-ai-eg-model
              type: Exact
              value: summary
      backendRefs:
        - name: model-service
          modelNameOverride: text-v3
```

当前调用方在已开放的标准模型接口发送 `model=coding`，匹配第一条规则并将上游模型覆盖为 `code-v2`；`model=summary` 使用同一 AIBackend 和 Backend，只覆盖为 `text-v3`。这份 ModelRoute 不绑定 Chat 或 Responses，也不声明客户端 schema；操作由标准路径决定。示例没有凭据；TLS 字段的本地实现见文末，真实握手、提供商认证和公网部署尚未验收。

HTTPRoute 和 MCPRoute 的引用分别如下；被引用 Backend 需另行定义，它们不使用上面的模型账号：

```yaml
# HTTPRoute.spec
parentRefs:
  - name: public
    sectionName: api
rules:
  - name: orders
    matches:
      - path: {type: PathPrefix, value: /orders}
    backendRefs:
      - name: orders-network
```

```yaml
# MCPRoute.spec；当前代理仅支持已选版本的单后端 POST 子集
parentRefs:
  - name: public
    sectionName: api
path: /mcp
backendRefs:
  - name: tools-network
    path: /mcp
```

## 裁决：网络入口与协议规则分两层处理

### API Server 校验单对象，Controller 解析跨对象关系

| 约束 | 所有者与执行位置 |
| --- | --- |
| 名称、字段格式、枚举、端口、引用列表重复、已命名规则名重复 | API validation；Strategy 调用；Admin proto 镜像约束须有一致性测试 |
| Create 技术名称冲突、Update resourceVersion 冲突 | 权威 API Server 存储，不用预查询保证原子性 |
| Gateway/Listener/Backend/AIBackend 引用存在性 | Controller；允许先创建路由后补引用 |
| Listener 冲突、跨 ModelRoute 别名优先级 | 同一个共享数据面 Controller；模型 POST 与普通 HTTP 的顺序见下文 |
| codec 转换对、运行时能力和凭据访问是否满足 | Controller/运行实例分别报告自己可确认的范围 |
| 客户端请求是否合法、命中哪个已编译模型规则 | Go 协议模块；不查询原始领域资源重新裁决 |

第一阶段不按六种资源机械创建六个 xDS Controller。一个所有者汇总已实现资源、稳定裁决并生成两类输出；MySQL 投影保留独立失败和重试边界。krt 派生集合读取不是跨资源事务；发布前核对结果收敛，事件与重试继续驱动最新期望。

### 多个模型规则可以共享一个网络入口

网络入口键由 Gateway/Listener 的有效 Host、已开放操作路径和方法决定。同一绑定上的 ModelRoute 规则汇总后用于各个已开放模型操作；不同模型别名不构成 HTTP 路径冲突。Envoy 把模型入口交给 Go；具体规则由 Go 选择。汇总仅是编译细节，不新增「模型入口组」资源、公开 ID 或生命周期。

相同入口下，相同模型条件需要按匹配优先级选择；有模型条件优先于 catch-all。模型规则不能读取客户端自报的内部路由标识来跳过匹配。输入格式由操作路径决定，不再有两个 ModelRoute 在同路径声明不同输入 schema 的冲突。

普通 HTTP 按 Gateway API 已实现匹配条件确定优先级：Exact 优先，PathPrefix 越长越优先；跨同类路由平局用创建时间、Namespace、name；同一路由最终用规则列表顺序，不按可选规则名排序。具有相同 match 的 HTTPRoute 不因重叠而整体删除，较低优先级规则保留为候选，优先规则删除后可重新选择。原 Ingate 将完全重复路径标为冲突；本次设计 review 已确认改变这一行为。

2026-10-15 接通模型入口时，按简单路由顺序处理重叠，不增加跨种类冲突检测或发布阻断：同一 Listener/Host 配置了 ModelRoute 后，`POST /v1/chat/completions` 先进入 Go，包括普通 HTTPRoute 也精确匹配该路径的情况；其他方法或路径继续按普通 HTTPRoute 匹配。模型未命中或目标失效在 Go 内拒绝，不回退到普通 HTTP。删除该入口全部 ModelRoute 后，才恢复纯 HTTP 路由表。2026-10-16 用户批准的 Responses 创建扩展对 `POST /v1/responses` 使用相同顺序，不增加操作字段或新的冲突规则；已通过 review 并获准本地提交，具体范围见[模型操作](ai-model-operations-and-deployment.md#模型操作)。MCP 精确路径同样优先普通 HTTP；与已启用模型入口完全重叠时按文末的 MCP 边界处理。此顺序是统一 Go Proxy 的本项目选择，不声称是官方模型 ExtProc 的实现方式。

有模型条件的规则之间按创建时间、名称、声明顺序选择，后端引用有效性不改变匹配优先级，失效不能使同名模型自动改由其他账号。

普通 HTTP 的 Exact 优先、较长 PathPrefix 优先保持不变。例如 HTTPRoute 的 `/` 前缀可以和模型的精确 `/v1/chat/completions` 共存，后者优先；它们不是完全相同的网络匹配。

单独处理附着、匹配和转发动作：附着不成功时没有该入口；规则语法/能力无效时按部分无效处理；引用失效时保留已选规则的匹配，替换成错误动作，不把不可用目标或凭据编入有效转发配置。HTTP 命中失效 Backend 引用返回 500，参考标准及 Envoy Gateway 翻译测试；上游连接故障不是同一类配置错误，按网络错误处理。第一切片不支持多目标，不能拿未来的多目标配置暗中重分配失效权重。

模型/MCP 采用同样的「保留匹配、返回协议错误」原则，具体 HTTP 状态及响应体随已选协议定稿；不另建 AI guard 概念。失效不落到普通 HTTP `/` 兜底，不匿名调用，也不自动换账号。删除所有对应协议路由后，原入口归属才消失，可能命中剩余 HTTP 兜底；管理端需展示这一影响，不能把删除路由解释成永久的路径封禁。永久禁止访问需要独立安全契约。

### 失效目标不能变成匿名调用或其他账号调用

后端认证 Header 由配置中的访问身份决定。默认不把客户端 Authorization、Cookie 和内部路由/发布标识传给模型或 MCP 后端；协议确需的身份转交必须单独批准。上游返回重定向时，首个切片不自动跟随到任意地址，避免配置外的目标访问和凭据外泄。

客户端不能在请求体、Header 或模型名中提供任意 Backend、URL、凭据引用或出站 Cluster。平台管理员配置网络目标属于高权限操作；SSRF 地址范围、DNS 重绑定和出口网络访问策略必须在实际部署安全契约中定义，不把允许 localhost/私网的正常业务能力机械删除。

## 状态和删除：接纳不等于已生效，也不等于探测成功

拟沿用 Gateway API `status.parents[]` 的 parentRef、controllerName、conditions 结构。Conditions 带 observedGeneration，只在观测结果变化时写回。不强制新建逐规则 status 数组或要求规则都有名字；诊断可以指出规则索引/可选名称，但不能用它另造一套状态协议。

- `Accepted=True` 表示该绑定至少有一个规则已由 Gateway 实现，不表示后端可调用。已实现匹配但引用失效时，可同时是 Accepted=True、ResolvedRefs=False，并返回错误；不能为了显示「不可用」篡改接纳语义。
- `ResolvedRefs` 独立说明引用解析结果；普通缺后端沿用 BackendNotFound，不能再造 BackendMissing 同义 Reason。
- 规则语法/能力部分无效时参考 PartiallyInvalid；单纯引用失效不机械等同于该条件。优先沿用 NoMatchingParent、NotAllowedByListeners、UnsupportedValue 等标准语义；SchemaConflict、UnsupportedConversion 等协议特有结果确有必要时才新增并说明。不复制 Agent Router 的单个 NotAccepted condition 为通用路由状态。
- Backend 未被使用不是错误，不必因无流量写 Accepted=False；网络连通性、模型账号有效性不由字段校验推断。
- 实例加载、Envoy ACK/NACK 和真实请求探测分别取证，见[编译资源契约](ai-compiled-config-contract.md#分发与应用)。不采用历史全局确认协议，也不在六种领域资源内拼一个无实例身份的全局 Ready。

删除继续允许造成悬空引用，不通过跨对象预查询强行阻止。删除 Backend 使依赖的 HTTP/MCP 规则和 AIBackend 引用失效；删除 AIBackend 只影响引用它的模型规则；删除 Gateway 或 Listener 只影响相应绑定。Controller 重新编译并移除失效转发目标、保留应报错的匹配，不级联删除管理员创建的其他资源。引用补齐后自动恢复。

普通配置更新后的已有流使用已捕获视图，新流使用新版本。删除和凭据撤销有不同安全优先级，不能仅靠旧流排空无限保留访问权限。撤销时限、旧流终止、重启恢复和离线实例处理属于尚未批准的发布/安全契约；在这些契约完成前，不承诺立即撤销或生产可用。

## 认证：沿用既有策略模式，凭据存储不能假设存在

我们需要区分三类身份：Admin API 管理员、AI/MCP 客户端、模型/MCP 上游账号。管理员 Session 不在数据面复用；后端 TLS 身份也不等同于提供商 APIKey。

本文拟采用附着 AIBackend 的 BackendSecurityPolicy，以及 MCPRoute 后端引用上的 securityPolicy，分别对应官方模型与 MCP 的现有方式。前者需要独立策略生命周期；后者与该路由的单后端服务绑定一起变化，不为形式对称强行统一。无策略只表示未配置服务账号，不能表示继承客户端身份；有认证要求的服务必须在凭据方案完成后接入。

当前 API Server 没有 Secret，不能只写 `secretRef` 就宣称可用。凭据存储仍需选择：安装具备独立权限与存储保护的凭据资源，或接入已有外部凭据系统。只做该选择的设计，不新建空 resolver 接口、文件监听替代设施或 Secret 业务 API。引用结构、版本关联、读权限、加密、轮转和撤销需独立定稿；真实 APIKey 接入在它之前停止。

非敏感网络和领域配置可以进入查询投影；凭据内容不进入 MySQL、运行时公开快照、状态、日志或示例。Proxy 只获得处理所分配配置所需权限，不能借 Watch 权限读取全部管理员数据或所有凭据。API Server 的存储 integration 测试未覆盖这套权限。

## 管理接口和查询模型随资源一起替换

六种资源均提供 Create/Get/List/完整 PUT/Delete，继续同时支持 HTTP 和 gRPC。Create/Update 返回权威 API Server 的保存结果；Get/List 只读取 MySQL 查询模型，接受异步滞后，不增加 read-your-write 回退。

PUT 带 `version`，不允许改资源名；Repository 获取当前对象、核对版本、保留 metadata、替换 spec，并由 API Server 再执行并发裁决。Status 更新也可能改变版本，投影需继续消费此类更新。Delete 保留当前按名称无版本前提的语义；不暗中增加阻止被引用资源删除的行为。

查询存储仍是可重建投影，不参与跨资源接纳。已实现的 HTTPRoute 将父引用和有序规则保存在专用表的 JSON 列，取消只用于拼回数组的子表及位置字段；Gateway Listener 和 Backend Endpoint 有对象内唯一键，仍保留关系表。ModelRoute/MCPRoute 使用单行 JSON 规则与父引用，AIBackend 使用协议与网络引用字段，均已接入管理消费者。本项目没有兼容压力，直接修正未发布的迁移基线、移除旧表定义，不保留升级历史；但不自动销毁任何已有数据库或 etcd 数据。详见 [0012](../decisions/0012-resource-names-and-query-schema.md)。

管理员查询是否展示完整 status、控制面接纳与实例生效状态如何关联，是后续管理契约问题。本稿不静默改变现有 Get/List 的数据来源和权限。

## 理由与取舍：保持独立变化边界，不复制参考项目全部功能

| 备选方案 | 本稿选择与代价 |
| --- | --- |
| ModelRoute 直接指向网络 Backend | 已确认不采用；模型服务契约独立复用。多了引用校验和删除影响，Controller 在编译时解析，不增加数据面 API 查询跳数 |
| 给 MCP 再增加一层 MCPBackend | 当前不采用；单后端服务参数放在 MCPRoute 的后端引用上。未来出现独立复用需求时再评审，不能为名称对称增加资源 |
| 直接采用 Kubernetes Gateway API 全量类型与行为 | 当前采用自有 Group 和明确子集；Namespace 作用域已确认，不额外引入 GatewayClass/Service 管理；必须公开差异，不能宣称兼容认证 |
| 所有策略都内联或都独立附着 | 不作统一规定；TLS 沿用 Envoy Gateway 内联子集，模型认证拟沿用独立 BackendSecurityPolicy，MCP 认证沿用引用内 securityPolicy。按独立权限/生命周期选择，不建立泛型 Policy 系统 |
| 仍让每条路由只含一个父入口、一个匹配 | 本稿用 parent/rule/match 结构，按 parent 的标准 Conditions 表达附着、接纳和引用结果，不发明逐规则 status 协议 |
| 用一个泛型协议对象处理 HTTP、模型和 MCP | 不采用；运行基础共用，协议资源和错误语义独立，不定义任意配置字典或插件接口 |

依据、固定提交、实际代码与反例测试见[上游对照记录](ai-gateway-upstream-alignment.md)。Gateway API 的实验 XBackend 不作为稳定网络资源标准；BackendTLSPolicy 是已有独立附着方案，Envoy Gateway v1.9.2 的内联 TLS 也是已有方案，不能简单把「内联」说成非标准。此处选择的是有来源的裁剪，不是全量兼容承诺。

## 历史资源替换与兼容边界

这会破坏旧的 Route/Upstream 类型、REST 资源路径、生成客户端、Admin protobuf 及对应管理路径。我们接受新项目的这一变化，不设计双写、旧接口转发、旧类型 alias 或双版本长期共存。

| 已删除的旧契约 | 当前契约 | 功能边界 |
| --- | --- | --- |
| Route.gatewayName + listenerName | HTTPRoute.parentRefs[].name + 可选 sectionName | 保留显式 Listener 绑定；增加整个 Gateway 附着和多个绑定 |
| Route.path | HTTPRoute.rules[].matches[].path | 保留 Exact/PathPrefix；拟支持逐规则接纳 |
| Route.upstreamName | HTTPRoute.rules[].backendRefs[].name | 保留普通转发，不强制进入 Go |
| Upstream.endpoints | Backend.endpoints | 保留 IP/DNS、端口、端点权重和确定排序 |
| routes/upstreams 管理接口及类型 | http-routes/backends 管理接口及类型 | 统一替换客户端和调用方，不保留兼容路由 |
| 单个 Route 的 Accepted | 逐 parent 的 Accepted/ResolvedRefs 等 Conditions | 引用失效不再等同于未接纳；查询方不能只用 Accepted 判断服务可用 |

还需明确核对：缺省 Gateway 附着、缺省 `/` 匹配、规则列表顺序、重叠路由优先级、失效引用保留匹配、TLS/认证与跨种类冲突，以及 MySQL 投影。普通 HTTP 成功路径保留，不表示状态和错误路径保持不变；这些是向成熟契约对齐的显式行为变化，须先通过用户 review。历史 ADR 和完成度报告保留其源码快照，不改成尚未实施的事实。

## 当前编译与状态边界

六类资源经 controller-runtime 源缓存进入 krt。对象转换、父引用附着、入口路由表与后端网络结果
分别维护依赖；发布前检查异步集合收敛，按最新期望生成 Envoy 与 AI Proxy 的命名资源。
失败不把部分输入当空配置；细节与复杂度边界见[增量设计](control-plane-incremental-design.md)。

Gateway 和 HTTPRoute 当前由协调器写回已观测的接纳与引用结果；AIBackend、ModelRoute、MCPRoute
虽有 status 结构及子资源，但尚未实现完整状态协调。保存成功、控制面发布、代理 ACK 和后端可用
不能合并为一个“已生效”状态。

## 验证与未实施范围

资源默认值、单对象校验、Spec/Status 隔离、Admin 协议镜像、确定排序、同空间引用、
失效匹配保留、配置删除/恢复和增量缓存复用已有对应测试。
MySQL 事务、Namespace 自动清理、真实 Envoy 转发与 TLS、跨进程恢复需要各自环境验收，
不能用 sqlmock 或 protobuf 检查替代。

未实施的资源能力包括 BackendSecurityPolicy、Secret 凭据、安全策略附着、跨 Namespace 授权、
入口 HTTPS 和多目标协议路由；这些仍是独立设计范围。当前 MCP 版本已固定，但外部客户端兼容性
尚未验收。不采用历史全局实例确认、每请求整份 revision 相等或双版本发布候选。
不提供未测量的 QPS、容量或撤销时限承诺。

## MCP 首版运行边界（2026-10-16）

MCPRoute 已接入资源编译、Delta ADS Loader、单次配置消费、受保护 UDS 与 Envoy 静态出口。
资源保持单 Backend，path 与 backendPath 保留显式尾斜杠，不新增会话或工具配置字段。
采用 2026-07-28 Streamable HTTP，只转发 POST；精确 MCP 路径的其他方法返回 405/Allow: POST，
优先普通 HTTP。同一 Listener/Host 已启用模型路由且入口路径与模型固定操作完全重叠时显式编译失败，
保留有效配置；没有模型路由的入口允许这些路径。同路径 MCP 候选沿原有排序，失效首选不回退。
协议校验、Origin、通知/SSE、源码依据及实际验证边界见
[模型与 MCP 部署设计](ai-model-operations-and-deployment.md#首版-mcpstreamable-http2026-07-28)。
旧稿未实现状态属于相应历史切片，当前不包含 legacy 会话、stdio、聚合、工具执行或凭据；上游 TLS 的本地实现见下一节。

## 上游 TLS 首版

用户确认 Backend 可选内联 TLS，HTTPRoute、模型与 MCP 共享同一网络能力，不新增策略资源。
`tls` 省略或 null 表示明文 HTTP；对象存在时 `sni` 必须为不带通配符、端口、协议头或 IP 的
DNS 主机名，`wellKnownCACertificates` 必须显式为 `System`，不推导默认值。
所有端点使用同一 SNI，并以同一 DNS 身份校验证书 DNS SAN；端点可为 DNS、IPv4 或 IPv6，
显式端口保持原值。TLS 身份不改变普通 HTTP 的 Host 透传或模型/MCP 按所选端点生成 Host 的规则。

[Envoy Gateway v1.9.2 Backend 源码](https://github.com/envoyproxy/gateway/blob/f9f92a112a4033abed5c8d43a5f7e37da257403c/api/v1alpha1/backend_types.go)
提供内联 tls、sni 和 wellKnownCACertificates 字段；固定必填 SNI、仅 System 是本项目确认的子集。
项目当前锁定 `go-control-plane/envoy v1.39.0`。对应
[Envoy v1.39.0 官方 Dockerfile](https://github.com/envoyproxy/envoy/blob/8eea3285d6bdb89f8ea34632cfe7ce1608a8f374/distribution/docker/Dockerfile-envoy)
的标准 Ubuntu 22.04 镜像安装 ca-certificates；
[Ubuntu 系统证书说明](https://manpages.ubuntu.com/manpages/jammy/man8/update-ca-certificates.8.html)
明确 bundle 为 `/etc/ssl/certs/ca-certificates.crt`。首版使用官方标准 Ubuntu Envoy 镜像，
由 Envoy 的 trusted_ca 显式加载该文件，不读取 Controller 主机的信任根，不假定 distroless 等变体具备相同文件。

锁定 API 的 system_root_certs 标为未实现，
[该版本验证上下文源码](https://github.com/envoyproxy/envoy/blob/8eea3285d6bdb89f8ea34632cfe7ce1608a8f374/source/common/ssl/certificate_validation_context_config_impl.cc)
读取 trusted_ca，不能只设置 SystemRootCerts 就宣称已加载系统根。
[DNS SAN matcher 源码](https://github.com/envoyproxy/envoy/blob/8eea3285d6bdb89f8ea34632cfe7ce1608a8f374/source/common/tls/cert_validator/san_matcher.cc)
对 DNS exact matcher 使用证书域名匹配规则，包括合法证书通配名；配置身份本身不接受通配符。
CA 文件属于 Envoy 运行环境，缺失或无效时由 Envoy 加载失败；控制面不能用检查自身文件系统代替部署验证。
系统信任根由部署维护，不开放资源级 CA、mTLS 或跳过验证。

本地源码已接通内部/版本化资源、校验、标准生成、Admin 创建/完整 PUT/查询、异步 MySQL JSON 投影，
以及 HTTP 和 AI 编译与共用 Envoy Cluster；AI Proxy 只接收协议与目标，不接收网络 TLS 配置。MySQL `tls` 为可空 JSON，
显式选择更新该列以保证主动清空；本次仅修正未发布迁移基线，没有运行实库迁移。
TLS 内容参与 AI 配置 revision 及模型/MCP 内容目标名，身份改变生成新目标，不将旧目标复用到不同 TLS 配置。
省略 TLS 的旧 JSON 和目标编码保持不变；旧二进制严格拒绝未知 TLS 字段，升级需先具备消费能力。

TLS 构建失败沿既有失败路径保留有效配置，握手或证书失败不会另建明文连接。
完整 PUT 主动删除 TLS 则会发布明文期望；二者必须区分。
两路配置仍最终一致，xDS Cache 安装不代表 Envoy ACK，失败时可能继续使用先前有效配置，
不能将本切片理解为即时撤销旧明文配置或中断在途连接。
现有测试覆盖字段镜像、投影转换、TLS protobuf 与目标绑定变化，不等于真实握手验证。
真实 Envoy/提供商握手、CA 文件加载和旧连接行为留待统一验收；普通检查以[审查记录](../refactoring-review.md)的日期化结果为准。
