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

## 第一阶段 AI Gateway

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
- Token 配额在请求前检查当前固定窗口，在响应结束后按上游返回的实际 `usage.total_tokens` 记账；并发中的请求可能造成有限超额，因此首版属于软额度而不是严格预扣

Token 配额用于预算保护，不替代计费系统。流式请求在完成标记到达前中断时可能漏记；Header 维度必须使用可信认证层写入且客户端无法伪造的值。自定义 OpenAI-compatible 上游若启用流式 Token 配额，需要支持 `stream_options.include_usage` 并在最终事件返回 usage。

第一阶段只支持文本 `system`、`user`、`assistant` 消息和 `model/messages/stream/temperature/top_p/max_tokens/stop`。当前不支持 Tools/function calling、多模态、Responses、Embeddings、自动同步厂商模型、多 Provider fallback/retry、OAuth/IAM 云认证及大文件请求。单次 AI 请求体上限为 1 MiB。

所有产品状态都通过声明式资源的 status 表达。Admin API 只访问 API Server，不直接查询 Controller；Controller 通过 status 子资源写入 `Accepted`、`ResolvedRefs` 和 `Programmed` 等观察结果，Policy 还使用 `status.targets[]` 记录每个目标的生效状态。

Gateway 使用固定的 standalone 入口：HTTP `8080`、HTTPS `8443`。多个逻辑 Gateway 共享相同的 Envoy Listener，通过 Host 和 TLS SNI 分流；HTTPS Listener 引用独立 Certificate 资源。

## 目录

源码仓库使用相对目录组织代码和本地开发文件：

```text
cmd/                    服务和 CLI 入口
configs/                各服务的 YAML 配置
internal/               控制面内部实现
pkg/                    声明式 API 类型与生成客户端
plugins/                内置 Proxy-Wasm 插件和 Ingate Redis ABI
web/console/            控制台前端
deploy/all-in-one/      all-in-one 镜像与运行配置
hack/                   代码生成脚本
install.sh              本地安装脚本
```

安装包和容器内统一使用下面的运行目录：

```text
/opt/ingate/                    程序、配置和随组件发布的文件
├── bin/                        安装或容器级入口
├── configs/kubeconfig         all-in-one 内部共享连接配置
├── admin-api/
│   ├── bin/ingate-admin-api
│   ├── configs/config.yaml
│   ├── console/
│   └── scripts/
├── apiserver/
│   ├── bin/ingate-apiserver
│   ├── configs/config.yaml
│   ├── certificates/           API Server 自身运行证书
│   └── scripts/
├── controller/
│   ├── bin/ingate-controller
│   ├── configs/config.yaml
│   └── scripts/
├── envoy/
│   ├── bin/envoy
│   └── configs/bootstrap.yaml
├── plugins/                    Ingate 内置插件
├── etcd/                       etcd 二进制和配置
└── redis/                      Redis 二进制和配置

/data/ingate/                   运行产生的持久化数据
├── admin-api/logs/
├── apiserver/logs/
├── controller/logs/
├── envoy/logs/
├── etcd/data/
├── redis/data/
├── plugins/                    外部安装或动态缓存的插件
└── backups/                    未来的备份目录
```

API Server 自身运行证书属于组件文件，放在 `/opt/ingate/apiserver/certificates`。用户为 Gateway 创建的 Certificate 仍是声明式资源，由 API Server 持久化到 etcd，不作为本地证书文件保存在该目录。

这套绝对路径只约束安装包和容器内布局。源码开发仍可使用 `configs/`、`_output/` 和 `ingate-dev/` 等相对路径。

## 构建

项目使用 Go 1.26。

```bash
make test
make build
make plugins-build
make console-build
make all-in-one-image
```

## 本地运行

```bash
./install.sh restart \
  --image ingate/all-in-one \
  --tag dev \
  --data-dir ./ingate-dev
```

默认入口：

```text
Console:      http://127.0.0.1:8001
Gateway HTTP: http://127.0.0.1:8080
Gateway TLS:  https://127.0.0.1:8443
```

all-in-one 内只运行产品必需组件：etcd、Redis、ingate-apiserver、ingate-controller、ingate-admin-api、Envoy 和 Console。测试后端使用独立容器，不进入产品镜像。

本地数据和日志默认保存在 `ingate-dev/`，不提交到 Git；它是开发环境对生产 `/data/ingate` 的本地映射。

all-in-one 首次启动时会把默认服务配置复制到 `ingate-dev/configs/` 并挂载进容器。修改 `logging.level` 会立即生效，其它服务配置需要重启。每个服务也支持通过 `--config` 指定配置文件、通过 `--version` 查看构建版本。
