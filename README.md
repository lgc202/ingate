# Ingate

Ingate 是面向 API 网关和 AI 网关的声明式 Envoy 控制面。应用服务、模型、MCP 和 Agent 统一建模为 `Upstream`，Envoy 是唯一数据平面。

## 架构

```text
Console -> ingate-admin-api -----+
CLI / SDK -----------------------+-> ingate-apiserver -> etcd
                                      ^          |
                                      | status   | watch spec
                                      |          v
                                 ingate-controller
                    Envoy Compiler -> Config Delivery
                                   -> xDS Snapshot Cache
                                      |
                                      v
                                    Envoy
```

一套 Ingate 表示一个环境、一个配置域和一组配置完全相同的 Envoy 实例。一套 Ingate 可以声明多个逻辑 Gateway；所有资源会被全量编译成同一份 Envoy 配置。IP Upstream 使用 EDS，hostname Upstream 使用 Envoy `STRICT_DNS` cluster。

主要组件：

- `ingate`：CLI 和本地管理入口
- `ingate-admin-api`：控制台产品 API
- `ingate-apiserver`：声明式资源 API，也是持久化数据的唯一入口
- `ingate-controller`：资源收敛、Envoy 配置编译、Delivery、ADS xDS 和资源 status 更新
- `Envoy`：唯一数据平面，二进制来自带 Redis 扩展 ABI 的 Higress Envoy
- `etcd`：由 apiserver 使用的声明式资源存储
- `Redis`：限流及 Token 配额等请求路径共享状态

内置限流、访问控制和 Token 配额以强类型 Policy 对外提供，策略通过自身的 `targetRefs[]` 直接声明生效的 Gateway 或 Route。用户不需要安装内置 Wasm 插件，也不需要选择本地或全局计数模式、算法或 Redis 地址；请求路径共享状态统一使用 Envoy bootstrap 中固定的 `ingate-system-redis`。

## AI Gateway

AI Gateway 不新增 AI runtime 或独立服务，继续使用现有资源和 Controller 编译链路：

```text
OpenAI Client
  -> Envoy
  -> 内置 tokenquota Wasm
  -> 内置 ai-proxy Wasm
  -> 按 model 选择模型 Upstream
  -> OpenAI / DeepSeek / 通义 / Anthropic / Gemini / 自定义兼容服务
```

- 对外只处理 OpenAI-compatible `POST /v1/chat/completions`，客户端不需要感知不同厂商的请求路径、认证 Header 和响应事件格式
- 模型服务仍使用 `Upstream(type=model)`；`protocol` 表达 OpenAI、Anthropic 或 Gemini 通信语义，`spec.model.provider` 表达厂商，`spec.model.models[]` 保存用户手工维护的可用模型目录
- 模型服务通过 `tls.serverName` 启用 HTTPS、SNI 和系统 CA 根证书包校验
- 模型服务的 API Key 直接随 `Upstream` 配置，不再创建独立凭据资源；Admin API 不回显密钥，只返回是否已配置，更新时省略密钥会保留原值，显式移除才会清除；配置 API Key 时必须使用 HTTPS
- 一条模型 RouteRule 的 `modelRouting.models[]` 中，每个公开模型别名独立引用模型 Upstream 和厂商模型；同一路由可以按请求体 `model` 跨厂商选择目标
- Envoy Config Compiler 生成 Cluster、受控内部选路 Header 和 `ai-proxy` 私有执行配置；用户不需要安装插件或编辑插件 JSON
- `ai-proxy` 将请求、普通响应、错误、Token usage 和 SSE 统一为 OpenAI-compatible 语义，响应中的 `model` 始终返回客户端公开别名
- `TokenQuotaPolicy` 可以应用到模型 Route 或承载模型 Route 的 Gateway，按共享预算、来源 IP 或请求 Header 值累计输入与输出 Token；多个 `targetRefs[]` 共享同一个策略预算池
- Token 配额在请求前检查当前固定窗口，在响应结束后按上游返回的实际 `usage.total_tokens` 记账；并发中的请求可能造成有限超额，因此当前能力属于软额度而不是严格预扣

Token 配额用于预算保护，不替代计费系统。流式请求在完成标记到达前中断时可能漏记；Header 维度必须使用可信认证层写入且客户端无法伪造的值。自定义 OpenAI-compatible 上游若启用流式 Token 配额，需要支持 `stream_options.include_usage` 并在最终事件返回 usage。

