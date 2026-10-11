# 三类路由复用 Backend，以 AIBackend 定义模型服务访问契约

- 状态：用户于 2026-10-10 review 通过设计，并于 2026-10-11 授权提交阶段 B 的 HTTP 资源替换当前实现。凭据、TLS 和首个 AI/MCP 协议等独立确认边界仍保留
- 作者：Ingate 开发协作
- 更新日期：2026-10-10
- 讨论入口：本仓库文档 review，依据 [0010 决策](../decisions/0010-unified-ai-proxy-config-delivery.md)及用户确认的资源关系
- 源码基线：`56205b84321c`，分支 `codex/rebuild-from-zero`
- 范围：资源契约、裁决边界、管理接口影响和实施顺序；不修改业务代码或生成文件

## 摘要

我们按已确认关系设计 Gateway、HTTPRoute、ModelRoute、AIBackend、MCPRoute、Backend。三种路由通过 `parentRefs` 绑定 Gateway；HTTPRoute 和 MCPRoute 直接引用 Backend，ModelRoute 引用 AIBackend，AIBackend 再引用 Backend。Backend 管理网络目标，AIBackend 管理模型服务契约，ModelRoute 管理模型入口与目标映射。

项目没有旧方案兼容压力，Route、Upstream 的职责统一替换到 HTTPRoute、Backend，不保留同义的兼容 API。替换仍须保留当前普通 HTTP 路径匹配、端点权重、资源版本并发控制、异步管理查询等功能。设计已通过 review；本次实施限于阶段 B，后续切片仍遵守各自确认边界。

本次修订按[固定版本上游对照](ai-gateway-upstream-alignment.md)收敛字段与行为。通用路由语义以 Gateway API 为主；模型服务以 Agent Router 为主；网络端点和 TLS 以 Envoy Gateway 为主。不把三者的全部功能混成一个自有框架，也不将名称相近等同于标准兼容。下文新增的策略附着建议仍待评审。

![已确认的资源引用关系](../diagrams/ai-resource-model.svg)

## 背景：现有契约能表达网络转发，不能表达模型服务

基线版本的 RouteSpec 只有 `gatewayName`、`listenerName`、`path`、`upstreamName`，UpstreamSpec 只有端点集合。它们没有客户端模型名、上游模型映射、协议 schema 或后端账号语义。阶段 B 已替换为 [HTTPRouteSpec](../../pkg/apis/gateway/v1alpha1/httproute.go)与 [BackendSpec](../../pkg/apis/gateway/v1alpha1/backend.go)；新增协议层尚未实现。

基线裁决按 Gateway/Listener 和规范化路径判断冲突；阶段 B 的[裁决](../../internal/controllermanager/controller/gateway/resolve.go)已保留同路径候选，按父引用独立附着。[Envoy 适配层](../../internal/controllermanager/dataplane/envoy/resources.go)按 Exact 优先、长 PathPrefix 优先稳定排序，IP/DNS 后端采用 EDS/STRICT_DNS。后端连接超时仍为 5 秒，没有上游 TLS 配置。改名不能填补这些协议和安全缺口。

当前 Admin 接口保留 [0005](../decisions/0005-admin-resource-management.md)的 API Server 写入、MySQL 异步查询分工；[资源存储映射](../../internal/apiserver/registry/gateway/storage.go)已替换为 Gateway、HTTPRoute、Backend 及其 status。没有 Namespace、Secret、运行时快照或租约资源。本文不假设完整 Kubernetes 已存在。

本稿输入是已确认讨论和现有代码，没有另建 PRD，也没有新增计费、配额、Agent、工具聚合、插件市场或多租户业务需求。

## 设计：先固定资源身份和引用边界

### API 沿用项目自己的 Group，不冒充 Gateway API 实现

拟采用 `gateway.ingate.io/v1alpha1`，保留无版本内部类型、版本化外部类型、生成 Conversion/DeepCopy/OpenAPI/Clientset 的现有形式。六种主资源使用 Kubernetes metadata、spec、status 结构。

| Kind | API Server 资源名 | 管理端集合路径（拟定） |
| --- | --- | --- |
| Gateway | gateways | /api/admin/v1/gateways |
| HTTPRoute | httproutes | /api/admin/v1/http-routes |
| ModelRoute | modelroutes | /api/admin/v1/model-routes |
| AIBackend | aibackends | /api/admin/v1/ai-backends |
| MCPRoute | mcproutes | /api/admin/v1/mcp-routes |
| Backend | backends | /api/admin/v1/backends |

拟继续采用集群级资源，名称在同一 Kind 内唯一；不同 Kind 的同名对象互不等同。当前没有租户或 Namespace 管理契约，暂不引入命名空间、跨命名空间引用和 ReferenceGrant。集群级资源不提供租户隔离；如果产品要提供租户自治，必须在该产品能力实现前重新评审作用域与权限，不能用名称前缀替代隔离。

