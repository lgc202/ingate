# Ingate Next Agent 工作约定

这个仓库是 Ingate 的全新重写，不应默认沿用旧 `../ingate` 的命名、目录结构或实现习惯。

## 项目方向

- 构建一个面向 API 网关和 AI 网关的声明式 Envoy 控制面。
- Envoy 是唯一数据平面，不为 Kong、Nginx 等假设提前设计 target 抽象。
- Higress 只作为带 Redis 扩展 ABI 的 Envoy 二进制来源；生产 Go 代码不依赖 Higress 产品模型、wrapper 或高层 SDK。
- 一套 Ingate 表示一个环境、一个配置域和一组配置完全相同的 Envoy 实例；一套 Ingate 可以包含多个逻辑 Gateway。
- `Upstream` 统一表示应用、模型、MCP 和 Agent 等网络目标；`type` 表达业务分类，`protocol` 表达实际通信语义。
- 命名要按新设计重新判断，不要被旧项目影响。例如使用 `Upstream`，不要使用 `Backend`。

## 当前范围

当前需要把已有编译链路收缩为直接的 Envoy 配置链路：

```text
Resource -> Envoy Config Compiler -> Config Delivery -> xDS Snapshot Cache -> Envoy
```

删除 RuntimeGroup、RuntimeSnapshot、Target、Translator、公开 Logical IR、独立 ingate-xds 和 ingate-dataplane。Controller 与 xDS 合并为一个进程，但按 Resource Watch、Compiler、Delivery、xDS 和 Status 分模块。声明式资源是唯一持久化事实；Delivery 的 Candidate 和 Active 只保存在进程内，Controller 重启后重新全量编译，不持久化 Last Good。

当前服务和系统组件：

- `ingate`：CLI 和本地调试入口
- `ingate-admin-api`：前端管理 API
- `ingate-apiserver`：声明式资源 API
- `ingate-controller`：资源状态收敛、Envoy 配置编译和 xDS 服务
- `Envoy`：唯一数据平面
- `etcd`：声明式资源持久化，仅由 ingate-apiserver 访问
- `Redis`：限流和 Token 配额等请求路径共享状态

当前不包含：

- AI runtime
- data-plane agent
- Kubernetes operator

## AI Gateway 当前范围

- 不新增 AI runtime 或独立服务，AI 请求继续使用现有 Controller 编译链路和 Envoy 数据面
- 对外统一支持 OpenAI-compatible `POST /v1/chat/completions`；当前上游支持 OpenAI、DeepSeek、通义千问兼容模式、Anthropic 原生协议、Gemini 原生协议和自定义 OpenAI-compatible 服务
- 模型服务仍建模为 `Upstream(type=model)`，通过 `protocol` 表达 OpenAI、Anthropic 或 Gemini 通信语义，通过 `spec.model.provider` 表达具体厂商；不新增 Provider、Model、AIRoute 或 AIBackend 等平行资源
- 模型目录由用户在模型 Upstream 的 `spec.model.models[]` 中手工维护，不自动同步厂商模型列表
- API Key 直接作为模型 Upstream 的认证配置，不创建独立凭据资源；Admin API 不回显密钥，只返回是否已配置，更新时省略表示保留、显式移除才会清除
- 一条模型 RouteRule 的 `modelRouting.models[]` 中每个客户端模型别名各自引用一个模型 Upstream 和 `upstreamModel`，同一路由可以按请求体 `model` 跨多个厂商和 Upstream 选择目标
- Envoy Route 使用受控内部 Header 和内部续接 Route 选择目标 Cluster，并由续接 Route 写入上游 Host；内置 `ai-proxy` Wasm 负责模型别名匹配、协议转换、路径与认证 Header 改写、响应与 SSE 归一化，用户不编辑插件私有配置
- `pkg/llm` 是不依赖 Ingate、Envoy、Proxy-Wasm、Gin 或 Kubernetes 的纯 Go 协议包，不发送模型 HTTP 请求、不读取环境变量、不管理密钥持久化；Provider 协议适配按子包隔离
- 模型 Upstream 通过 `tls.serverName` 使用 HTTPS、SNI 和系统 CA 根证书包校验；配置或保留 API Key 时必须启用 HTTPS
- 当前只支持文本 `system`、`user`、`assistant` 消息，以及 `model`、`messages`、`stream`、`temperature`、`top_p`、`max_tokens`、`stop`；普通响应、SSE、错误和 Token usage 统一为 OpenAI-compatible 结构，响应 `model` 返回客户端公开别名
- 当前不支持 Tools/function calling、多模态、Responses、Embeddings、自动模型同步、多 Provider fallback/retry、模型级重试、OAuth/IAM 云认证或大文件请求；单次请求体上限为 1 MiB
- `TokenQuotaPolicy` 为一个策略定义一个共享预算池，只应用到目标 Gateway 或 Route 下的模型 RouteRule；支持所有请求共享、按客户端 IP 和按请求 Header 值区分预算池
- Token 配额固定统计归一化响应中的输入与输出 `total_tokens`，请求前检查当前固定窗口已用额度，响应结束后按实际 usage 记账；并发中的请求可能造成有限超额，不把当前能力描述为严格预扣的硬额度
- 受 Token 配额保护的 OpenAI-compatible 流式请求由 `ai-proxy` 内部注入 usage 请求参数，客户端协议仍不开放 `stream_options`

