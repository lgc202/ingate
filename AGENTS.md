# Ingate Next Agent 工作约定

这个仓库是 Ingate 的全新重写，不应默认沿用旧 `../ingate` 的命名、目录结构或实现习惯。

## 项目方向

- 构建一个声明式 Envoy API 网关控制面。
- Envoy 是唯一数据平面，不为 Kong、Nginx 等假设提前设计 target 抽象。
- Higress 只作为带 Redis 扩展 ABI 的 Envoy 二进制来源；生产 Go 代码不依赖 Higress 产品模型、wrapper 或高层 SDK。
- 一套 Ingate 表示一个环境、一个配置域和一组配置完全相同的 Envoy 实例；一套 Ingate 可以包含多个逻辑 Gateway。
- `Upstream` 表示普通 HTTP 上游服务，通过端点、TLS、负载均衡和健康检查描述连接方式。
- 命名要按新设计重新判断，不要被旧项目影响。例如使用 `Upstream`，不要使用 `Backend`。

## 当前范围

当前需要把已有编译链路收缩为直接的 Envoy 配置链路：

```text
Resource -> Envoy Config Compiler -> Config Delivery -> xDS Snapshot Cache -> Envoy
```

删除 RuntimeGroup、RuntimeSnapshot、Target、Translator、公开 Logical IR、独立 ingate-xds 和 ingate-dataplane。Controller 与 xDS 合并为一个进程，但按 Resource Watch、Compiler、Delivery、xDS 和 Status 分模块。声明式资源是唯一持久化事实；Delivery 的 Candidate 和 Active 只保存在进程内，Controller 重启后重新全量编译，不持久化 Last Good。

当前服务和系统组件：

- `ingate-console`：控制台静态资源和管理 API 反向代理入口
- `ingate-admin-api`：前端管理 API
- `ingate-apiserver`：声明式资源 API
- `ingate-controller`：资源状态收敛、Envoy 配置编译和 xDS 服务
- `Envoy`：唯一数据平面
- `etcd`：声明式资源持久化，仅由 ingate-apiserver 访问
- `Redis`：内置限流插件的共享计数存储

当前已经落地的控制面资源包括 Gateway、Route、Upstream、Certificate、RateLimitPolicy 和 IPRestrictionPolicy。在此基础上，产品将扩展为统一的 API 与 AI 网关，围绕模型服务接入、对外模型发布、调用方授权、用量治理和请求分析形成完整链路。

AI 网关的产品对象保持克制：

- `Service` 表示实际连接的普通 HTTP、模型或 MCP 服务；现有 `Upstream` 如何演进为统一 Service 需要在后端协议设计时一次性确定，不建立并行兼容层。
- `Model` 表示客户端使用的稳定对外模型名，并引用一个或多个模型服务线路；厂商、凭据和真实模型属于 Service，不单独创建 Provider、ProviderModel 或 Credential 资源。
- `Caller` 表示调用网关的应用或服务，承载访问密钥、模型与 Route 权限、额度和用量归属。
- AI 限额、内容安全、参数约束和缓存继续采用有明确业务语义的强类型 Policy，不暴露数据面插件私有配置。
- 请求明细、Token、成本和线路尝试属于运行记录，不建模为声明式资源。

当前先使用 Mock 数据完成控制台产品体验，确认模型接入、发布、授权、调用和排障流程后再设计后端资源与执行链路。Agent 编排、Prompt 管理、数据集、模型训练、计费开票和复杂审批流不属于网关核心范围；后续 Agent 服务作为 Caller 使用 Ingate 的模型与 MCP 能力。

## 工程实现原则

- 新能力必须从真实调用方和可验收场景开始，不创建空接口、空目录、占位资源和没有读写方的状态。
- 能直接使用 Go 标准库、现有依赖或目标项目官方实现时优先复用；参考开源项目必须能指向实际源码，不根据架构图自造运行时概念。
- 第三方框架和实现细节不能进入 Ingate 的外部 API、资源名称和产品术语。
- 函数、接口和中间结构只有在隔离真实变化或明显提高主流程可读性时才增加。
- 错误、重试和兼容逻辑按真实调用方需要设计，不建立无人消费的错误分类、状态机和历史兼容层。
- 当前项目没有需要兼容的生产历史数据时，错误的资源或配置设计直接重写，Git 负责保存历史。
- 代码、生成协议、README 和架构文档只保留一套当前事实；撤销的设计直接删除，不在主文档中保留过程性 Plan。
- Go 标识符和日志消息使用英文；代码注释使用中文，解释领域语义、设计约束和不明显的原因。
- 日志消息和结构化字段中的错误文本都不能包含中文；中文错误只用于返回给前端的用户提示。Admin API 使用 Kratos Error 的稳定英文 `reason/message` 表达日志语义，中文提示只放在响应元数据中。

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