资源名称沿用当前 DNS subdomain 规则；Listener 名称沿用 DNS label 规则。端点集合采用现有确定排序；路由规则列表保留声明顺序，因为它是 Gateway API 规定的最终同资源匹配裁决因素。更新仍使用原样的 `metadata.resourceVersion`，不把它解释成跨资源业务发布版本。

### parentRefs 沿用 Gateway 与 Listener 的附着语义

三种路由统一使用以下引用形状；下列是字段声明示意，不是 Go 类型或已安装 API：

```yaml
parentRefs:
  - name: public
    sectionName: api
```

`name` 引用 Gateway，必填；`sectionName` 可选，设置时引用其中的 Listener，省略时引用整个 Gateway，由其符合协议及附着要求的 Listener 接纳。至少一个 Listener 接纳即表示该 parent 附着成功，不表示所有 Listener 均成功。沿用 Gateway API 的语义，不再要求额外填写 sectionName 才能使用。

引用限定到 Gateway，不接受任意 Group/Kind，不增加未实现的 namespace 或按端口绑定。Gateway 改 Listener 端口时，显式 Listener 引用仍稳定；删除或改 Listener 名称时，该绑定失效。省略 sectionName 会使后续新增的合格 Listener 也可能接纳该路由，管理端须展示此影响；需要固定入口时显式填写。

`parentRefs` 是非空集合，重复的 `(name, sectionName)` 在写入时拒绝。同一 Gateway 的多条引用必须都显式指定不同 Listener，不能同时声明整个 Gateway 和其某个 Listener；该约束沿用上游以避免重叠引用。拟允许一个路由绑定多个入口，各绑定独立接纳和报告状态。非空要求是本项目只管理有入口路由的范围限制；Gateway API HTTPRoute 本身允许无 parent 的对象，本文不宣称完整一致性。

### Gateway 继续只拥有入口配置

| 字段 | 拟定行为 |
| --- | --- |
| spec.displayName | 可修改、允许重复的展示名称，不参与引用和匹配 |
| spec.listeners[].name | Gateway 内唯一，供 sectionName 引用 |
| spec.listeners[].protocol | 本次 HTTP 资源替换阶段只接受现有 HTTP；HTTPS 需完成证书与权限契约后接入 |
| spec.listeners[].port | 1..65535 |
| spec.listeners[].hostname | 保留空值或精确 DNS 名称；空值表示该端口任意 Host |

模型和 MCP 是 HTTP 之上的业务协议，不把 `Model` 或 `MCP` 添加为 Listener 网络协议。公网入站 TLS、AI 客户端认证和 Listener 的路由附着授权需要单独完成安全设计；当前管理员 Session 不承担这些职责。

Gateway、HTTPRoute、Backend 的 Admin 创建请求不接收技术名称；后台生成不可修改的 `metadata.name`，供引用、查询和必要时展示“资源 ID”。三种资源均提供可修改且允许重复的 `spec.displayName`；列表参数 `display_name` 使用包含匹配，不限制为前缀。原生 API 保留 name / generateName 与 metadata.uid，MySQL 不额外投影 UID。详见 [名称契约](../decisions/0012-resource-names-and-query-schema.md)。

跨 Gateway 的 Listener 冲突仍按 [0002](../decisions/0002-shared-data-plane-controller.md)的创建时间、名称稳定裁决。暂不引入 wildcard Host、GatewayClass、部署编排或流量管理策略资源。

### Backend 管理网络目标，不管理模型账号

| 字段 | 拟定行为与依据 |
| --- | --- |
| spec.endpoints[].address | IP 或 DNS 名称，不允许 URL、路径、查询和用户信息；保留现有端点能力 |
| spec.endpoints[].port | 明确端口，1..65535，不根据协议悄悄覆盖 |
| spec.endpoints[].weight | 正 int32，保留当前端点相对权重；不是模型路由分流权重 |
| spec.tls | 可选；设置即要求加密上游连接，省略保留当前明文 HTTP 行为。沿用 Envoy Gateway Backend 的内联 TLS 形状，不另造 transport 包装 |
| spec.tls.sni | 首个 TLS 切片拟要求固定 DNS 身份，用于 SNI，并以同一 DNS 身份验证证书；不从不可信客户端 Host 推导身份。这是明确标注的安全子集，上游允许省略或自动派生 |
| spec.tls.wellKnownCACertificates | 首个 TLS 切片仅支持 System，使用运行环境系统信任根；名称沿用上游，不另造 trust 枚举 |