## 工程实现原则

- 新能力必须从真实调用方和可验收场景开始，不创建空接口、空目录、占位资源和没有读写方的状态。
- 能直接使用 Go 标准库、现有依赖或目标项目官方实现时优先复用；参考开源项目必须能指向实际源码，不根据架构图自造运行时概念。
- 第三方框架和实现细节不能进入 Ingate 的外部 API、资源名称和产品术语。
- 函数、接口和中间结构只有在隔离真实变化或明显提高主流程可读性时才增加。
- 错误、重试和兼容逻辑按真实调用方需要设计，不建立无人消费的错误分类、状态机和历史兼容层。
- 当前项目没有需要兼容的生产历史数据时，错误的资源或配置设计直接重写，Git 负责保存历史。
- 代码、生成协议、README 和架构文档只保留一套当前事实；撤销的设计直接删除，不在主文档中保留过程性 Plan。
- Go 标识符和日志消息使用英文；代码注释使用中文，解释领域语义、设计约束和不明显的原因。
- 日志消息和结构化字段中的错误文本都不能包含中文；中文错误只用于返回给前端的用户提示，`xerrors.UserError` 通过 `LogValue` 向日志提供稳定的英文语义。

## 部署边界

- 服务二进制、YAML 配置、环境变量、健康检查、日志和优雅退出必须保持部署方式中立，业务代码不能感知 Docker Compose、systemd 或 Kubernetes。
- Docker Compose 只是开发、联调和演示方式，不是唯一部署模型，也不代表生产拓扑。
- 不再提供把 etcd、Redis、Envoy 和多个 Go 服务塞进同一个容器的 all-in-one 镜像。
- Compose 中每个服务使用独立容器和健康检查；Controller 与 Envoy 共享网络命名空间，使未启用 mTLS 的 xDS 继续只监听 loopback。
- 后续只有出现真实交付需求时才分别设计 systemd 或 Kubernetes，不提前建立通用部署抽象。
- 安装包和容器内使用 `/opt/ingate/<component>` 保存二进制、配置和静态资源；运行数据由具体部署方式挂载到明确目录。
- API Server 自身运行证书与用户声明的 Gateway Certificate 是不同概念，后者始终由 API Server 持久化到 etcd。
- 内置插件固定放在 Envoy 可读取的 `/opt/ingate/plugins`，用户不感知插件文件路径。

## 内置治理插件

