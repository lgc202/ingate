# 0011：HTTPRoute / Backend 统一契约

- 日期：2026-10-10
- 状态：设计 review 通过，用户于 2026-10-11 授权提交阶段 B 当前实现；保留验证缺口
- 范围：继任 0001、0002、0005 中的 HTTP 资源名称及路由语义；其他生命周期和职责不变

## 已实现范围与来源

沿用 [资源契约](../design/ai-gateway-resource-contract.md)与[固定版本上游对照](../design/ai-gateway-upstream-alignment.md)，不导入参考项目的业务、未实现类型或新依赖。原生 Group 仍为 gateway.ingate.io/v1alpha1，资源仍为集群级；这是 Gateway API 语义的明确子集，不宣称完整兼容认证。

Gateway → HTTPRoute → Backend 构成当前 HTTP 主路径。HTTPRoute 用 parentRefs 绑定入口，各父引用分别接纳；缺省 sectionName 选择该 Gateway 所有合格 HTTP Listener，Accepted=True 只表示至少一个入口接纳。重复父引用及整个 Gateway 与其 Listener 的重叠引用在写入时拒绝。

规则名可选，规则和匹配保持声明顺序。缺省 matches/path、路径类型和值展开为 PathPrefix /；Exact 优先，其次最长 PathPrefix，平局使用跨资源创建时间、资源名称及同资源规则列表顺序。完全重叠的路由保留为候选，失效后端不会使低优先级目标抢占匹配。当前每条规则必须且只能引用一个 Backend，不开放 Header、Method、Query、Regex、filters 或多目标。

未知字段不是已支持条件。[Admin JSON 解码](../../internal/adminapi/server/codec.go)对 Gateway/HTTPRoute/Backend 的创建、更新使用局部严格 ProtoJSON，拒绝未知字段，避免把未实现的 Method/Header 条件丢弃后展开为全路径匹配。按框架的 JSON subtype 入口覆盖 application/json、protojson 及已有 text/json 等形式，拒绝格式错误的 Content-Type；不修改框架全局解码选项或账户接口；Gateway 创建/更新的严格解码由 0012 名称契约补齐。原生 API 沿用 Kubernetes 的 fieldValidation：默认 Warn 可能丢弃未知字段，要求严格拒绝的原生写入方必须指定 `fieldValidation=Strict`。该入口差异已由 [Admin 反例](../../internal/adminapi/server/crud_test.go)及[原生集成测试](../../internal/apiserver/integration_test.go)覆盖；不宣称任意传输、任意调用方都具有同样的未知字段策略。

缺失 Backend 的规则继续生成匹配，并以 Envoy directResponse 返回 500。父状态可以同时 Accepted=True、ResolvedRefs=False/BackendNotFound；引用恢复后重新生成转发动作。缺 Gateway 或合格 Listener 时 Accepted=False/NoMatchingParent。状态保留 observedGeneration、稳定的 lastTransitionTime 及其他控制器的记录；不新增逐规则状态。Backend 未使用不表示错误，仍不由 Controller 写 Accepted。

## 完整消费者路径

| 边界 | 当前入口 |
| --- | --- |
| 内外资源、默认值、校验 | [HTTPRoute](../../pkg/apis/gateway/v1alpha1/httproute.go)、[Backend](../../pkg/apis/gateway/v1alpha1/backend.go)、[defaults](../../pkg/apis/gateway/v1alpha1/defaults.go)、[validation](../../pkg/apis/gateway/validation/httproute.go) |
| 原生存储 | [Registry](../../internal/apiserver/registry/gateway/storage.go)：gateways、httproutes、backends 及各自 status |
| 裁决、编译和发布 | [Controller](../../internal/controllermanager/controller/gateway/controller.go)、[Gateway 裁决](../../internal/controllermanager/controller/gateway/gateway.go)、[HTTPRoute 绑定](../../internal/controllermanager/controller/gateway/httproute.go)、[Envoy](../../internal/controllermanager/dataplane/envoy/resources.go) |
| Admin HTTP/gRPC | [HTTPRoute proto](../../api/admin/v1/httproute.proto)、[Backend proto](../../api/admin/v1/backend.proto)：/api/admin/v1/http-routes、/api/admin/v1/backends |
| 读写分工 | [Repository](../../internal/adminapi/repository/httproute.go)：写 API Server，Get/List 只读 MySQL；Create/Update 返回权威保存结果 |
| 异步投影 | [Reconciler](../../internal/controllermanager/controller/projection/httproute.go)、[查询模型](../../internal/adminapi/repository/mysql/httproute.go)：HTTPRoute 单行 JSON 保存有序父引用、规则、匹配和后端引用；Gateway/Backend 仍使用有对象内唯一键的子表 |