端点集合必须非空，沿用当前地址/端口重复校验及确定排序。原稿额外提出的 IP 文本归一化、DNS 小写与根点改写不作为本次资源替换条件；它们不是新增协议能力所必需。若后续发现真实重复目标问题，再结合网络解析语义与 Admin/API Server 一致性测试评审，不借重构扩大校验范围。

一个 Backend 的端点是同一服务的可替换网络副本，TLS 身份和连接策略一致。不能把具有不同 API 契约、账号或证书身份的提供商地址随意塞进一个端点池。相同域名也不强制合并 Backend；需要连接、健康或安全隔离时使用不同 Backend 对象。

连接超时暂保留现有内部 5s，不为了参考项目有流量策略就新增 connectTimeout API。连接超时不约束整个模型响应或流空闲；后者随首个协议运行切片另行定稿。

HTTP Host/authority 与 TLS SNI 是不同语义，不能因为使用 TLS 就强制新增 authority 字段。普通 HTTP 保留当前 Host 转发；模型出口的 Host 重写参考 Agent Router 的自动 hostname rewrite，待出口拓扑确定后在编译/网络适配层验收，不凭客户端路径或 Host 决定上游身份。

访问 HTTPS 模型提供商必须先实现并验收 TLS。TLS 构建或验证失败不能降级为明文。主动删除 tls 是改变期望配置，不等于证书失效；是否禁止已加密服务改为明文、谁可修改以及旧流撤销时限，仍须安全 review。首个切片不同时支持内联 TLS 与独立 BackendTLSPolicy，避免先引入尚无需求的合并优先级；私有 CA、mTLS、跳过证书校验和其他网络治理能力不随上游字段一并开放。

### HTTPRoute 保留现有普通转发功能

拟采用 `spec.parentRefs` 和 `spec.rules[]`。规则的 `name` 可选，填写时在资源内唯一；不以规则名决定匹配优先级。第一实现切片每个规则只接受一个 Backend 引用；集合形状用于明确规则与目标绑定，不意味着已支持多个目标或故障转移。未支持的输入拒绝，不静默取第一项。

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
| spec.backendRef.name | 引用一个 Backend，网络地址只在 Backend 维护 |
| spec.schema.name | 沿用 Agent Router VersionedAPISchema 的命名，如 OpenAI；不把 Chat Completions 操作再造为 OpenAIChatCompletions schema |
| spec.schema.version | 沿用提供商版本语义；如 AzureOpenAI 使用 API version，OpenAI 的该字段不表示 `/v1`。实际开放枚举与校验由已实现 codec 决定 |
| spec.schema.prefix | 上游 API 路径前缀，如 `/v1`；沿用官方 prefix，不另加含义重叠的 basePath，不含域名、query、fragment |

schema 定义 API 格式，操作能力另由实现范围限定。即使接口自称 OpenAI-compatible，也要用选定操作和测试证据确认兼容范围；支持 Chat Completions 不表示支持 Responses、音频或图片。首个 codec、实际提供商和协议版本仍待用户确认；不在 schema 枚举中列出尚未实现的协议。

拟参考官方 BackendSecurityPolicy，通过 targetRefs 附着 AIBackend，分别管理 API 契约和访问身份。它不是第四类路由，也不改变已确认引用关系。凭据权限、轮转和撤销本就具有独立生命周期，因此不再以「暂时减少资源」为理由把认证写死进 AIBackend。

这是新增待评审的策略建议，不是已批准的第七种 API：目标类型、冲突规则、凭据存储和首个认证形式通过独立 review 后才实现。字段沿用实际选定上游认证类型，不提前做通用注入 DSL、空凭据 resolver 或 providerOptions 字典；不允许明文密钥进入公开配置。

AIBackend 可被多条 ModelRoute 使用，也可访问该服务提供的多个模型。共享账号轮转不要求重写所有模型路由；路由切换模型不要求重建 Backend。认证或 TLS 的必需引用不可用时关闭该访问路径，不回退到无认证、客户端 Authorization 或其他账号。

### ModelRoute 同时声明客户端 API 和模型选择

| 字段 | 拟定行为 |
| --- | --- |
| spec.parentRefs | 入口绑定 |
| spec.path | 模型 API 的精确入口路径；不根据 schema 自动创建公网路径 |
| spec.schema | 客户端输入 API 格式；与 AIBackend 的上游格式形成转换对 |
| spec.rules[].name | 可选，设置时资源内唯一；省略不影响使用，最终同资源匹配按规则列表顺序 |
| spec.rules[].matches[].model | 非空客户端模型名，按原值精确匹配；不推断别名、不增加 wildcard |
| spec.rules[].backendRefs[].name | 引用 AIBackend；第一切片每规则一个目标 |
| spec.rules[].backendRefs[].modelNameOverride | 可选；沿用官方名称，设置时覆盖请求模型名，省略时保留原值，不从资源名猜测 |