- 限流、鉴权、访问控制、AI token 配额这类核心治理能力可以使用数据面插件执行，但控制面产品模型必须保持强类型资源，不让用户直接编辑插件私有 JSON。
- 内置治理插件不建模为用户创建的通用插件资源或插件绑定资源；用户配置的是对应的强类型 Policy 和必要的依赖资源。
- 强类型 Policy 通过自身的 `targetRefs[]` 直接引用 Gateway 或 Route，不再使用独立 `PolicyBinding`。`targetRefs[]` 允许为空，表示策略已保存但当前不应用到流量。
- xDS 对内置治理插件采用长期形态：Listener / HCM 注入一次内置 Wasm filter，filter 配置携带 Envoy Config Compiler 生成的可执行策略索引，插件通过当前 xDS route name 定位 route/rule 配置。
- Redis 是系统组件，不建模为 RedisStore 资源；RateLimitPolicy 和 TokenQuotaPolicy 自动使用 Envoy bootstrap 中的 `ingate-system-redis`。RateLimitPolicy 不向用户暴露 Local/Global 计数模式或限流算法，实际执行方式由 Ingate 内部统一选择。
- Redis 扩展由 Ingate 自己维护最小 ABI adapter，现有插件继续使用标准 Proxy-Wasm SDK，生产代码不 import `github.com/higress-group/...`。
- 内置插件随 Ingate 数据面镜像发布，默认放在 `/opt/ingate/plugins`。
- 用户自定义插件仍走普通插件模型，不和内置治理插件混用同一套产品协议。

## Go 版本

- 使用 Go 1.26。
- 完成改动前运行 `make verify`；修改镜像或 Compose 时还要运行 `make docker-up` 并实际验证组件状态。

## 编码规范

### 使用领域名称

- Ingate 自己定义的包、类型和变量不要使用 `runner`、`runtime`、`snapshot` 这类无法直接说明职责的笼统名称，应按实际领域行为命名，例如 `Proxy`、`Compiler`、`Delivery`、`RouteIndex`、`ConfigFingerprint`。
- 不要只把笼统名称替换成同样空泛的 `engine`、`manager`、`processor`；名称应让调用方无需进入实现就能理解其职责。
- 外部协议或依赖库的正式术语可以保留，例如 `runtime.Object`、Envoy xDS `Snapshot` 和 `HttpConnectionManager`。

### 保持代码直接

- 优先写清楚、直接、可读的代码。
- 不要为了“看起来健壮”写大量没有实际意义的防御性编程。
- 对外部输入、跨进程边界、持久化数据、网络返回、配置解析等边界必须校验。
- 对包内刚构造出来、语义已确定的值，不要层层判空或重复校验。
- 构造函数的必需依赖由调用方保证，不做静默 nil 兜底；例如 logger 由服务入口统一注入，不在下层 `New` 中替换成 discard logger。

### 控制抽象层级

- 不要封装太多没有明显收益的子函数。
- 如果一段逻辑只有一个调用点，而且放在当前位置更容易理解，就保持内联。
- 只有在下面情况才拆函数：
  - 逻辑本身形成清晰步骤；
  - 能显著降低当前函数复杂度；
  - 有真实复用；
  - 需要隔离可测试的领域逻辑。
- 调用跳转层级尽量浅，读一条主流程时不应频繁跳 4-5 层才能理解业务含义。

### Admin API 分层

`ingate-admin-api` 按 `handler / dto / service / store` 分层，各层职责必须清楚：

- `handler` 是 HTTP 入口，只负责请求绑定、参数校验、调用 service 和写统一响应。
- `dto` 定义控制台 API 的请求和响应模型，负责产品 DTO 与内部资源模型之间的纯转换。
- `service` 承载用例和业务语义，负责调用 store、处理资源状态和跨资源协调。
- `store` 只封装资源读写，不承载控制台产品语义。
- 前端不直接依赖 Kubernetes 风格资源对象；admin-api 返回面向页面和操作的产品 DTO。

### Handler 层

