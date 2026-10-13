# 0012：资源名称与查询投影修正

- 日期：2026-10-10
- 状态：产品契约已确认，`31a711be` 已提交；用户于 2026-10-11 授权提交本轮回归补齐与文档调整，真实组件测试留到后续统一测试阶段
- 范围：Gateway、HTTPRoute、Backend 的管理契约及 MySQL 查询模型

2026-10-13 更新：本文保留名称与投影修正的历史快照。集群级作用域及仅 name 唯一的投影目标由 [0013](0013-namespaced-resources.md)替代；name、displayName 和不额外投影 UID 的契约不变。Namespace 基础设施已提交，本轮业务存储、管理和数据库投影切换待 review，后续状态以 0013 为准。

2026-10-11 提交前删除 6 个旧 Route/Upstream 空目录；只移除目录本身，没有删除文件或运行数据。重新执行格式、已跟踪与未跟踪文件空白检查、整仓 lint、vet、test、build 和 API Server 集成测试，全部通过；在隔离副本中重新生成，36 个生成文件与工作区一致。集成测试使用临时 etcd 和 HTTPS，未启用认证授权。下文各阶段的“等待 review”与“未提交”保留为当时的执行记录，本次授权不表示真实 MySQL、Envoy 和 Redis 故障转移已验收。

## 名称契约

Admin 创建请求只接收展示名称和资源配置，不接收技术名称。服务端生成不可修改的 `metadata.name`；资源引用、查询和修改始终使用这个名称。原生声明式 API 继续保留 Kubernetes 的 name / generateName 机制，不改为 UID 寻址。

管理服务使用标准库 `crypto/rand.Text` 生成随机后缀，转为小写后加资源种类前缀，满足现有 DNS 名称约束；唯一性仍由 API Server 原子创建裁决，不做先查后写。每次创建产生新资源，不按展示名称去重：创建响应丢失后重新请求可能产生另一个资源。本次不引入幂等令牌或自动重试机制。

三种资源都有可修改的 `spec.displayName`，允许重复，不做阻断式重名查询，也不建立展示名称唯一索引。管理响应中的 `name` 就是后台生成的资源 ID，供界面按需展示、复制和辨认。它不是 Kubernetes 的 `metadata.uid`；后者仍由 API Server 管理，但不额外投影到 MySQL 或加入管理响应。界面应同时展示名称及适用的资源摘要，不要求用户输入或记忆技术标识。本仓库没有前端，本次不宣称界面已经实现。

当前资源仍为集群级，不新增 Namespace、租户或权限模型。名称规则不改变 HTTP 匹配、引用解析、端点权重、resourceVersion 更新检查及 MySQL 异步查询的一致性边界。

## 提交后的 MySQL 回归补齐（2026-10-11，用户已授权提交）

用户同意先修复两个已识别的测试缺口，再提出真实 MySQL/Envoy 的隔离验证方案。生产查询逻辑原本正确，本次不改资源、数据库或对外行为；只在 HTTPRoute 的 Count 与取页阶段之间补语义分段，并展开相关测试的密集配置。

用户随后明确当前开发阶段不安排这类实库与真实数据面测试，留待后续统一执行。本轮已补齐的回归测试和排版整理保留待 review；隔离验证方案仅作备查，不实施 Compose 环境、联动 fixture 或 ACK/NACK 测试工程。当前继续已确认契约范围的开发，日常单测、静态检查与提交前 review 不取消，也不将真实组件的未验证状态改为已通过。

Gateway/Backend 列表用例包含三个主对象，其中两个有各自子记录，一个没有子记录；主对象按名称排序，子记录按父 ID 排序，顺序故意不同。测试核对整个返回页、分页参数和总数，不只检查子记录数量。GetGateway 子查询错误用例核对包装错误、回滚和零值返回，防止交付半成品。

测试侧连接池记录器将事务操作继续交给原 sqlmock 连接，只复制实际 `BeginTx` 参数。GetGateway、ListGateways、GetBackend、ListBackends、ListHTTPRoutes 五个入口均检查只执行一次只读 Repeatable Read 事务，未添加生产测试接口或新依赖。