请求 `model` 不是网络 Host，也不是 Backend 的属性。具体上游模型覆盖属于一次路由到模型服务的绑定。`matches[].model` 是本项目待评审的类型化适配：官方从请求体提取模型，再以内部 Header 匹配；统一 Go 模块直接对提取结果匹配，无须把 `x-ai-eg-model` 这个实现载体开放为客户端可信输入。复用其精确模型匹配语义，不增加新的「模型实体」或发现机制。

首个模型切片拟要求每条规则至少一个精确 model match；不提供省略匹配即开放所有模型的 catch-all。它是操作范围裁剪，和 HTTPRoute 的缺省 `/` 不是同一个契约；如果批准全模型转发，应直接补充官方 catch-all 语义与权限反例，而非增加另一个模型选择 DSL。

入口路径与上游路径独立。候选 Chat Completions codec 输出操作的相对路径 `/chat/completions`，与 AIBackend schema.prefix 的 `/v1` 形成 `/v1/chat/completions`；不把完整客户端入口路径再接到 prefix 后产生 `/v1/v1/...`，也不从客户端 URI 推导任意上游路径。ModelRoute.path 是显式暴露单一操作的适配，与官方根 prefix 下支持多种操作不同，须通过用户确认；不将其称为官方原样契约。

本稿的模型映射只改变出站请求的目标模型。响应中的模型身份默认保留上游 API 语义，不自动按输入别名逆向改写；如果客户端契约要求隐藏或返回别名，需要在首个 codec review 中明确。日志和诊断需区分请求别名、实际模型及服务目标，但不记录原始提示词和密钥。

模型入口路径确定后，只接受所选模型 API 支持的方法。非法请求体、缺失模型、未知别名和不支持的操作按该输入 API 的错误格式返回，不伪造一套所有提供商通用的响应 body。错误码、usage、工具调用和 SSE 事件需随首个 codec 一起定稿。

### MCPRoute 直接引用 Backend，并在引用上定义服务访问参数

| 字段 | 拟定行为 |
| --- | --- |
| spec.parentRefs | Gateway/Listener 绑定 |
| spec.path | MCP 的精确入口路径，省略为 `/mcp`，沿用官方默认值 |
| spec.backendRefs[].name | 第一切片只引用一个 Backend，采用单后端代理模式 |
| spec.backendRefs[].path | MCP 上游端点路径，省略为 `/mcp`；沿用官方默认值，不从客户端拼接任意目标 URL |
| spec.backendRefs[].securityPolicy | 沿用官方 MCP 后端引用的安全配置附着位置；只开放已批准认证形式，不继承模型账号 |

MCPRoute 不经过 AIBackend，因为它不需要模型 API schema 转换。首个切片限定远程 Streamable HTTP，不启动 stdio 命令；协议版本是 SDK/客户端/后端的运行兼容契约，暂不新增每路由 protocol.transport/versions 配置。单后端透明代理也要覆盖所选版本的正常请求、通知、流式响应、取消与协议错误，不能仅实现 tools/call 或丢弃自己暂时不解析的合法方法。

这里的透明指保留所选版本的协议交互，不表示透传客户端身份。Origin 的允许范围、协议版本协商、对应版本要求的元数据一致性校验和必要 Header 的转发属于 MCP 切片的强制契约；在协议与客户端选择后逐项确定，不用一个普通 HTTP 反向代理测试替代。

不在本稿加入 MCPBackend、工具聚合、工具前缀重命名、工具过滤、跨实例会话存储或 Redis 状态。当前只有一个实际 MCP 后端选择单元；未来若有独立的 MCP 服务复用与治理需求，再评审它的独立资源职责。

