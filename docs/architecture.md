# Ingate 架构

![Ingate 产品流量模型](assets/ingate-overview.png)

## 设计边界

Ingate 使用官方 Envoy 作为唯一数据平面。控制面直接把声明式资源编译为 Envoy 配置，不维护 Envoy 私有分支，也不为 Kong、Nginx 等其他数据平面预设通用适配层。

一套 Ingate 表示一个环境、一个配置域和一组配置完全相同的 Envoy 实例。一套 Ingate 可以创建多个逻辑 Gateway；生产、测试、机房或租户之间需要控制面隔离时，应分别部署多套 Ingate。

Ingate 的产品流量模型固定为：

**Gateway → Route → Service**

API 和 AI 是 Route 与 Service 的不同类型，不形成两套平行的资源体系。Console 使用 Service 这一产品名称，声明式 API 当前使用 `Upstream` 表达同一个对象。

## 三条运行链路

### 控制链路

控制链路只负责声明、校验和发布配置：

1. Browser 通过 `ingate-console` 访问 `ingate-admin-api`
2. Admin API 完成产品协议校验和业务规则检查，通过生成客户端读写 `ingate-apiserver`
3. API Server 是 etcd 的唯一访问者，负责资源 CRUD、List/Watch、版本和 Status 子资源
4. `ingate-controller` Watch 声明式资源，把当前完整资源集合编译为 Envoy Listener、Route、Cluster 和 Secret
5. 新配置通过完整性校验后成为 Active 配置，并由 xDS 下发到 Envoy
6. Controller 把资源是否参与当前有效配置回写到 API Server Status

Admin API、Controller、Authorization 和 AI Processing 都不能直接访问 etcd。声明式资源是配置事实来源；Controller 不持久化 Last Good，进程重启后从资源重新全量编译。

### 流量链路

客户端只访问 Envoy，管理组件不在业务请求路径上。

- API Route 由 Envoy 完成匹配、治理、负载均衡和 HTTP Service 转发
- 使用调用方访问模式的 Route 通过 Envoy `ext_authz` 调用 `ingate-authz`，公开 Route 不执行远程鉴权
- AI Route 通过 Envoy `ext_proc` 调用 `ingate-ai-extproc`，完成调用方 Token 额度检查、对外模型名选择、真实模型名改写、凭据注入和协议转换
- OpenAI 兼容模型线路保持 Chat Completions 协议，只改写线路相关字段
- Anthropic 模型线路在 OpenAI Chat Completions 与 Anthropic Messages 之间转换请求、普通响应和 SSE 流式响应

`ingate-authz` 和 `ingate-ai-extproc` 都通过 API Server Watch 自己执行所需的最小资源集合。它们只处理同步请求所需的判断和改写，不承担配置持久化或分析查询。

### 观测链路

请求观测与同步流量处理解耦：

1. Envoy 通过 ALS 把请求元数据发送给 `ingate-als`
2. ALS 优先投递 Kafka；Kafka 暂时不可用时，未投递记录进入本地 WAL，并在恢复后重放
3. `ingate-analytics` 批量消费 Kafka，每条事件以稳定记录 ID 异步写入 ClickHouse
4. ClickHouse 在服务端合并小写入，并通过物化视图维护流量与模型用量的分钟聚合
5. Admin API 调用 Analytics gRPC 查询请求记录、趋势、响应分布和资源排行

观测链路不持久化请求 Header、查询参数或正文。Kafka 和 ClickHouse 故障不能改变 Envoy 的路由结果，但会影响请求记录的可见时间和本地 WAL 占用。

Analytics 使用 At Least Once 消费语义：请求事实和模型调用都成功持久化后才提交 Kafka offset。消费失败或 offset 提交失败会重投整批消息；写入使用记录 ID 作为稳定的 ClickHouse 去重 token，物化视图在源事件去重后再累计指标，因此 Poll 批次边界变化不会造成请求量或 Token 重复计算。去重日志只覆盖近期在线重投；超出窗口的离线历史回放需要先清理并重建对应时间范围的聚合数据。

## 组件职责