| 本次检查 | 结果与边界 |
| --- | --- |
| 子记录归属故障注入 | 临时 overlay 把 Gateway/Backend 的归属索引都改为 0，两项列表测试均失败 |
| 只读选项故障注入 | 单独把 ReadOnly 改为 false，五个读取入口的测试均失败 |
| 隔离级别故障注入 | 单独改为 Read Committed，五个读取入口的测试均失败 |
| MySQL 包普通测试与 race | 真实仓库实现均通过；这里使用 sqlmock，不是实库验收 |
| 整仓格式、lint、vet、普通测试、build | 均通过，lint 为 0 issues；构建有共享模块 stat cache 写权限警告，退出码为 0 |

故障注入仅写入临时源码 overlay，没有将错误实现写入工作区。按 GO-FUNC-012 及 go-code-review 逐文件审查声明顺序、职责、命名、错误与断言；go-testing 用于补有效正反例，go-functions/go-style-core 用于签名与语义分段。文档按 technical-writing 区分已验证证据与拟执行验收。规则来源与上述 GoCanon MCP 快照相同。

已跟踪差异和新增方案分别执行空白检查。go-documentation 的通用单文件脚本将 Test 函数和测试文件没有独立包注释报告为缺失；人工确认包注释仍在 store.go，测试函数按仓库惯例不重复写名称注释，新增 BeginTx 已有职责说明。这些报告不作为新增代码缺陷，也不将该脚本表述为通过。

本次未改 API 类型或 API Server 存储契约，没有重新生成代码或重跑 API Server 存储集成测试。未启动容器、真实 MySQL 或 Envoy，没有操作已有数据；真实 Redis 故障转移仍未验证。方案见 [HTTP 主链路隔离验证](../design/http-gateway-validation-plan.md)。用户于 2026-10-11 确认提交并继续，提交前重新审查全部 8 个手写变更文件并执行仓库要求的日常检查；后面的待 review 与测试缺口描述保留为此前阶段的历史记录。

## 投影表与查询

项目没有历史兼容要求，直接修正尚未发布的 00002 资源表基线，删除 00003 和旧 Route/Upstream 表定义，不新增升级迁移或自动清库逻辑。已经应用旧 Schema 的数据库不能靠重跑修改后的迁移修复；运行环境的重建需要单独执行，本次不操作任何已有数据库或 etcd 数据。

HTTPRoute 主表保存稳定名称、展示名称、版本、时间，以及 parentRefs / rules 的 JSON 数组。顺序由数组表达，不再使用四类子表、position 或 rule_index；匹配优先级仍依赖原声明顺序。GORM 使用已有 serializer:json，不增加 JSON 驱动或自制序列化框架。

Gateway Listener 与 Backend Endpoint 有独立的对象内键，保留关系表及原有组合唯一索引。全部表和列补中文注释。名称唯一索引只镜像资源身份，不用于裁决 API Server 写入。不为展示名称包含匹配添加没有查询收益依据的普通或前缀索引。

管理列表保留页码分页，参数 `display_name` 按展示名称包含匹配，空值不筛选；例如输入“订单”能找到“生产订单网关”。HTTPRoute 额外提供按 Gateway 精确筛选。搜索把 `%`、`_` 和转义符当作用户文字，不允许它们扩大匹配范围。Count 与当前页必须应用同一筛选条件。父引用使用 MySQL JSON_CONTAINS 检查，不用字符串子串匹配；本次不承诺这些筛选的索引性能，不增加规则、路径或任意 JSON 查询语言。

### 字段与注释的职责

表注释表达业务对象，字段注释表达数据含义；数据来源、可编辑规则、同步流程和一致性边界集中在本节说明，不逐列重复架构解释。

- `id` 是 MySQL 主键，子表的 `gateway_id`、`backend_id` 引用对应的主键，不是 API 资源标识。
- `name` 保存 API 资源的唯一标识，管理端由后台生成；`display_name` 保存用户可编辑的名称，不作为引用键。
- `version`（MySQL 模型字段为 `Version`）保存最近同步的 API 资源版本，作为管理更新请求的并发校验令牌；值按不透明字符串处理，不用于比较跨对象的先后顺序。API Server 原生对象仍使用标准字段 `ResourceVersion`，投影时显式映射，不由 MySQL 生成或递增。
- `created_at` 保存资源在 API Server 中的创建时间，不是 MySQL 插入时间。`updated_at` 保存 MySQL 记录最后写入的时间，首次投影和 Status 同步也会设置它，不能当作用户修改资源配置的时间。
- `parents` 和 `rules` 是 HTTPRoute 的有序配置；JSON 数组顺序参与恢复原声明，不由规则名称或数据库行顺序决定。