完整 PUT 保留 metadata 并进行 resourceVersion 并发检查，Status 更新仍由投影追平。Delete 仍按名称删除，允许悬空引用，不增加 in-use 禁止、级联删除或读后写一致性回退。管理员账户、Redis Session、日志和版本功能不变。

旧 Route/Upstream 原生类型、Registry、生成客户端、Admin protobuf 和生产调用方已替换，不保留同义 alias 或兼容路径。数据面适配层的 dataplane.Route 是单个 RDS 匹配动作，不是旧的持久化 Route API。

## 数据和生成边界

[00002 资源表基线](../../internal/adminapi/migration/00002_create_gateway_resources.sql)直接定义 gateways、gateway_listeners、httproutes、backends、backend_endpoints。按新项目要求移除旧 Route/Upstream 表定义和 00003；HTTPRoute 的四类有序子表被 JSON 数组替代。没有操作任何已有数据库或旧 etcd 数据；应用过旧基线的数据库需要单独重建，不能重跑已应用的 Goose 版本来更新 Schema。名称、搜索和查询模型修正见 [0012](0012-resource-names-and-query-schema.md)。

API 和 protobuf 仍使用仓库现有生成器；旧 protobuf 机械产物按明确文件路径移除，Kubernetes client-gen 按自己的生成标记清理旧客户端。生成文件不手改，移除的源码与产物可通过 Git 基线恢复。go.mod、go.sum 和 buf.lock 不变。

## 阶段 B 最初审查与验证记录

以下保留初次审查的历史结果，不代表 0012 修正后的最终代码；本轮规则来源、审查与执行结果以 [0012](0012-resource-names-and-query-schema.md) 为准。

本阶段按 AGENTS 同时核对已跟踪差异与未跟踪文件，逐个审查手写代码及移除的旧实现：类型/方法/辅助函数顺序、职责、错误路径、边界复制、注释和正反例。范围包括 proto、内外 API 类型、默认值/校验、Registry、Controller/Envoy、Admin 各层、投影及迁移、装配与测试；`pkg/generated/clientset/versioned/clientset_test.go` 是手写测试，未按生成目录排除。生成文件只核对源契约与可重复性，不按手写代码逐行评审。

GoCanon MCP 当前不可调用，使用本地规则源 `/Users/lgc202/workspace/source/lgc202/gocanon`。实施期间基线为 e4f88af7b385cf13d58fa55a5b7856971f60f997，收尾审查重新核对为 6f86c253fa7f983d402f5db24edd761b4bcc22bb；最后核对时工作树有 `.gitignore` 和生成器 `.gitignore.tmpl` 两处修改，以及未跟踪的 `docs/decisions/0065-portable-local-mcp.md`，后者未作为本阶段依据。已完整重读最新开发入口、AGENTS 和规范索引，并核对两版本差异；本阶段使用的核心规范未变。规范仍为草案，作为人工审查依据；目标 AGENTS 与已经确认的资源契约优先，不把 HEAD 当作 dirty 工作树的完整版本。

按任务选择 Go 命名/函数（尤其 GO-FUNC-012/013/014）、防御、错误、测试、Kratos、Repository、参数校验、APIServer 和 Controller 规范。使用 Go 代码审查和测试 skill 辅助检查，没有借参考实现新增业务、抽象、依赖或兼容路径。

### 审查结论与已处理项