| 组件 | 上游依赖 | 职责 |
| --- | --- | --- |
| `ingate-console` | Admin API | 托管控制台并代理管理请求 |
| `ingate-admin-api` | API Server、Analytics | 提供面向 Console 的产品 API 和业务校验 |
| `ingate-apiserver` | etcd | 提供声明式资源 API，是 etcd 的唯一访问者 |
| `ingate-controller` | API Server | 收敛资源状态、编译 Envoy 配置、提供 xDS、回写 Status |
| `Envoy` | Controller、Authorization、AI Processing、ALS | 接收业务流量并执行数据面配置 |
| `ingate-authz` | API Server | 校验调用方访问密钥和 Route 授权 |
| `ingate-ai-extproc` | API Server、Redis | 检查和结算调用方 Token 额度、选择模型线路并转换模型协议 |
| `ingate-als` | Kafka | 接收请求记录并可靠投递 |
| `ingate-analytics` | Kafka、ClickHouse | 写入请求事实并提供分析查询 |

## 资源关系

| 资源 | 作用 | 主要引用关系 |
| --- | --- | --- |
| `Gateway` | 定义 HTTP/HTTPS 监听入口 | HTTPS Listener 引用 Certificate |
| `Route` | 定义 API/AI 请求匹配和转发 | 引用 Gateway、Upstream；可要求 Caller |
| `Upstream` | 定义 HTTP 或模型 Service 的连接方式 | 被 Route 的普通目标或模型线路引用 |
| `Certificate` | 保存可复用的 TLS 证书和私钥 | 被 Gateway HTTPS Listener 引用 |
| `Caller` | 保存访问密钥摘要和 Route 权限 | 被 Authorization Watch |
| `IPRestrictionPolicy` | 限制 Gateway 或 Route 的客户端 IP | 通过 `targetRefs[]` 引用 Gateway 或 Route |
| `RateLimitPolicy` | 声明 Gateway 或 Route 的限流意图 | 通过 `targetRefs[]` 引用 Gateway 或 Route |
| `TokenQuotaPolicy` | 限制 Caller 在自然周期内的模型 Token 用量 | 通过 `targetRefs[]` 引用 Caller |

资源使用 `metadata.name` 作为不可变 ID，使用 `spec.displayName` 保存用户可编辑名称。Admin API 创建资源时生成 UUID，并把底层 `metadata/spec/status` 转换为面向 Console 的平铺协议。

## 配置发布

Controller 的发布顺序是 Resource Watch、Reconcile、Envoy Compiler、Delivery 和 xDS：

- Resource Watch 维护声明式资源缓存并触发收敛
- Reconcile 每次读取完整资源集合，不对单个资源做局部拼接发布
- Compiler 生成 Envoy Listener、Route、Cluster 和 Secret，并记录资源诊断信息
- Delivery 只在整套配置通过校验后切换 Active 配置
- xDS Snapshot Cache 向 Envoy 提供当前 Active 配置
- Status Writer 把接收、引用解析和发布结果写回对应资源

Candidate 和 Active 只存在于 Controller 进程内。重启时允许短暂重新收敛，不向 etcd 写入派生配置或 Last Good。

## 数据归属

| 数据 | 唯一持久化位置 | 写入者 |
| --- | --- | --- |
| 声明式资源与 Status | etcd | API Server |
| Gateway Certificate 密钥材料 | etcd | API Server |
| 当前 Envoy 有效配置 | Controller 内存 | Controller |
| ALS 待投递记录 | ALS 本地 WAL | ALS |
| 请求明细与模型调用记录 | ClickHouse | Analytics |
| 流量与模型用量聚合 | ClickHouse 物化视图 | ClickHouse |
| 当前周期 Token 额度计数 | Redis | AI ExtProc |

Redis 是安装级系统组件，不建模为用户资源。AI ExtProc 使用 Redis 保存 TokenQuotaPolicy 的实时计数；额度配置本身仍由 API Server 持久化到 etcd。完整执行语义见 [Token 额度原理](resources/token-quota-policy.md)。RateLimitPolicy 目前只有资源协议和管理能力，Controller 不生成限流执行配置。

## 部署约束

服务二进制、YAML 配置、健康检查、结构化日志和优雅退出保持部署方式中立。Docker Compose 只用于开发和联调，不代表生产拓扑。

开发 Compose 中，每个服务运行在独立容器。Controller、Envoy 与 AI ExtProc 共享网络命名空间，使 xDS 和 AI Processing 连接只监听 loopback；Authorization 和 ALS 通过 Compose 内部网络访问，不向宿主机暴露端口。其他部署方式必须提供等价的网络隔离，或为这些内部 gRPC 连接配置传输安全。

安装路径统一使用 `/opt/ingate/<component>`；运行数据由部署方式挂载到明确目录。
