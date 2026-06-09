# Ingate Next Agent 工作约定

这个仓库是 Ingate 的全新重写，不应默认沿用旧 `../ingate` 的命名、目录结构或实现习惯。

## 项目方向

- 构建一个面向 API 网关、AI 网关和多运行时 target 的声明式控制面。
- 核心抽象优先表达网关领域语义，而不是某个具体数据面实现。
- Envoy xDS 只是第一个 target，不是核心模型本身。
- `RuntimeGroup` 是一等资源，表示一组数据面运行时的逻辑投放单元；Gateway 只引用 RuntimeGroup ID，不在 Gateway service 中硬编码运行组选项。
- 命名要按新设计重新判断，不要被旧项目影响。例如使用 `Upstream`，不要使用 `Backend`。

## 当前范围

第一个实现里程碑已经完成核心编译链路：

```text
Resource -> Compiler -> Logical IR -> Target Translator -> RuntimeSnapshot
```

当前阶段开始划分长期服务边界，但只做必要入口，不提前实现临时 store、复杂 controller、真实 xDS 协议或 AI runtime。

第一批服务边界：

- `ingate`：CLI 和本地调试入口
- `ingate-admin-api`：前端管理 API
- `ingate-apiserver`：声明式资源 API
- `ingate-controller`：资源状态收敛
- `ingate-xds`：Envoy xDS 配置服务

暂时不加入：

- etcd
- plugin runtime
- AI runtime
- data-plane agent
- Kubernetes operator

## 内置治理插件

- 限流、鉴权、访问控制、AI token 配额这类核心治理能力可以使用数据面插件执行，但控制面产品模型必须保持强类型资源，不让用户直接编辑插件私有 JSON。
- 内置治理插件不建模为用户创建的通用插件资源或插件绑定资源；用户配置的是对应的 Policy、PolicyBinding 和必要的依赖资源。
- xDS 对内置治理插件采用长期形态：Listener / HCM 注入一次内置 Wasm filter，filter 配置携带 target translator 生成的可执行策略索引，插件通过当前 xDS route name 定位 route/rule 配置。
- 内置插件随 Ingate 数据面镜像或安装包发布，默认放在 `/opt/ingate/plugins`；`/var/lib/ingate/plugins` 预留给未来动态下载、缓存或用户安装的外部插件。
- 用户自定义插件仍走普通插件模型，不和内置治理插件混用同一套产品协议。

## Go 版本

- 使用 Go 1.26。
- 完成改动前运行 `make test` 和 `make build`。

## 编码规范

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

- 当前已落地执行链路保留 `RateLimitPolicy`、`AccessControlPolicy`、`PolicyBinding` 和 `RedisStore`；鉴权等治理能力后续重新设计后再加入。
- 核心治理能力可以在数据面通过内置插件执行，但用户协议和 admin-api 不能暴露为普通插件资源、插件绑定资源或插件私有 JSON。
- 内置治理插件由系统自动注入、自动配置、随 `RuntimeSnapshot` 下发；用户不需要独立安装插件，也不需要感知插件版本、phase、priority 或 Wasm 文件路径。
- `PolicyBinding` 只表达策略绑定到哪个资源或规则，不承载策略配置本身；策略配置必须放在对应强类型 Policy 资源中。
- Redis、外部服务、证书等可复用运行依赖应独立建模，例如 `RedisStore`，策略通过 ID 引用，不把连接信息复制到每条策略规则里。
- 内置治理插件可以参考 Higress 等项目的实现思路，但不能依赖第三方产品的 wrapper、matchRules 或专属 host function 作为 Ingate 长期运行时边界。
- 后续新增策略时按资源类型拆分 admin-api 的 handler、dto、service、store，不把不同策略堆进一个大 `policy` 文件，也不写进 Route/Gateway 的用例层。
- 编译链路中，compiler 负责解析强类型策略和绑定关系，target translator 负责生成目标运行时配置；compiler 不生成插件私有 JSON。

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

- 行为变更必须配套测试。
- 测试优先覆盖领域行为，而不是为了覆盖率测试无意义的转发函数。
- 不要为了让测试好 mock 而污染生产代码设计。
- 能直接构造具体类型测试时，不要引入 mock 或接口。

### 注释

- 代码注释使用中文，包括 package comment、导出类型注释、导出函数注释和必要的实现说明。
- 注释要解释领域含义、设计约束或不明显的原因，不要复述代码本身。
- 涉及核心链路、跨层转换、协议适配、AI 网关配置生效逻辑等关键或难懂的地方，要主动补充说明性注释。
- 注释要讲清楚“为什么这样做”和“配置如何生效”，尤其要说明普通网关配置与 AI 网关配置的差异。
- 注释不需要以句号结尾，保持简洁即可。
- 英文专有名词可以保留英文，例如 Envoy、xDS、Gateway、Route、Upstream、RuntimeSnapshot。
- 对外协议字段、错误文本、CLI 输出等用户可见字符串，按实际产品语境决定中英文，不受代码注释语言限制。

## Git 规则

- 提交要聚焦，一个提交只做一类事情。
- 不要从这个仓库修改旧项目 `../ingate`。
- 不要提交 `_output/`、`.gocache/`、`.gomodcache/` 或其它本地构建产物。