- Handler 方法只做上层流程控制，不写业务逻辑、资源拼装细节或复杂参数转换。
- 请求参数通过 `gin.Context` 的 `ShouldBindJSON`、`ShouldBindQuery`、`ShouldBindUri` 绑定。
- 绑定失败属于用户输入错误，返回 `http.StatusBadRequest` 和 `err.Error()`，不记录错误日志。
- 绑定成功后调用请求 DTO 的 `Validate` 方法做语义校验和必要的字段转换。
- `Validate` 失败也属于用户输入错误，返回 `http.StatusBadRequest` 和 `err.Error()`，错误文本应尽量明确。
- Handler 只向 service 传递已校验、已转换的参数或资源对象。
- Service 返回错误时，进入 `if err != nil` 后先用项目统一 logging 入口记录 `Error` 日志，再判断是否为可展示错误。
- 可展示业务错误统一使用 `xerrors.UserError` 表达，Handler 使用 `errors.AsType[*xerrors.UserError](err)` 判断并返回其错误文本。
- 非 `UserError` 的 service 错误返回明确的通用失败文案，不把内部错误细节直接暴露给前端。
- Handler 不需要按 `http.StatusConflict`、`http.StatusNotFound` 等细分业务失败状态；后台业务失败默认使用 `http.StatusInternalServerError` 放入统一响应体。
- Service 层如果希望前端展示明确原因，应返回 `UserError`，例如同名冲突、引用不存在、规则冲突等。
- Handler 中优先保持直接主流程，不为了少写几行代码抽 `writeServiceError` 这类 helper；如果未来出现真实共享边界，再提炼统一方法。
- helper 函数中不允许调用 `response.GinJSONResponse`、`response.GinAbortJSONResponse`、`ctx.JSON` 等响应输出方法，可以返回 error 在主入口中处理。
- 不再使用 `response.WriteResult`、`response.WriteError` 这类二次封装；统一使用 `response.GinJSONResponse` 和 `response.GinAbortJSONResponse`。
- 不记录用户输入校验失败日志。
- 操作日志、审计日志这类横切能力按产品需求接入，但不得把核心业务逻辑写进 Handler。

### Admin API 错误响应与校验边界

- Admin API 统一响应体为 `{ "code": number, "msg": string, "data": any }`，Handler 统一使用 `response.GinJSONResponse` 和 `response.GinAbortJSONResponse` 写出。
- `msg` 是当前接口最小稳定错误文案，前端可以直接用于 Toast 或页面错误提示；不要把内部错误、底层存储错误或英文堆栈直接放进 `msg`。
- 请求绑定失败、DTO `Validate` 失败属于请求自身错误，返回 `http.StatusBadRequest`，不记录错误日志。
- Service 返回错误时，Handler 一进入 `if err != nil` 就记录 `Error` 日志，再判断是否是 `xerrors.UserError`。
- `xerrors.UserError` 表示可以展示给用户的业务错误，例如同名冲突、引用不存在、版本冲突、运行状态冲突；非 `UserError` 统一返回通用失败文案。
- 同名校验、跨资源引用存在性、版本冲突、权限、运行状态冲突等依赖系统状态的规则，必须以后端 Service 层结果为最终裁决。
- 前端本地校验只做请求自身可确定的基础输入校验，例如必填、长度、格式、数字范围、同一个表单内重复项；不要根据列表快照对资源名称唯一性、引用存在性等系统状态做硬拦截。
- 前端可以做低风险提示，但不能阻止用户提交依赖后端裁决的场景；保存失败后以前端收到的后端错误为准。
- 如果需要字段级错误展示，后端应返回稳定字段路径，不使用展示文案作为字段标识。推荐将 `data` 约定为 `{ "fieldErrors": [{ "field": "name", "message": "路由名称已存在" }] }`，其中 `field` 使用请求 DTO 的 JSON 字段路径，例如 `name`、`rules[0].targets[0].upstreamID`。
- 字段级错误只增强前端定位能力，不改变后端 Service 是系统状态校验权威这一原则。

### Admin API DTO