当前只支持文本 `system`、`user`、`assistant` 消息和 `model/messages/stream/temperature/top_p/max_tokens/stop`。当前不支持 Tools/function calling、多模态、Responses、Embeddings、自动同步厂商模型、多 Provider fallback/retry、OAuth/IAM 云认证及大文件请求。单次 AI 请求体上限为 1 MiB。

所有产品状态都通过声明式资源的 status 表达。Admin API 只访问 API Server，不直接查询 Controller；Controller 通过 status 子资源写入 `Accepted`、`ResolvedRefs` 和 `Programmed` 等观察结果，Policy 还使用 `status.targets[]` 记录每个目标的生效状态。

Gateway 使用固定的 standalone 入口：HTTP `8080`、HTTPS `8443`。多个逻辑 Gateway 共享相同的 Envoy Listener，通过 Host 和 TLS SNI 分流；HTTPS Listener 引用独立 Certificate 资源。

## 目录

源码仓库使用相对目录组织代码和本地开发文件：

```text
cmd/                         服务和 CLI 入口
configs/                     直接运行服务时使用的默认 YAML 配置
internal/                    控制面内部实现
pkg/                         声明式 API 类型与生成客户端
plugins/                     内置 Proxy-Wasm 插件和 Ingate Redis ABI
web/console/                 控制台前端
deploy/docker-compose.yaml   开发联调环境
deploy/docker/               开发镜像与容器配置
scripts/make-rules/          Makefile 规则
hack/                        代码生成脚本
```

服务二进制、配置、健康检查、日志和退出流程不依赖具体部署方式。容器内使用下面的组件目录：

```text
/opt/ingate/
├── admin-api/
│   ├── bin/ingate-admin-api
│   ├── configs/config.yaml
│   └── console/
├── apiserver/
│   ├── bin/ingate-apiserver
│   ├── configs/config.yaml
│   └── certificates/           API Server 自身运行证书
├── controller/
│   ├── bin/ingate-controller
│   └── configs/config.yaml
├── envoy/
│   ├── bin/envoy
│   └── configs/bootstrap.yaml
└── plugins/                    Ingate 内置 Wasm 插件
```

API Server 自身运行证书属于组件运行文件。用户为 Gateway 创建的 Certificate 仍是声明式资源，由 API Server 持久化到 etcd，二者不能混用。

源码开发继续使用 `configs/` 和 `_output/`。systemd、Kubernetes 等其它交付方式只在出现真实需求时设计，不为它们提前建立部署抽象。

## 构建

项目使用 Go 1.26 和 Node.js 24。首次开发前可以检查宿主工具，并把 Kubernetes 代码生成器安装到仓库本地：

```bash
make help
make check-tools
make tools
make verify
```

`make tools` 使用 `go.mod` 中锁定的版本，将生成器安装到 `_output/tools`，不会写入全局 `$GOPATH/bin`。`make verify` 执行格式化、生成代码检查、Go 编译检查、服务构建、Wasm 插件构建和 Console 构建。

## Docker Compose 开发联调

Docker Compose 只是本地开发、联调和演示入口，不表示 Ingate 的唯一或生产部署拓扑。

```bash
make docker-up
make docker-ps
```

默认入口：

```text
Console:      http://127.0.0.1:8001
Gateway HTTP: http://127.0.0.1:8080
Gateway TLS:  https://127.0.0.1:8443
```

开发环境由独立的 etcd、Redis、ingate-apiserver、ingate-controller、ingate-admin-api 和 Envoy 容器组成。Console 静态资源包含在 Admin API 镜像中。

Controller 和 Envoy 共享网络命名空间，xDS 继续只监听 `127.0.0.1`，不会因为拆分容器而暴露到开发网络。Compose 直接使用固定版本的 `lgc202/ingate-envoy:v0.1.0`；该镜像从 Higress Gateway `v2.2.3` 提取带 Redis ABI 的 Envoy 二进制，并加入 Ingate 内置 Wasm 插件。

etcd、Redis 和 API Server 证书使用独立 Docker Volume。Go 服务日志写入容器标准输出，通过 `make docker-logs` 查看。停止环境不会删除 Volume：

```bash
make docker-logs
make docker-down
```

每个服务仍可脱离 Docker 直接运行，通过 `--config` 指定配置文件、通过 `--version` 查看构建版本。
