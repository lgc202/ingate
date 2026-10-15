# 0015：AI Proxy 应用分层与能力包布局

- 状态：已接受；用户于 2026-10-15 确认布局，代码已通过 review 并获准本地提交
- 日期：2026-10-15

## 背景

AI Proxy 的配置消费已经提交，请求体解析与模型覆盖仍在本地开发。原布局将 aiconfig 和 OpenAI 请求处理直接放在组件根目录；未提交的 OpenAI 请求对象还同时承担协议字段、入口校验和出站编码。继续接入服务和更多协议时，需要区分应用编排与这些能力的所有权。

官方 Agent Router 固定提交 `daa9f891a8afcb18576d4593ad18870d1d18abac` 将协议类型、入口操作、协议适配和后端鉴权分别放在 [apischema](https://github.com/theagentrouter/agent-router/tree/daa9f891a8afcb18576d4593ad18870d1d18abac/internal/apischema)、[endpointspec](https://github.com/theagentrouter/agent-router/blob/daa9f891a8afcb18576d4593ad18870d1d18abac/internal/endpointspec/endpointspec.go)、[translator](https://github.com/theagentrouter/agent-router/tree/daa9f891a8afcb18576d4593ad18870d1d18abac/internal/translator) 和 [backendauth](https://github.com/theagentrouter/agent-router/blob/daa9f891a8afcb18576d4593ad18870d1d18abac/internal/backendauth/auth.go)。Ingate 借鉴职责划分，保留已确认的统一 HTTP Proxy，不移植 ExtProc 或完整转换框架。

## 决策

`internal/aiproxy` 根目录保留组件组合根及 conf、server、handler、service、repository 等实际需要的应用分层。组件自身的配置消费和协议能力统一放在 `internal/aiproxy/pkg/<capability>`；这里的 pkg 是组件私有实现的目录分组，不表示对外发布或全仓库共享。这是 Ingate 的已确认目录选择，不作为 GoCanon 对所有组件的通用要求。

```text
internal/
├── aiproxy/
│   ├── handler/              # Kratos 原生模型 HTTP 适配与请求上下文
│   ├── service/
│   │   └── model/            # 模型请求准备的用例编排
│   └── pkg/
│       ├── aiconfig/          # ConfigMap 加载、只读配置发布与查询
│       ├── apischema/
│       │   └── openai/       # 当前实际使用的 OpenAI 协议字段
│       ├── endpointspec/     # 客户端操作的请求解析与校验
│       ├── modelproxy/       # 标准 ReverseProxy、请求目标交接与受限 UDS 出口
│       └── translator/       # 当前 OpenAI 到 OpenAI 的模型覆盖
└── pkg/
    └── aidataplane/           # Controller 和 Proxy 共用的编译契约
```

上图只列当前实际实现的包；model Service 与 HTTP Handler 是包迁移后的后续切片，已通过用户 review 并获准本地提交。不预建 conf、server、repository、backendauth、其它协议或空接口；后续按已确认的真实调用增加。Repository 只承接实际存储访问，不为完整套用 CRUD 分层而包一层透传。

Service 与 Handler 已提交为 `81d3e027`。后续 modelproxy 切片保留未提交、待 review：它依赖按值的 aiconfig.ModelBackend 和共享目标契约，不反向导入 Handler；请求目标的 Context 交接归该转发能力所有。Handler 只保留入口 Context，字段 next 改为 forwarder，以明确它不是任意中间件链。modelproxy 不是新 Provider 框架，也不包含模型选择、配置 Watch、公开错误格式或凭据管理。

依赖从应用编排流向能力包；能力包不依赖组件的 conf、Handler、Service、Repository 或 Controller 实现。pkg 目录本身不增加 Go 的编译期可见性限制，组件所有权与依赖方向仍须通过实际 import 审查。endpointspec 可以使用 apischema 的协议类型，translator 按已识别的输入协议与选中后端协议适配，不负责路由选择、凭据加载或网络连接池。协议类型不承载监听、配置和请求生命周期。

Controller 继续只依赖 `internal/pkg/aidataplane`，不导入 Proxy 的 aiconfig。顶层 `pkg/apis`、`pkg/generated` 的公开契约定位不变；不把 Proxy 私有实现搬到那里，也不把仅有一个组件所有者的包提升到仓库级 internal/pkg。

## 当前实施范围与行为

aiconfig 整包迁移，只调整文档引用和测试夹具路径，不改变 ConfigMap 筛选、UID 身份、保留或撤回策略、路由顺序和并发交接。未提交的 OpenAI 请求处理按职责重组，不保留旧包别名。

apischema/openai 只定义当前需要提取的 model 字段，不宣称覆盖完整请求，也不用于重新编码上游请求。endpointspec 保留原有 UTF-8、JSON 对象、精确字段名、非空模型和重复顶层键校验。translator 接收经过入口校验的原始 JSON，空覆盖或相同模型返回独立的原始字节副本；其它覆盖只改变顶层 model 的值，保留扩展字段与数字精度，允许重编码改变排版与转义形式。

请求体由调用方保留；解析只返回独立的协议字段，不再用同一个对象同时保存请求体和提供出站编码方法。每次适配从传入的原始字节生成独立结果，不原地修改源数据，不通过可变协议对象反向决定待改写的原始模型名。后续调用必须在同一请求内使用同一份原始字节和捕获配置；包迁移切片没有实现该运行链，后续新增的 model Service 统一负责捕获和请求准备，Handler 不应再次捕获并混用。

入口校验和非空模型覆盖各自解码原始 JSON，较原包装对象增加了解码工作；当前复用标准库与已有严格 JSON 库，不为此增加 JSON 修改依赖或跨包暴露可变字段表。未做性能基准，不宣称这次结构调整改善吞吐或分配。

此结构变更不增加接口端点、厂商能力、依赖或转换矩阵，不更改领域资源与编译 JSON 契约。用户确认布局不等于批准所有未来包或完整服务实现；本轮代码已完成 review 并获准本地提交，不推送或创建 PR。

## 验证边界

以迁移前现有测试为行为基线，保留配置筛选、错误更新、对象替换、撤回及请求解析和字段保持的正反例；请求体所有权改为分别核对解析结果与源字节独立、适配不修改源字节及返回值独立。普通仓库检查和针对配置包的 race 检查用于验证本地实现，不替代真实 Envoy/Proxy、UDS、凭据或模型服务联调。

本轮重新查询 GoCanon MCP，来源 `/Users/lgc202/workspace/source/lgc202/gocanon`，revision `08260b72394a104ebe8286088eb23383cd6a6235`、版本 `08260b7-dirty`；沿用已完整读取且来源未变的开发入口和相关规范，草案不等同于稳定约束。已审查全部手写文件和删除路径，迁移包逐文件比对确认除夹具路径外内容一致；新拆分逻辑逐函数检查职责、命名、错误保留与反例断言，再独立复查 GO-FUNC-012/013/014 的声明顺序、签名、换行及语义空行。

`gofmt -l cmd internal pkg`、`git diff --check` 和全部未跟踪文件的空白检查无诊断；`golangci-lint run ./...` 为 0 issues，`go vet ./...`、`go test ./... -count=1`、`go build ./...` 与 `go test -race ./internal/aiproxy/... -count=10` 均通过。没有变更声明式 API、存储契约、依赖或生成源，未执行 API integration 或重新生成；真实组件验证及性能验证未执行。本轮实现已通过用户 review 并获准本地提交；下一步继续独立的 HTTP 入口切片。