- Must Fix：新增 Admin 测试有两处遗漏 seed 参数，首次收尾重跑因而编译失败；已修正调用并显式支持空 seed，定向及最终全仓库测试通过。
- Must Fix：严格解码原先只识别两个精确 MIME 类型；text/json 或格式错误但仍被框架识别为 JSON 的 Content-Type 可走宽松解码。使用 protobuf 原字段名的反例复现了未知 method 被丢弃、返回 200 并展开成 PathPrefix /；已按 JSON subtype 覆盖并拒绝无效 Content-Type。回归测试已确认旧分支失败、修正后拒绝且无 API 写入，同时验证三种有效 JSON 入口的正常输入仍成功。
- Should Fix：原有顺序测试使用同名资源，且反转输入后再编译持有输入引用的旧结果，不能可靠证明输入排列无关；已改用唯一资源名，反转前保存编译结果。补强多父接纳/拒绝、其他控制器记录保留、旧父清除、generation/transitionTime、无额外状态写入，以及批量查询的子集合顺序、孤立匹配拒绝、子写入失败回滚断言。fake 客户端初始化会写入测试对象的 resourceVersion，输入不变的快照在初始化完成后获取，避免把测试设施修改误报为实现修改。
- Nits：HTTPRoute/Backend REST 类型集中在构造函数之前，matcher 在其辅助函数之前；投影测试辅助函数按调用顺序排列，移除查询测试中无意义的常量字符串拼接。未改变运行行为。

以上问题已处理，未留下已确认的阶段 B 代码阻塞项；这是本次差异审查结论，不是全仓库未来不会违规的保证。

### 最终执行记录（2026-10-10）

Go 为 1.27.0，golangci-lint 为 2.13.2。Go 检查使用 `GOMAXPROCS=2`、独立临时 `GOCACHE` 与 `-p 2` 限制负载；lint 使用独立缓存。普通测试中的本地 gRPC、xDS 和 miniredis 只使用测试自有的回环端口。未清理共享 Go 缓存。

| 检查 | 最终结果 | 证明范围 |
| --- | --- | --- |
| `gofmt -l cmd internal pkg` | 通过，无输出 | 手写及生成 Go 格式 |
| `git diff --check`，另逐个对 55 个未跟踪文件执行 `git diff --no-index --check /dev/null <file>` | 通过，无空白问题 | 不遗漏新增文件；未跟踪目录已展开到文件 |
| `golangci-lint run ./...` | 通过，0 issues | 当前配置的静态规则，不替代语义审查 |
| `go vet -p 2 ./...` | 通过 | 普通构建标签的 Vet 检查 |
| `go test -p 2 ./... -count=1` | 通过 | 冻结后的最终普通测试；此前失败不冒充通过 |
| `go build -p 2 ./...` | 通过，exit 0 | 全仓库编译；沙箱曾提示 module stat cache 写权限受限，但构建成功 |
| `go test -p 2 -tags=integration ./internal/apiserver -count=1` | 通过 | 隔离 etcd + HTTPS：CRUD、筛选正反例、Watch、CAS、Spec/Status 隔离、缺省值、重叠父引用与 Strict 未知字段拒绝 |
| `buf generate`、`bash scripts/generate-api.sh` | 通过，36 个生成文件的路径及 SHA-256 前后完全一致 | 已安装 Buf 1.73.0 与 Makefile 固定版本一致；插件与 Kubernetes 生成器仍沿用仓库固定版本，未改生成文件 |
| `buf lint` | 通过 | protobuf 契约静态检查 |
| 本地文档链接、代码围栏与 SVG XML | 通过 | 本次 9 份 Markdown 的 105 处本地文件链接无断链、围栏成对；5 张 SVG 结构有效。另经独立文档读者复核当前/历史/未来边界；当前架构 SVG 已实际渲染并检查布局 |

此前收尾的并行检查与生成因运行时间异常被中断，没有可确认的最终结果，本记录不引用它们作为成功证据；本次已重新完成上述必需检查。此前竞态检查同样中断，本次未重跑 `-race`，不引用更早历史记录替代最终版本的竞态证据。文档本地链接、结构和 SVG 渲染另行核对；图示不证明真实转发。

单元测试可证明生成了保持优先级的 500 动作，不能证明真实 Envoy 已安装该配置并返回 500。API integration 隔离了认证、授权和 Admission，不能证明部署权限正确。sqlmock 不证明 MySQL 方言、迁移、事务与故障恢复；没有启动真实 MySQL、Envoy、Manager 联动或 Redis 拓扑。

## 后续阶段

阶段 C 先确认受控环境、真实 HTTP 主链路和 TLS 契约；再按已选模型/协议推进阶段 D、E。Go AI Proxy、凭据、运行视图、加载/ACK/激活、模型转换和 MCP 均未在本阶段引入。当前代码不作生产就绪或模板级合规声明；用户代码 review 通过前不提交。