名称、引用键的 VARCHAR 长度沿用声明式 API 的现有约束；展示名称没有已确认的长度上限，保留现有 Gateway 的 LONGTEXT 表示，不仅在数据库层增加隐含长度限制。组合唯一索引同时约束 Listener 名称、Endpoint 地址与端口，并支持所属资源的读取；不为没有明确用途的字段添加索引。

## 修复依据与验证

原 HTTPRoute 五表只为还原嵌套配置，生产读取需要四次子查询、两级位置映射及跨表替换事务。改为单行 JSON 投影后，移除孤立子记录和顺序拼装路径；通过多规则、多匹配的逆字母顺序用例验证顺序不变，并覆盖 JSON 解码失败不返回半成品。

行为变更限于已确认的名称契约和查询条件。结构调整使用 refactor 技能的最小变更原则；GoCanon 草案用于人工审查，不据此创造产品约束。执行来源为 GoCanon MCP 启动快照，规则源 `/Users/lgc202/workspace/source/lgc202/gocanon`，revision `6f86c253fa7f983d402f5db24edd761b4bcc22bb`，dirty=true。

### 手写代码审查

2026-10-10 的审查同时核对已跟踪差异、删除文件与当时的 55 个未跟踪文件，生成产物按固定契约和可重复性单独检查。手写代码按 GO-FUNC-012 检查类型、入口、接收者方法和辅助函数的排列，再核对职责、命名、转换、错误路径和注释；没有添加猜测语义的扫描器。

- Must Fix（已处理）：移除旧资源表基线与 00003，取消 HTTPRoute 仅为恢复数组而设置的子表和位置字段；补齐后台生成标识、可重复展示名称及完整 PUT 的传递链路。撤掉额外 UID 投影、展示名称前缀参数和没有查询收益依据的展示名称索引。
- Should Fix（已处理）：Count 与分页查询应用同一筛选条件，输入中的通配符按文字转义；增加不匹配输入、重复展示名称、改名保留身份、JSON 写入失败回滚及解码失败不返回半成品的断言。多行 JSON 查询验证不同路由的配置不会混在一起，逆字母顺序的父引用、规则和匹配验证不被重新排序。
- Nits（已处理）：统一数据库注释为字段含义，修正英文代码注释与辅助类型的排列；Status 消息仅报告 Backend 引用不存在，不宣称真实请求已经返回 500。同步当前设计、完成度文档和历史记录的适用边界。

2026-10-11 用户确认修复全仓库审查发现的问题，同时要求整理手写代码的声明顺序、换行和语义分段。资源类型边界按 REPO-004 与 GoCanon 已接受 ADR 0042 对齐：Repository 的 Create/Update 接收版本化资源；产品视图与分页结果定义在 Repository 根包；Service 不重复声明分页结果。删除 Domain 中的 Gateway、HTTPRoute、Backend 及镜像子类型，子配置复用版本化 API 类型。写入前和返回视图时复制可变配置，HTTPRoute 缺省值不修改调用方输入。账号与会话仍属于 Domain，公开 protobuf、名称、引用、查询一致性和更新冲突契约不变。

2026-10-11 用户确认产品查询类型统一命名为 `repository.GatewayInfo`、`repository.HTTPRouteInfo`、`repository.BackendInfo`，同时用于管理查询与创建、更新响应。Repository 门面仍使用 `Gateway`、`HTTPRoute`、`Backend`，分页结果仍使用对应的 `Page` 类型。GoCanon ADR 0042 的参考名称采用 `GatewayView`；本项目按已确认的管理端语境使用 `Info`，保持该 ADR 的类型边界，不复制 Service 结果，不保留旧名称别名。此次只改变内部 Go 类型名称与注释，字段、公开 API、数据库和行为均不变。

