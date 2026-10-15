# 0015：AI Proxy 应用分层与能力包布局

- 状态：布局已接受并提交；2026-10-15 用户完成原生 Client 重构、modelproxy 命名及 HTTP 装配的 review，授权本地提交
- 日期：2026-10-15

## 背景

AI Proxy 的配置消费已经提交，请求体解析与模型覆盖仍在本地开发。原布局将 aiconfig 和 OpenAI 请求处理直接放在组件根目录；未提交的 OpenAI 请求对象还同时承担协议字段、入口校验和出站编码。继续接入服务和更多协议时，需要区分应用编排与这些能力的所有权。

官方 Agent Router 固定提交 `daa9f891a8afcb18576d4593ad18870d1d18abac` 将协议类型、入口操作、协议适配和后端鉴权分别放在 [apischema](https://github.com/theagentrouter/agent-router/tree/daa9f891a8afcb18576d4593ad18870d1d18abac/internal/apischema)、[endpointspec](https://github.com/theagentrouter/agent-router/blob/daa9f891a8afcb18576d4593ad18870d1d18abac/internal/endpointspec/endpointspec.go)、[translator](https://github.com/theagentrouter/agent-router/tree/daa9f891a8afcb18576d4593ad18870d1d18abac/internal/translator) 和 [backendauth](https://github.com/theagentrouter/agent-router/blob/daa9f891a8afcb18576d4593ad18870d1d18abac/internal/backendauth/auth.go)。Ingate 借鉴职责划分，保留已确认的统一 HTTP Proxy，不移植 ExtProc 或完整转换框架。

## 决策

`internal/aiproxy` 根目录保留组件组合根及 conf、server、handler、service、repository 等实际需要的应用分层。组件自身的配置消费和协议能力统一放在 `internal/aiproxy/pkg/<capability>`；这里的 pkg 是组件私有实现的目录分组，不表示对外发布或全仓库共享。这是 Ingate 的已确认目录选择，不作为 GoCanon 对所有组件的通用要求。

```text
internal/
├── aiproxy/
│   ├── app.go                # 显式装配原生 Client、Service、Handler 和 HTTP Server
│   ├── conf/                 # 进程启动配置 protobuf，不含动态路由
│   ├── server/               # 原生 HTTP 构造、Unix 入口 Header 适配、路由与 ErrorEncoder
│   ├── handler/              # 可信入口读取、协议错误和响应流写回
│   ├── service/
│   │   └── modelproxy/       # 解析、选路、适配及 Envoy HTTP 调用
│   └── pkg/
│       ├── aiconfig/          # ConfigMap 加载、只读配置发布与查询
│       ├── apischema/
│       │   └── openai/       # 当前实际使用的 OpenAI 协议字段
│       ├── endpointspec/     # 客户端操作的请求解析与校验
│       ├── envoyclient/      # 固定 Envoy UDS 的原生 HTTP Client 构造
│       └── translator/       # 当前 OpenAI 到 OpenAI 的模型覆盖
└── pkg/
    └── aidataplane/           # Controller 和 Proxy 共用的编译契约
```

上图只列当前实际实现的包；模型 Service 与 HTTP Handler 的原切片已经提交，当前 service/modelproxy 及 ModelProxyHandler 命名随请求链重构已通过 review，并获准本地提交。不预建 conf、repository、backendauth、其它协议或空接口；server 随真实对象装配加入。Repository 只承接实际存储访问，不为完整套用 CRUD 分层而包一层透传。

Service 与 Handler 已提交为 `81d3e027`，此前的 ReverseProxy 实现与 Envoy 出口已提交为 `350de244`。用户随后要求完整用例进入 Service，当前删除旧 pkg/modelproxy 包及目标 Context 交接，不保留兼容入口。Handler 只依赖 Service，仍用 Context 接收可信入口；选中后端、出站路径、Header 和正文直接在 Service 内构造，再调用原生 Client.Do。envoyclient 只构造受限连接池，不包含模型选择、公开错误编码或凭据管理。

HTTP 装配切片已通过 review，并获准本地提交：[NewHTTPServer](../../internal/aiproxy/app.go) 创建固定 UDS 的原生 Client，注入 modelproxy.Service，再创建 ModelProxyHandler 并交给 server.New 注册 Chat Completions。Server 只接收已有 Unix Listener，不增加默认 TCP 入口。Listener、Loader 和内部 Client 由成功构造链保证有效，不新增逐层 nil 检查及相应伪失败测试；Server.New 只装配和注册，直接返回原生 Server。外部 socket 路径和正文大小约束仍由构造边界检查。本步还没有 Run、二进制、配置文件或可信入口适配，不能把装配函数等同于已经可部署。

依赖从应用编排流向能力包；能力包不依赖组件的 conf、Handler、Service、Repository 或 Controller 实现。pkg 目录本身不增加 Go 的编译期可见性限制，组件所有权与依赖方向仍须通过实际 import 审查。endpointspec 可以使用 apischema 的协议类型，translator 按已识别的输入协议与选中后端协议适配，不负责路由选择、凭据加载或网络连接池。协议类型不承载监听、配置和请求生命周期。

Controller 继续只依赖 `internal/pkg/aidataplane`，不导入 Proxy 的 aiconfig。顶层 `pkg/apis`、`pkg/generated` 的公开契约定位不变；不把 Proxy 私有实现搬到那里，也不把仅有一个组件所有者的包提升到仓库级 internal/pkg。

## 当前实施范围与行为

用户确认应用用例命名为 modelproxy。原 service/model 和 ModelHandler 容易被误读为模型资源管理，实际职责是模型请求代理；本次仅使用重命名迁移包、Handler、装配引用与文档，减少概念歧义，不增加分层或改变协议。行为基线为重命名前的模型匹配、Header 隔离、普通响应、SSE 与取消测试；成功条件是引用统一到 service/modelproxy、无旧名称兼容别名、公开路径/错误/数据契约不变且回归通过。旧 pkg/modelproxy 转发器仍保持删除，不因同名恢复旧方案。

2026-10-15 用户进一步确认调整请求主线。当前根因是模型准备与 HTTP 出站编排分散在 Service、Handler 和 ReverseProxy 之间，Handler 同时持有两类执行依赖，并通过 Context 隐式交接目标。本次按职责迁移：Service 解析、捕获配置、选路、适配并调用原生 HTTP Client；Handler 只处理入站读取、公开错误和响应写回；出口包只构造固定 Envoy UDS 的原生 Client，不新增 Client 接口或流框架。该 Service 明确属于模型 HTTP 代理用例，返回原生 HTTP Response，不承诺与 HTTP 协议无关，但不依赖 Kratos、入站 Request 或 ResponseWriter。验证基线为现有模型匹配、覆盖、请求隔离和 UDS/SSE 用例；成功条件是主流程完整进入 Service、删除目标 Context 与 ReverseProxy，同时保留目标绑定、无重定向、Header 隔离、响应状态/正文、流刷新和取消行为。不新增业务能力；按仓库要求保留未提交改动，待 review 后再提交。

Service.ChatCompletion 成功后将原生响应及 Body 所有权交给 Handler，不预读整个正文。[响应写回](../../internal/aiproxy/handler/response.go) 负责移除逐跳 Header、复制状态和正文、刷新 SSE/未知长度响应、交付 Trailer，并最终关闭 Body。响应开始后的读取、写入或 Flush 失败使用 net/http.ErrAbortHandler 中断连接，不再返回错误让框架追加 JSON。后续若加入 panic 恢复中间件，必须保留该原生中断约定。Client 不跟随重定向、不自动解压；无整次请求超时，不增加模型重试。当前仅支持普通响应与 SSE：不转发中间 1xx，拒绝非预期 101 升级，不声称等价实现通用 ReverseProxy。

已核对的官方固定提交中，模型通过 ExtProc，MCP 的 HTTP 连接使用原生 Client；本次是在统一 HTTP Proxy 前提下采用后者的网络调用方式，不把 Service/Handler 分层或本项目的 UDS 目标绑定声称为官方原样实现。

aiconfig 整包迁移，只调整文档引用和测试夹具路径，不改变 ConfigMap 筛选、UID 身份、保留或撤回策略、路由顺序和并发交接。未提交的 OpenAI 请求处理按职责重组，不保留旧包别名。

apischema/openai 只定义当前需要提取的 model 字段，不宣称覆盖完整请求，也不用于重新编码上游请求。endpointspec 保留原有 UTF-8、JSON 对象、精确字段名、非空模型和重复顶层键校验。translator 接收经过入口校验的原始 JSON，空覆盖或相同模型返回独立的原始字节副本；其它覆盖只改变顶层 model 的值，保留扩展字段与数字精度，允许重编码改变排版与转义形式。

请求体由调用方保留；解析只返回独立的协议字段，不再用同一个对象同时保存请求体和提供出站编码方法。每次适配从传入的原始字节生成独立结果，不原地修改源数据，不通过可变协议对象反向决定待改写的原始模型名。modelproxy.Service 在同一请求内使用同一份原始字节和捕获配置，Handler 不再次捕获配置或接收选中后端。

入口校验和非空模型覆盖各自解码原始 JSON，较原包装对象增加了解码工作；当前复用标准库与已有严格 JSON 库，不为此增加 JSON 修改依赖或跨包暴露可变字段表。未做性能基准，不宣称这次结构调整改善吞吐或分配。

此结构变更不增加接口端点、厂商能力、依赖或转换矩阵，不更改领域资源与编译 JSON 契约。用户确认布局不等于批准所有未来包或完整服务实现；本次请求链重构与装配已通过代码 review，仅授权本地提交，不推送或创建 PR。

## 验证边界

模型入口切片已提交为 `d988f8df`。当前启动配置切片在 conf 保存 protobuf，在组件根目录 config.go 读取并校验；示例为 configs/ai-proxy.yaml。这里复用 Admin API 的严格 YAML → protojson → Protovalidate 方式，不导入 Admin API 的配置类型，也不增加全仓库加载器。配置只覆盖已有构造依赖；本步未接入 Run、命令行或 Watch 生命周期，不因增加 conf 就声称进程可运行。上文“不预建 conf”针对此前协议切片；当前 conf 已有明确的启动输入契约和校验测试，不是占位包。

`921484b3` 已提交原生 Client 与 HTTP 装配。后续模型公共入口切片在 server 增加路由 Filter，将受保护 Unix 入口的内部 Header 转为已有 Context，不把这项传输职责放进 Service 或另建能力包。该切片已通过用户 review 并获准本地提交；[实现记录](../design/ai-model-operations-and-deployment.md#envoy-模型入口与-go-入口适配)说明通信契约和真实组件未验证的边界。上文缺少入口适配的描述属于此前装配切片的范围，不代表这次新增代码的状态。

以下为原包迁移的历史检查记录，不代替当前请求链重构的检查。当前变更及其验证范围见[实现记录](../design/ai-model-operations-and-deployment.md#http-对象装配与路由注册)。

以迁移前现有测试为行为基线，保留配置筛选、错误更新、对象替换、撤回及请求解析和字段保持的正反例；请求体所有权改为分别核对解析结果与源字节独立、适配不修改源字节及返回值独立。普通仓库检查和针对配置包的 race 检查用于验证本地实现，不替代真实 Envoy/Proxy、UDS、凭据或模型服务联调。

本轮重新查询 GoCanon MCP，来源 `/Users/lgc202/workspace/source/lgc202/gocanon`，revision `08260b72394a104ebe8286088eb23383cd6a6235`、版本 `08260b7-dirty`；沿用已完整读取且来源未变的开发入口和相关规范，草案不等同于稳定约束。已审查全部手写文件和删除路径，迁移包逐文件比对确认除夹具路径外内容一致；新拆分逻辑逐函数检查职责、命名、错误保留与反例断言，再独立复查 GO-FUNC-012/013/014 的声明顺序、签名、换行及语义空行。

`gofmt -l cmd internal pkg`、`git diff --check` 和全部未跟踪文件的空白检查无诊断；`golangci-lint run ./...` 为 0 issues，`go vet ./...`、`go test ./... -count=1`、`go build ./...` 与 `go test -race ./internal/aiproxy/... -count=10` 均通过。没有变更声明式 API、存储契约、依赖或生成源，未执行 API integration 或重新生成；真实组件验证及性能验证未执行。本轮实现已通过用户 review 并获准本地提交；下一步继续独立的 HTTP 入口切片。