- 新增接口的请求类型默认命名为 `{Method}Req`，响应类型默认命名为 `{Method}Resp`；已有稳定 DTO 可在重构时逐步迁移。
- 不允许用户直接传入的派生字段使用 `json:"-"`。
- 请求 DTO 如需校验或参数转换，统一实现 `Validate() error`；`Validate` 只返回 error，不返回转换结果。
- `Validate` 只处理请求自身能判断的校验和纯转换，例如字符串拆分、枚举归一化、数字范围校验、跨字段约束。
- 不允许把 store、service、client 等外部依赖传入 DTO `Validate`；同名校验、引用存在性、资源冲突等依赖系统状态的规则放在 service 层。
- 请求 required 语义优先写在 `Validate` 中，只有确实需要依赖 gin binding 行为时才使用 `binding:"required"`。
- 响应 DTO 可以提供 `New{Method}Resp` 构造函数；构造函数只做字段映射和格式整理，不写业务逻辑。
- DTO 字段必须表达控制台产品契约，不要把内部资源字段、中文展示名或临时实现细节泄漏成协议主键。

### Admin API 资源标识

- 控制台创建资源时，由后端生成 UUID 作为不可变资源 ID，并写入底层资源的 `metadata.name`。
- Admin API 和前端协议字段统一使用 `id` 表达资源主键，不使用 `name` 表达不可变 ID。
- Service 用例层参数按领域命名，例如 `gatewayID`、`routeID`、`upstreamID`；不要把存储层的 `metadata.name` 命名泄漏到用例语义中。
- Store 和 Kubernetes generated client 边界可以继续使用 `name`，因为这里对接的是 apiserver 存储协议。
- 不额外增加 `spec.id`；`metadata.name` 是底层资源主键，Admin API 的 `id` 是它在控制台产品语义中的映射。
- 面向用户展示和编辑的名称使用 `spec.displayName`，Admin API 管理的资源必须填写，不使用展示名称作为资源 ID 或跨资源引用键。
- 同一类资源内 `displayName` 必须唯一，避免控制台出现用户无法区分的重名资源。
- `displayName` 的唯一性校验属于系统状态校验，放在 service 层，不放在 DTO `Validate`。
- 资源之间的引用使用资源 ID，不使用 `displayName`。
- 声明式 apiserver 可以保留调用方指定 `metadata.name` 的能力；Admin API 面向控制台体验，创建流程可以和声明式 API 不同。

### 治理策略与内置插件

- 当前已落地执行链路保留 `RateLimitPolicy` 和 `AccessControlPolicy`；删除 `PolicyBinding` 和 `RedisStore`，鉴权等治理能力后续重新设计后再加入。
- 核心治理能力可以在数据面通过内置插件执行，但用户协议和 admin-api 不能暴露为普通插件资源、插件绑定资源或插件私有 JSON。
- 内置治理插件由系统自动注入、自动配置并通过 Envoy xDS 配置生效；用户不需要独立安装插件，也不需要感知插件版本、phase、priority 或 Wasm 文件路径。
- `RateLimitPolicy` 和 `AccessControlPolicy` 通过 `targetRefs[]` 表达策略应用到哪些 Gateway 或 Route；策略配置和目标引用都由对应强类型 Policy 承载。
- Policy 的总体结果写入 `status.conditions`，每个目标的解析和生效结果写入 `status.targets[]`。缺失目标不影响其他有效目标继续发布；任一目标已生效时总体可视为已生效，部分生效和异常由 `status.targets[]` 表达；没有目标或所有目标都未接入流量时使用 `NotApplied`。
- 外部服务、证书等可复用运行依赖按真实产品需求独立建模；系统 Redis 是安装级基础组件，不进入用户资源协议。
- 内置治理插件可以参考 Higress 等项目的实现思路，但不能依赖第三方产品的 wrapper、matchRules 或高层配置协议；Redis hostcall 通过 Ingate 自己维护的最小 ABI adapter 隔离。
- 后续新增策略时按资源类型拆分 admin-api 的 handler、dto、service、store，不把不同策略堆进一个大 `policy` 文件，也不写进 Route/Gateway 的用例层。
- Envoy Config Compiler 负责解析强类型策略的 `targetRefs[]`，并生成 Envoy 与内置插件可执行配置；插件私有结构不能泄漏到用户 API。