- 限流和 IP 访问限制使用数据面插件执行，但控制面产品模型必须保持强类型资源，不让用户直接编辑插件私有 JSON。
- 内置治理插件不建模为用户创建的通用插件资源或插件绑定资源；用户配置的是对应的强类型 Policy 和必要的依赖资源。
- 强类型 Policy 通过自身的 `targetRefs[]` 直接引用 Gateway 或 Route，不再使用独立 `PolicyBinding`。`targetRefs[]` 允许为空，表示策略已保存但当前不应用到流量。
- xDS 对内置治理插件采用长期形态：Listener / HCM 注入一次内置 Wasm filter，filter 配置携带 Envoy Config Compiler 生成的可执行策略索引，插件通过当前 xDS route name 定位 route/rule 配置。
- Redis 是系统组件，不建模为用户资源；RateLimitPolicy 自动使用 Envoy bootstrap 中的 `ingate-system-redis`。RateLimitPolicy 不向用户暴露 Local/Global 计数模式或限流算法，实际执行方式由 Ingate 内部统一选择。
- Redis 扩展由 Ingate 自己维护最小 ABI adapter，现有插件继续使用标准 Proxy-Wasm SDK，生产代码不 import `github.com/higress-group/...`。
- 内置插件随 Ingate 数据面镜像发布，默认放在 `/opt/ingate/plugins`。
- 用户自定义插件仍走普通插件模型，不和内置治理插件混用同一套产品协议。

## Go 版本

- 使用 Go 1.26。
- 项目可安装的开发工具统一放入 `_output/tools`，生成脚本不得污染全局 `$GOPATH/bin`；工具版本优先由 `go.mod`、`package-lock.json` 等现有依赖清单锁定。
- Go 生成工具通过 `make tools` 安装，宿主机上的 Go、Node、npm 和 Docker Compose 等外部工具通过 `make check-tools` 检查。
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

`ingate-admin-api` 完整采用 Kratos 的 `server / service / biz / data` 分层和 Wire 装配：

- `api/admin/v1` 保存控制台 HTTP API 的 Proto 和 Buf 生成代码，是前端产品协议；前端不直接依赖 Kubernetes 风格资源对象。
- `server` 只装配 Kratos HTTP transport、中间件、统一响应和错误编码，不写领域转换或业务规则。
- `service` 实现生成的 HTTP service 接口，负责 Proto 请求校验、Proto 与 Ingate 资源之间的协议转换和调用 biz usecase。
- `biz` 承载用例、业务语义和消费者侧 Repository 接口，负责同名校验、引用校验、资源状态和跨资源协调；不得依赖生成客户端或 HTTP transport。
- `data` 实现 biz 的 Repository 接口并屏蔽 API Server 客户端细节。
- `data/apiserver` 封装声明式资源读写，Admin API 不直接访问 etcd、Redis 或关系数据库。
- `conf` 使用 Proto 定义进程配置，通过 Kratos Config 加载；`cmd/ingate-admin-api` 使用 Kratos App 管理 transport、启动钩子和优雅退出。
- Kratos 已提供的 Config、Error、Log、HTTP、Middleware 和生命周期能力优先直接使用，不再为 Admin API 维护 Gin、Cobra、自定义日志或自定义错误类型。

### Admin API service 层

- Service 方法只做协议入口流程：校验请求自身、转换为 biz 用例参数、调用 usecase、转换响应。
- 请求绑定由 Buf 生成的 Kratos HTTP handler 完成；语义校验失败返回 Kratos `BadRequest`。
- 同名、引用存在性、版本冲突和运行状态冲突等依赖系统状态的规则放在 biz，不放在 Proto 转换函数中。
- Service 不直接访问 API Server 或生成客户端。
- 操作日志由 Kratos middleware 统一记录；Service 不重复记录错误。
- 私钥等敏感请求内容不得出现在请求日志或错误日志中。

### Admin API 错误响应与校验边界

- Admin API 统一响应体为 `{ "code": number, "msg": string, "data": any }`，由 Kratos HTTP response/error encoder 统一写出。
- `msg` 是当前接口最小稳定错误文案，前端可以直接用于 Toast 或页面错误提示；不要把内部错误、底层存储错误或英文堆栈直接放进 `msg`。
- 请求绑定和 service 校验失败属于请求自身错误，返回 `http.StatusBadRequest`。
- 可展示业务错误使用 Kratos Error 的稳定英文 `reason/message`，中文用户提示放在 `user_message` metadata；非 Kratos 业务错误统一转换为内部错误并保留 cause 供日志记录。
- 同名校验、跨资源引用存在性、版本冲突、权限、运行状态冲突等依赖系统状态的规则，必须以后端 biz 层结果为最终裁决。
- 前端本地校验只做请求自身可确定的基础输入校验，例如必填、长度、格式、数字范围、同一个表单内重复项；不要根据列表快照对资源名称唯一性、引用存在性等系统状态做硬拦截。
- 前端可以做低风险提示，但不能阻止用户提交依赖后端裁决的场景；保存失败后以前端收到的后端错误为准。
- 如果需要字段级错误展示，后端应返回稳定字段路径，不使用展示文案作为字段标识。推荐将 `data` 约定为 `{ "fieldErrors": [{ "field": "name", "message": "路由名称已存在" }] }`，其中 `field` 使用请求 Proto 的 JSON 字段路径，例如 `name`、`rules[0].targets[0].upstreamID`。
- 字段级错误只增强前端定位能力，不改变后端 biz 是系统状态校验权威这一原则。