重命名后逐文件核对 10 个手写 Go 文件的声明、返回类型、转换和测试引用，并重新执行格式、已跟踪与 53 个未跟踪文件空白检查、整仓 lint、vet、test、build，全部通过。API 类型、protobuf 和存储契约未变，本次没有重新生成代码，也没有重跑 race 或 API Server 集成测试；下文保留的是此前完整修复的执行记录，真实组件验证缺口仍然存在。代码未提交，等待用户 review。

### MySQL 版本字段改名（2026-10-11）

用户确认三张资源主表使用 `version`，MySQL 模型使用 `Version`；API Server 原生 `ResourceVersion` 保持不变，在投影边界显式映射。同步修改写入 SQL、读取映射、SQL mock 期望和历史文档的适用说明。字段仍保存不透明字符串，版本值、公开 API 和更新冲突契约不变，不增加别名、旧列兼容或依赖；只修正未发布的 00002 基线，没有执行运行环境的数据迁移。

本次逐文件复核 14 个手写 Go 文件、1 个 SQL 文件和 2 个决策文档的新增差异，声明顺序与职责没有改变，错误检查保持相邻。按 go-naming 区分本地字段与来源对象的标准字段，按 go-testing 补充三种资源读取响应的版本断言。GoCanon MCP 规则来源、revision 和 dirty 状态与上述记录一致。格式、已跟踪及未跟踪文件空白检查、整仓 lint、vet、test、build 均通过；临时 overlay 丢弃 MySQL 读取映射的版本后，三种资源的 HTTP 查询测试均按预期失败，错误实现没有写入仓库。

本次没有改变 API 类型或 API Server 存储契约，未重新生成代码，也未重跑 API Server 集成测试。真实 MySQL 迁移和运行验证仍未执行；上一轮审查指出的多资源子记录归属与事务选项测试缺口，不属于本次改名修复，仍待处理。代码未提交，等待用户 review。

### 全仓库审查修复（2026-10-11）

Snapshot 引用校验原先把 500 直返路由当作转发路由，因空 Cluster 名称拒绝整份配置。本次只让非转发动作跳过 Cluster 引用检查，实际转发动作仍要求后端存在。新增 Publisher 测试验证直返配置确实进入 Snapshot、状态为 500，以及后端恢复后替换为转发；不以转换函数生成动作替代发布验证。

真实 RequireSession 增加 HTTP Transport 下的有效会话、缺少或伪造 Token、过期会话、改密版本撤销和 Redis 故障测试，同时检查身份注入及失败时下游调用次数为零。SQL 使用 sqlmock，Redis 使用测试专有的 miniredis；不据此声称真实 Redis 故障转移已验证。CI 的 protobuf 漂移检查加入 Controller Manager 配置目录。

手写代码整理包括资源查询视图、参数转换、Controller 父引用附着、Status 组装、投影及相关测试。密集的多字段配置展开，多行签名逐参数排列；错误赋值和检查保持相邻，查询、转换、状态更新等阶段之间分段。父引用附着从资源索引与排序中分离，移除同类型字段之间的无意义逐项复制，不引入通用 CRUD 或自定义语义扫描器。

修复仍留在工作区等待 review，不自动提交。没有增加依赖或管理接口。删除的重复 Domain 资源类型可从 Git 历史或本轮临时源码副本恢复；没有删除或迁移运行环境的数据。

### 修复后执行记录（2026-10-11）

Go 与 golangci-lint 版本、GoCanon 规则快照均与上述记录一致。修复完成后重新核对 `git status --short`、差异文件与未跟踪文件，并复核手写代码的声明顺序、职责、注释和语义分段；生成文件不按手写代码逐行审查。