### 标准库与依赖使用

- 可以使用新版本 Go 标准库能力来简化代码，例如 `slices.Contains`、`slices.IndexFunc`、`maps.Clone` 等。
- 可以使用 `github.com/samber/lo` 简化集合转换、过滤、查找等样板代码。
- 使用 `lo` 的前提是让代码更短、更清楚；不要为了函数式写法牺牲直接可读性。
- 简单 `for` 循环如果更直观，就保留 `for` 循环。
- 引入新依赖前要确认它解决的是重复样板或明显可读性问题，不要为了单个很小用法增加依赖。

### 常量

- 有明确领域含义、协议含义或重复使用的数字和字符串，优先定义为常量。
- 不要散落魔法数字、魔法字符串，例如 target 名称、资源 kind、默认值、固定版本前缀、协议名称等。
- 常量命名要表达业务含义，不要只把字面量机械搬到 `const`。
- 有明确枚举语义的常量优先定义专用类型，例如 `type Kind string` 再定义对应 `const`。
- 专用常量类型和对应 `const` 要紧贴放置，中间不要插入不相关声明。
- 专用常量类型命名要结合所在 package，避免外部使用时重复啰嗦，例如 `resource.KindRoute` 优于 `resource.ResourceKindRoute`。
- 只在局部上下文内明显自解释、且不会复用的普通字面量，可以保持内联。

### 文件组织顺序

- Go 文件内代码组织尽量保持固定顺序：常量、变量、结构体定义、导出函数、工具函数。
- 类型定义中如果有专用常量类型，要和对应 `const` 紧贴放在一起，放在结构体定义之前。
- 不为追求顺序把强相关的小块拆散；如果某个文件已有更清楚的局部组织方式，以可读性优先。

### 函数与 receiver

- 不要写太多游离函数。
- 如果函数天然属于某个类型的行为，优先收成 method receiver。
- 只有在下面情况使用游离函数：
  - 它是纯转换、纯构造、纯校验，并且不属于某个稳定对象；
  - 它会被多个类型或包共同使用；
  - 它作为小型 helper 能让调用点更清楚。
- receiver 名称保持简短、稳定，避免 `this`、`self` 这类命名。

### 接口使用

- 不要提前定义接口。
- 不允许只为了测试而定义生产代码接口。
- 接口必须来自真实协作边界，例如：
  - 多个生产实现；
  - 外部系统边界；
  - package 之间需要反转依赖；
  - 调用方只需要一个很小的能力集合。
- 接口尽量定义在消费者侧，而不是实现侧。
- 接口要小，避免胖接口。
- 生产代码优先返回具体类型，除非隐藏实现细节确实有价值。

### 测试

- 当前项目不维护单元测试，不新增 `*_test.go` 或前端单元测试文件。
- `make test` 仅用于执行 Go 全包编译检查，避免删除测试后失去最基本的编译验证。
- 行为验证优先使用实际组件联调、端到端请求和人工验收，不为测试引入 mock、接口或额外抽象。

### 注释

- 代码注释使用中文，包括 package comment、导出类型注释、导出函数注释和必要的实现说明。
- 注释要解释领域含义、设计约束或不明显的原因，不要复述代码本身。
- 涉及核心链路、跨层转换、协议适配、AI 网关配置生效逻辑等关键或难懂的地方，要主动补充说明性注释。
- 注释要讲清楚“为什么这样做”和“配置如何生效”，尤其要说明普通网关配置与 AI 网关配置的差异。
- 注释不需要以句号结尾，保持简洁即可。
- 英文专有名词可以保留英文，例如 Envoy、xDS、Gateway、Route、Upstream。
- 对外协议字段、错误文本、CLI 输出等用户可见字符串，按实际产品语境决定中英文，不受代码注释语言限制。

## Git 规则

- 提交要聚焦，一个提交只做一类事情。
- 不要从这个仓库修改旧项目 `../ingate`。
- 不要提交 `_output/`、`.gocache/`、`.gomodcache/` 或其它本地构建产物。