### Admin API Proto

- Proto 字段表达控制台产品契约，不把内部资源字段或临时实现细节泄漏为外部协议。
- Proto 和 `*.pb.go`、`*_http.pb.go` 使用 Buf `source_relative` 放在同一 API package；生成文件不得手工修改。
- 请求自身能判断的必填、格式、范围和跨字段约束在 service 层校验并转换；不得把 data、client 或 usecase 传给纯转换函数。
- API 中使用 `id` 表达不可变资源主键，使用 `name` 表达面向用户的展示名称。

### Admin API 资源标识

- 控制台创建资源时，由后端生成 UUID 作为不可变资源 ID，并写入底层资源的 `metadata.name`。
- Admin API 和前端协议字段统一使用 `id` 表达资源主键，不使用 `name` 表达不可变 ID。
- Biz 用例参数按领域命名，例如 `gatewayID`、`routeID`、`upstreamID`；不要把存储层的 `metadata.name` 命名泄漏到用例语义中。
- Repository 和 Kubernetes generated client 边界可以继续使用 `name`，因为这里对接的是 apiserver 存储协议。
- 不额外增加 `spec.id`；`metadata.name` 是底层资源主键，Admin API 的 `id` 是它在控制台产品语义中的映射。
- 面向用户展示和编辑的名称使用 `spec.displayName`，Admin API 管理的资源必须填写，不使用展示名称作为资源 ID 或跨资源引用键。
- 同一类资源内 `displayName` 必须唯一，避免控制台出现用户无法区分的重名资源。
- `displayName` 的唯一性校验属于系统状态校验，放在 biz 层，不放在 service 的请求转换中。
- 资源之间的引用使用资源 ID，不使用 `displayName`。
- 声明式 apiserver 可以保留调用方指定 `metadata.name` 的能力；Admin API 面向控制台体验，创建流程可以和声明式 API 不同。

### 治理策略与内置插件

- 当前已落地执行链路保留 `RateLimitPolicy` 和 `IPRestrictionPolicy`；删除 `PolicyBinding` 和 `RedisStore`，鉴权等治理能力后续重新设计后再加入。
- 核心治理能力可以在数据面通过内置插件执行，但用户协议和 ingate-admin-api 不能暴露为普通插件资源、插件绑定资源或插件私有 JSON。
- 内置治理插件由系统自动注入、自动配置并通过 Envoy xDS 配置生效；用户不需要独立安装插件，也不需要感知插件版本、phase、priority 或 Wasm 文件路径。
- `RateLimitPolicy` 和 `IPRestrictionPolicy` 通过 `targetRefs[]` 表达策略应用到哪些 Gateway 或 Route；策略配置和目标引用都由对应强类型 Policy 承载。
- Policy 的总体结果写入 `status.conditions`，每个目标的解析和生效结果写入 `status.targets[]`。缺失目标不影响其他有效目标继续发布；任一目标已生效时总体可视为已生效，部分生效和异常由 `status.targets[]` 表达；没有目标或所有目标都未接入流量时使用 `NotApplied`。
- 外部服务、证书等可复用运行依赖按真实产品需求独立建模；系统 Redis 是安装级基础组件，不进入用户资源协议。
- 内置治理插件可以参考 Higress 等项目的实现思路，但不能依赖第三方产品的 wrapper、matchRules 或高层配置协议；Redis hostcall 通过 Ingate 自己维护的最小 ABI adapter 隔离。
- 后续新增策略时按资源类型拆分 ingate-admin-api 的 service、biz 和 data 文件，不把不同策略堆进一个大 `policy` 文件，也不写进 Route/Gateway 的用例层。
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
- 涉及核心链路、跨层转换、协议适配和配置生效逻辑等关键或难懂的地方，要主动补充说明性注释。
- 注释要讲清楚“为什么这样做”和“配置如何生效”。
- 注释不需要以句号结尾，保持简洁即可。
- 英文专有名词可以保留英文，例如 Envoy、xDS、Gateway、Route、Upstream。
- 对外协议字段、错误文本、CLI 输出等用户可见字符串，按实际产品语境决定中英文，不受代码注释语言限制。

## Git 规则

- 提交要聚焦，一个提交只做一类事情。
- 不要从这个仓库修改旧项目 `../ingate`。
- 不要提交 `_output/`、`.gocache/`、`.gomodcache/` 或其它本地构建产物。