MCP 实现版本必须在开发前明确选择，并记录能力矩阵。本轮查阅的 [2026-07-28 Streamable HTTP](https://modelcontextprotocol.io/specification/2026-07-28/basic/transports/streamable-http)已取消协议级会话和 GET 流端点；但 Agent Router v1.2.0 的 POST 主入口仍只调用 legacy 分支，modern 代码尚未接通，不能把 SDK 含有类型或分支当成发布网关端到端支持。没有旧 Ingate API 兼容压力，不等于外部客户端版本可以忽略；首个版本待客户端/后端和互操作验证确定，示例不预选最新版本。

## 示例：同一个模型服务提供两个对外模型

以下是拟定字段的构造示例，仅解释资源归属，当前 API Server 无法接收。域名和模型均为示意，没有访问真实提供商，没有创建凭据。

```yaml
apiVersion: gateway.ingate.io/v1alpha1
kind: Gateway
metadata:
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
  name: public-models
spec:
  parentRefs:
    - name: public
      sectionName: api
  path: /v1/chat/completions
  schema:
    name: OpenAI
  rules:
    - name: coding
      matches:
        - model: coding
      backendRefs:
        - name: model-service
          modelNameOverride: code-v2
    - name: summary
      matches:
        - model: summary
      backendRefs:
        - name: model-service
          modelNameOverride: text-v3
```

调用方请求中的 `model=coding` 匹配第一条规则。Go 按输入 schema 读取模型名，使用 AIBackend 的上游 schema/prefix，并把该次目标模型覆盖为 `code-v2`；连接使用 Backend 的端点及 TLS 配置。选择 `summary` 时仍访问同一个模型服务，只将目标模型覆盖为 `text-v3`。示例未包含待定的凭据策略，不能用于有认证要求的真实服务，也不是公网部署配置；不能靠未配置策略获得客户端身份。

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
# MCPRoute.spec；运行协议版本须先完成客户端与 SDK 能力核对
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
| Listener 冲突、跨种类入口归属、跨 ModelRoute 别名冲突 | 同一个共享数据面 Controller |
| codec 转换对、运行时能力和凭据访问是否满足 | Controller/运行实例分别报告自己可确认的范围 |
| 客户端请求是否合法、命中哪个已编译模型规则 | Go 协议模块；不查询原始领域资源重新裁决 |

第一阶段不按六种资源机械创建六个 xDS Controller。一个所有者汇总已实现资源、稳定裁决并生成两类输出；MySQL 投影保留独立失败和重试边界。多个 Cache List 仍不是跨资源事务，必须允许重复协调和短暂引用失效。

### 多个模型规则可以共享一个网络入口

网络入口键由 Gateway/Listener 的有效 Host、路径和已实现的方法约束决定。不同 ModelRoute 使用同一个精确路径、相同输入 schema 时，可以在内部编译结果中共用一份模型路由表；不同模型别名不构成 HTTP 路径冲突。Envoy 把该入口交给 Go，并传入可信的入口及发布标识；具体模型规则由 Go 在请求中选择。这个分组仅是编译实现细节，不新增「模型入口组」资源、公开 ID、管理接口或生命周期。

相同入口下，相同模型别名需要按匹配优先级选择。模型规则不能读取客户端自报的内部路由标识来跳过匹配。同一路径使用不兼容的输入 schema 时，无法靠同一个请求体解析器区分，报告入口冲突，不随意选择 schema 或把已转换请求再次转换。

普通 HTTP 按 Gateway API 已实现匹配条件确定优先级：Exact 优先，PathPrefix 越长越优先；跨同类路由平局用创建时间、资源名称；同一路由最终用规则列表顺序，不按可选规则名排序。具有相同 match 的 HTTPRoute 不因重叠而整体删除，较低优先级规则保留为候选，优先规则删除后可重新选择。原 Ingate 将完全重复路径标为冲突；本次设计 review 已确认改变这一行为。

HTTPRoute、MCPRoute 和模型路由表之间完全相同的网络匹配，是上游未为本项目统一运行方式规定的跨种类冲突。拟按创建时间，再按 Kind、资源名称裁决入口归属；Kind 的字典序只是同时间稳定裁决，不是业务优先级。模型表先稳定确定输入 schema，再对兼容规则按创建时间、资源名称和规则列表顺序选择别名。后端引用是否有效不参与匹配优先级，失效不能使同名模型自动改由其他账号或输入 API 服务。这个跨种类规则必须单独通过 review，不称为 Gateway API 标准。

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
- 实例加载、Envoy ACK/NACK 和真实请求探测仍按 [0010 的候选发布协议](../decisions/0010-unified-ai-proxy-config-delivery.md#候选发布流程必须区分加载ack-和真实生效)单独设计，不在六种领域资源内拼一个无实例身份的全局 Ready。

删除继续允许造成悬空引用，不通过跨对象预查询强行阻止。删除 Backend 使依赖的 HTTP/MCP 规则和 AIBackend 引用失效；删除 AIBackend 只影响引用它的模型规则；删除 Gateway 或 Listener 只影响相应绑定。Controller 重新编译并移除失效转发目标、保留应报错的匹配，不级联删除管理员创建的其他资源。引用补齐后自动恢复。

普通配置更新后的已有流使用已捕获视图，新流使用新版本。删除和凭据撤销有不同安全优先级，不能仅靠旧流排空无限保留访问权限。撤销时限、旧流终止、重启恢复和离线实例处理属于尚未批准的发布/安全契约；在这些契约完成前，不承诺立即撤销或生产可用。

## 认证：沿用既有策略模式，凭据存储不能假设存在

我们需要区分三类身份：Admin API 管理员、AI/MCP 客户端、模型/MCP 上游账号。管理员 Session 不在数据面复用；后端 TLS 身份也不等同于提供商 APIKey。

本文拟采用附着 AIBackend 的 BackendSecurityPolicy，以及 MCPRoute 后端引用上的 securityPolicy，分别对应官方模型与 MCP 的现有方式。前者需要独立策略生命周期；后者与该路由的单后端服务绑定一起变化，不为形式对称强行统一。无策略只表示未配置服务账号，不能表示继承客户端身份；有认证要求的服务必须在凭据方案完成后接入。

当前 API Server 没有 Secret，不能只写 `secretRef` 就宣称可用。凭据存储仍需选择：安装具备独立权限与存储保护的凭据资源，或接入已有外部凭据系统。只做该选择的设计，不新建空 resolver 接口、文件监听替代设施或 Secret 业务 API。引用结构、版本关联、读权限、加密、轮转和撤销需独立定稿；真实 APIKey 接入在它之前停止。

非敏感网络和领域配置可以进入查询投影；凭据内容不进入 MySQL、运行时公开快照、状态、日志或示例。Proxy 只获得处理所分配配置所需权限，不能借 Watch 权限读取全部管理员数据或所有凭据。API Server 的存储 integration 测试未覆盖这套权限。

## 管理接口和查询模型随资源一起替换

各集合拟提供当前已有的 Create/Get/List/完整 PUT/Delete，继续同时支持 HTTP 和 gRPC。Create/Update 返回权威 API Server 的保存结果；Get/List 只读取 MySQL 查询模型，接受异步滞后，不增加 read-your-write 回退。

PUT 带 `version`，不允许改资源名；Repository 获取当前对象、核对版本、保留 metadata、替换 spec，并由 API Server 再执行并发裁决。Status 更新也可能改变版本，投影需继续消费此类更新。Delete 保留当前按名称无版本前提的语义；不暗中增加阻止被引用资源删除的行为。

查询存储仍是可重建投影，不参与跨资源接纳。已实现的 HTTPRoute 将父引用和有序规则保存在专用表的 JSON 列，取消只用于拼回数组的子表及位置字段；Gateway Listener 和 Backend Endpoint 有对象内唯一键，仍保留关系表。ModelRoute/MCPRoute/AIBackend 的投影只在真实消费者阶段按批准字段设计。本项目没有兼容压力，直接修正未发布的迁移基线、移除旧表定义，不保留升级历史；但不自动销毁任何已有数据库或 etcd 数据。详见 [0012](../decisions/0012-resource-names-and-query-schema.md)。

管理员查询是否展示完整 status、控制面接纳与实例生效状态如何关联，是后续管理契约问题。本稿不静默改变现有 Get/List 的数据来源和权限。

## 理由与取舍：保持独立变化边界，不复制参考项目全部功能

| 备选方案 | 本稿选择与代价 |
| --- | --- |
| ModelRoute 直接指向网络 Backend | 已确认不采用；模型服务契约独立复用。多了引用校验和删除影响，Controller 在编译时解析，不增加数据面 API 查询跳数 |
| 给 MCP 再增加一层 MCPBackend | 当前不采用；单后端服务参数放在 MCPRoute 的后端引用上。未来出现独立复用需求时再评审，不能为名称对称增加资源 |
| 直接采用 Kubernetes Gateway API 全量类型与行为 | 当前采用自有 Group 和明确子集；避免引入未要求的 Namespace/GatewayClass/Service 管理，但必须公开差异，不能宣称兼容认证 |
| 所有策略都内联或都独立附着 | 不作统一规定；TLS 沿用 Envoy Gateway 内联子集，模型认证拟沿用独立 BackendSecurityPolicy，MCP 认证沿用引用内 securityPolicy。按独立权限/生命周期选择，不建立泛型 Policy 系统 |
| 仍让每条路由只含一个父入口、一个匹配 | 本稿用 parent/rule/match 结构，按 parent 的标准 Conditions 表达附着、接纳和引用结果，不发明逐规则 status 协议 |
| 用一个泛型协议对象处理 HTTP、模型和 MCP | 不采用；运行基础共用，协议资源和错误语义独立，不定义任意配置字典或插件接口 |

依据、固定提交、实际代码与反例测试见[上游对照记录](ai-gateway-upstream-alignment.md)。Gateway API 的实验 XBackend 不作为稳定网络资源标准；BackendTLSPolicy 是已有独立附着方案，Envoy Gateway v1.9.2 的内联 TLS 也是已有方案，不能简单把「内联」说成非标准。此处选择的是有来源的裁剪，不是全量兼容承诺。

## 兼容性：这是契约替换，不能只改文件名

这会破坏旧的 Route/Upstream 类型、REST 资源路径、生成客户端、Admin protobuf 及对应管理路径。我们接受新项目的这一变化，不设计双写、旧接口转发、旧类型 alias 或双版本长期共存。

| 当前契约 | 目标契约 | 功能边界 |
| --- | --- | --- |
| Route.gatewayName + listenerName | HTTPRoute.parentRefs[].name + 可选 sectionName | 保留显式 Listener 绑定；增加整个 Gateway 附着和多个绑定 |
| Route.path | HTTPRoute.rules[].matches[].path | 保留 Exact/PathPrefix；拟支持逐规则接纳 |
| Route.upstreamName | HTTPRoute.rules[].backendRefs[].name | 保留普通转发，不强制进入 Go |
| Upstream.endpoints | Backend.endpoints | 保留 IP/DNS、端口、端点权重和确定排序 |
| routes/upstreams 管理接口及类型 | http-routes/backends 管理接口及类型 | 统一替换客户端和调用方，不保留兼容路由 |
| 单个 Route 的 Accepted | 逐 parent 的 Accepted/ResolvedRefs 等 Conditions | 引用失效不再等同于未接纳；查询方不能只用 Accepted 判断服务可用 |

还需明确核对：缺省 Gateway 附着、缺省 `/` 匹配、规则列表顺序、重叠路由优先级、失效引用保留匹配、TLS/认证与跨种类冲突，以及 MySQL 投影。普通 HTTP 成功路径保留，不表示状态和错误路径保持不变；这些是向成熟契约对齐的显式行为变化，须先通过用户 review。历史 ADR 和完成度报告保留其源码快照，不改成尚未实施的事实。

## 实现计划：按完整消费者路径切分，阶段完成后等待 review

2026-10-11 用户调整推进顺序：阶段 B 后继续阶段 D/E 的契约评审与已批准实现；阶段 C 的真实组件联动、ACK/NACK、实库及故障场景留到后续统一测试，不作为当前开发前置条件。下面阶段字母标识范围，不再表示必须顺序完成。TLS、认证和凭据的设计与必要实现仍随对应功能推进；实际密钥接入前的安全边界不因测试延期取消。备查清单见 [HTTP 主链路隔离验证方案](http-gateway-validation-plan.md)。

| 阶段 | 范围与受影响路径 | 验收及停止条件 |
| --- | --- | --- |
| A 契约 review | 本稿的作用域、字段、裁决、状态、管理路径；另行完成凭据与首个协议选择 | 字段逐项确认后才编码；未解决部分不以示例补全 |
| B HTTP 资源统一替换 | pkg/apis 内外类型、validation、Scheme；Registry；Controller 裁决/编译/状态；dataplane；Admin proto/Handler/Service/Repository 产品视图；删除重复的 Domain 资源类型；投影与迁移；测试及文档 | 所有生产消费者统一到 HTTPRoute/Backend；已实现 HTTP 功能正反例；生成代码可重复；审查全部手写改动后等用户 review |
| C 后续统一测试：HTTP 主链路与 TLS | API Server、Manager、ADS、真实 Envoy、可控 HTTP/HTTPS 后端；明确认证环境 | 当前不搭建；统一测试时覆盖创建/更新/删除、Host/路径不匹配、DNS/IP、证书错名/不受信任、ACK/NACK/重连；TLS 失败不明文降级 |
| D 模型契约与最小运行切片 | 已选 schema、AIBackend/ModelRoute、派生运行视图与 Go 模型模块；认证策略与凭据方案先通过独立 review | 同路径不同别名、未知模型、引用失效、非流/SSE、取消、可信入口标识、配置切换和轮转；不在此加入提供商业务重试 |
| E MCP 契约与最小运行切片 | 已选协议版本、MCPRoute、单后端 Proxy；复用已验证加载与生命周期 | 该版本完整请求与流式交互、非法 Origin、版本不符、取消与后端失败；不默认加入聚合或 Redis 会话 |
| F 部署与故障验证 | 实际入站/出站权限、凭据保护、撤销界限、长流排空、实例状态及容量 | 环境证据齐备后评估上线，不把静态检查和单测当作完成验收 |

生成沿用 [generate-api.sh](../../scripts/generate-api.sh)与现有 `make generate-kubernetes`、`make generate-proto`。API 类型和 protobuf 是各自机械产物的来源，生成文件不手改。新增资源只在它的真实消费者阶段引入，不先创建未实现 Registry、空 Controller 或空协议包。

当前生成脚本会清理带 client-gen 标记的旧文件，但其他生成产物是否残留、发现接口是否仍暴露旧资源，需要单独核对。工作区旧 API 数据、MySQL 数据和迁移版本先只读盘点；需要重建、迁移或删除时，列出确切目标并取得授权，不借契约替换自动清理用户数据。

这一计划不授权安装 Envoy、启动容器、改变外部服务或接入真实模型密钥。新二进制、HTTP/SSE 运行参数、双输出发布及激活状态的详细设计属于后续稿。

## 验证矩阵：每项能力都要有失效时会失败的断言

| 契约 | 正例 | 必需反例与故障场景 | 验证层 |
| --- | --- | --- | --- |
| parentRefs | 显式 Listener、多入口和缺省整个 Gateway | 缺 Gateway、缺 Listener、重叠引用、零个合格 Listener；部分 Listener 成功不伪称全部成功 | validation、裁决、真实 API |
| HTTP 匹配 | Exact、路径段前缀、Host、缺省 `/` | `/api` 不匹配 `/apix`、不同 Host、同资源列表顺序、跨资源时间/名称平局；重排无关资源结果不变 | 单元 + 真实 Envoy |
| Backend | IP/DNS、端点权重、端点排序稳定 | 重复地址/端口、空端点、TLS 错名/不受信任、不允许构建失败降级 | validation + 网络组件 |
| 内部模型路由表 | 同路径两个 model 到正确目标；省略 override 保留原值 | 相同别名按顺序、schema 冲突、未知 model、优先目标失效不改换账号或输入 API | 编译 + Go + Envoy |
| 失效引用 | 目标恢复后重新服务 | HTTP 保留 match 返回 500；模型/MCP 返回协议错误；不落到 HTTP 兜底、不匿名调用 | 整链路与故障注入 |
| 身份和可信标识 | 批准身份用于指定目标 | 伪造入口/发布标识、客户端 Authorization/Cookie 不泄露、重定向不带出密钥 | 真实中间件 + 测试后端 |
| MCP | 所选版本的普通与 SSE 响应 | 版本不符、非法 Origin、方法错误、流关闭取消、通知语义 | 选定客户端/后端互操作 |
| CRUD/投影 | 保存后异步可查询、完整 PUT | 陈旧版本冲突、status 更新令牌追平、删除悬空引用、投影失败恢复 | API integration + 实库 |
| 生效与撤销 | 目标版本加载后切换 | NACK、版本失配、发布中重启、离线实例、撤销和有界旧流 | 整链路及运行恢复 |

代码阶段按仓库 AGENTS 检查全部新增/修改手写文件及调用方，执行 gofmt、diff check、lint、vet、test、build；API Server 存储变化执行 integration，API/proto 变化重新生成并核对。纯文档阶段只检查结构、链接、示例一致性与空白，不重复声称先前 Go 检查等于本稿契约已实现。

暂不填写没有测量依据的 QPS、延迟或内存上限。实现阶段要定义并验证请求体/事件大小、并发流、有界缓冲、旧视图保留数量、退出期限及 API 资源规模上限；缺少这些界限不能评估生产容量。

## 待确认事项：资源关系不再重选，剩余选择影响实现范围

1. 自有 Group、集群级作用域与 HTTP 子集的附着、匹配、状态和失效引用变化已确认；对应阶段 B 实现另行代码 review。
2. 首个模型 API/操作、真实提供商、类型化 model 匹配及单操作入口适配；OpenAI 只是构造示例，不因参考例自动新增所有提供商。
3. BackendSecurityPolicy 的独立附着建议、凭据存储与权限、AI 客户端入站认证，以及内联 TLS 子集与出口部署边界；未完成前不接真实密钥或公网入口。
4. MCP 的目标客户端与协议版本、Go SDK 的真实支持范围；先确定单后端代理，不把聚合当作已有需求。
5. 运行时资源、发布标识、实例确认、撤销、故障恢复和保留界限；沿 0010 候选流程另行定稿。

以上独立确认事项阻止对应未定范围的实现，不阻止已批准的 HTTP 阶段 B；安全机制与首个 AI/MCP 协议不因设计 review 自动扩展。

## 本轮工作流与证据边界

本轮没有可调用的 GoCanon MCP，沿用本地规则源 `/Users/lgc202/workspace/source/lgc202/gocanon`，HEAD `e4f88af7b385cf13d58fa55a5b7856971f60f997`。该规则仓库有其他未提交改动，本轮读取的开发入口、规范索引、执行清单和选定规范文件未修改。

按文档任务沿用已完整读取的开发入口及 Generic APIServer、Controller Manager、参数校验规范；这些规则文件本次没有变化，仍为草案，仅作为设计审查依据。重点核对 APISERVER-007/009/010/014/015、CONTROLLER-005/008/011/013/015/016、VALIDATION-008/009/010/011/013，与目标仓库 0002、0005 和 0010 的实际边界对照，不套入 GoCanon 参考业务。

初稿采用 to-design 整理选择、备选方案与破坏性影响；设计修订采用 technical-writing 核对可追溯依据，采用 prd-to-spec 整理字段、行为、调用方和验收矩阵。HTTP 子集已通过设计 review，并按阶段 B 实施；本阶段代码与验证见 [0011](../decisions/0011-http-resource-contract.md)。这不代表真实 Envoy、提供商、MCP、凭据或实库验收。