| 检查 | 结果 | 证明范围 |
| --- | --- | --- |
| `gofmt -l cmd internal pkg api` | 通过，无输出 | 最终代码格式 |
| `git diff --check`，另对 53 个未跟踪文件执行 `git diff --no-index --check` | 通过 | 已跟踪与新增文件的空白检查，不代替语义审查 |
| `golangci-lint run ./...`、`go vet ./...`、`go build ./...` | 通过，lint 为 0 issues | 已配置静态检查及全仓库编译 |
| `go test ./... -count=1`、`go test -race ./... -count=1` | 通过 | 普通测试及其执行路径的竞态检测；不覆盖未执行的真实组件路径 |
| `go test -tags=integration ./internal/apiserver -count=1` | 通过 | 临时 etcd + HTTPS 存储契约；测试未启用认证授权，不视为权限验收 |
| 固定版本 `buf generate` 与 `bash scripts/generate-api.sh` | 通过 | 在隔离源码副本中连续生成两次，36 个生成文件均与工作区逐字节一致 |
| `buf lint`、本地文档链接检查 | 通过，119 个本地文件链接可达 | protobuf 静态检查与文件存在性，不检查页面锚点或外部链接 |
| Publisher 旧实现故障注入 | 新增测试按预期失败 | 捕获 500 直返被当作空 Cluster 引用而拒绝的原始缺陷 |
| RequireSession 认证绕过故障注入 | 新增测试按预期失败 | 捕获身份未注入，以及伪造、过期、撤销会话与依赖故障时下游仍执行的退化 |

两次故障注入都使用临时源码 overlay，没有把错误实现写入仓库；真实实现下相关测试通过。Go 构建使用临时 GOCACHE，生成使用已有本地模块下载缓存；测试经批准开放本机回环监听。没有清理共享缓存，没有修改 go.mod、go.sum 或 buf.lock，没有操作已有 MySQL、Redis 或 etcd 数据。

真实 MySQL 的 JSON 查询、事务与索引，真实 Manager/Envoy 联动，以及 Redis TLS 和真实故障转移仍未执行。新增中间件测试使用 sqlmock 与 miniredis；普通 Redis 测试和竞态检测通过，不等于这些真实组件能力已验收。远端 CI 也未运行，不宣称模板级或生产就绪。

### 本轮执行记录（2026-10-10）

Go 为 1.27.0，golangci-lint 为 2.13.2。以下检查针对本轮未提交代码，不引用 0011 的历史结果冒充本轮验证。

| 检查 | 结果 | 证明范围 |
| --- | --- | --- |
| `gofmt -l cmd internal pkg` | 通过，无输出 | Go 格式 |
| `git diff --check`，另逐个对 55 个未跟踪文件执行 `git diff --no-index --check` | 通过 | 已跟踪与新增文件的空白检查 |
| `golangci-lint run ./...` | 通过，0 issues | 已配置静态规则 |
| `go vet ./...` | 通过 | Vet 覆盖的检查 |
| `go test ./... -count=1` | 通过 | 普通单元、HTTP/gRPC、SQL mock 和控制器测试 |
| `go build ./...` | 通过 | 全仓库编译 |
| `go test -tags=integration ./internal/apiserver -count=1` | 通过 | 隔离 etcd + HTTPS：存储、缺省值、筛选、Watch、CAS、重复展示名称及更新身份保留；未验收认证授权 |
| `buf generate`、`bash scripts/generate-api.sh` | 通过，两次生成的 36 个文件路径及 SHA-256 一致 | 固定版本生成器与契约一致，生成文件没有手改 |
| `buf lint`、本地文档链接检查 | 通过 | protobuf 静态检查与文件链接可达，不替代语义审查 |
| 搜索退化验证 | 临时退回前缀匹配时相关测试失败，恢复包含匹配后通过 | 断言能抓住本次查询语义退化；不证明真实 MySQL 的执行结果 |

最初生成时，网络代理 DNS 查询失败，共享 Go 编译缓存也存在缺失条目；最终使用本地模块下载缓存作为 GOPROXY，并为 Kubernetes 生成器设置独立临时 GOCACHE，完成固定版本生成与重复性核对。没有清理共享缓存，没有修改 go.mod、go.sum 或 buf.lock。

sqlmock 只能证明 SQL 组装、字段传递和错误路径，不证明真实 MySQL 的 JSON 查询、事务或索引效果。本机 PATH 未发现 mysql/mysqld 可执行文件，Docker daemon 未运行，没有已确认可丢弃的 MySQL 测试环境；本轮未启动真实 MySQL、重建数据库、运行真实 Manager/Envoy 联动或重跑 Redis 拓扑与竞态检查。不把这些缺口表述为已经验收，也不作生产就绪声明。
