---
title: 系统架构
description: Ingate 的控制链路、同步流量链路、观测链路、运维辅助链路和数据归属
---

Ingate 使用官方 Envoy 作为唯一数据平面。控制面把声明式资源编译为 Envoy 配置，不维护私有 Envoy 分支，也不为其他数据平面预设适配层。

![Ingate 系统架构，Gateway、Route 和 Service 位于统一的 API 与 AI 流量路径中](/ingate/images/architecture/system.png)

## 组件架构

系统按控制面、同步流量处理、异步观测和运维辅助拆分。只有鉴权、AI 协议适配等请求级处理组件位于同步链路中；Assistant 不参与配置发布或业务流量。

![Ingate 组件架构，展示控制面、数据面和观测组件之间的通信方向](/ingate/images/architecture/components.png)

## 控制链路

控制链路负责声明、校验和发布配置：

1. Browser 通过 Console 访问 Admin API
2. Admin API 完成产品协议校验和业务规则检查，再调用 API Server
3. API Server 提供资源 CRUD、List/Watch、版本和 Status，并且是 etcd 的唯一访问者
4. Controller Watch 当前资源集合，编译完整 Envoy Listener、Route、Cluster 和 Secret
5. 配置通过完整性校验后作为 Candidate 通过 xDS 下发；所有当前已连接的 Envoy 实例都接受后才成为 Active，任一 NACK 或 ACK 超时都恢复上一个 Active 配置
6. Controller 把资源是否进入当前有效配置写回 Status

声明式资源是配置事实来源。Controller 不把派生配置或 Last Good 写入 etcd，重启后从资源重新全量编译。

API Server 只允许携带内部 Bearer Token 的控制面组件访问资源、发现和监控端点；`/healthz`、`/livez` 与 `/readyz` 保持匿名可探测。客户端必须校验 API Server 的 TLS 证书，不能通过跳过证书校验来换取部署便利。

## 同步流量链路

客户端只访问 Envoy，Console、Admin API、API Server 和 Controller 不参与业务请求。

- API Route 由 Envoy 完成匹配、治理、负载均衡和 HTTP Service 转发
- 受保护 Route 通过 Envoy `ext_authz` 调用 Authz，校验访问密钥和 Route 权限
- 命中 RateLimitPolicy 的公开 Route 也调用 Authz 执行限流，但不会因此要求访问密钥
- AI Route 通过 Envoy `ext_proc` 调用 AI ExtProc，完成额度检查、模型选择、凭据注入和协议转换
- IPRestrictionPolicy 编译为 Envoy 原生 RBAC 配置，不调用外部服务

Authz 和 AI ExtProc 通过 API Server Watch 自己所需的最小资源集合。Redis 只保存同步判断所需的实时计数，不保存策略配置。

## 异步观测链路

请求观测不阻塞同步转发：

1. Envoy 通过 ALS 发送请求元数据
2. Ingate ALS 优先投递 Kafka；Kafka 不可用时写入本地 WAL，恢复后重放
3. Analytics 批量消费 Kafka，把请求事实和模型调用写入 ClickHouse
4. ClickHouse 通过物化视图维护分钟流量与模型用量聚合
5. Admin API 调用 Analytics gRPC 查询请求记录和分析结果

Kafka 或 ClickHouse 故障不会改变 Envoy 的路由结果，但会延迟请求记录，并增加 ALS 本地 WAL 占用。

Analytics 使用 At Least Once 消费语义。请求事实和模型调用都成功写入后才提交 Kafka offset；稳定记录 ID 和确定性批次 token 用于处理重投，避免在线重试重复累计请求量或 Token。

## 运维辅助链路

运维助手是可选的控制面辅助能力，不是数据平面，也不是声明式配置的事实来源：

1. 一个 Assistant 进程同时承载 Kratos HTTP 服务与 Temporal Worker
2. Assistant 分别检查 MySQL、Temporal、模型端点和进程内 Worker，并通过 HTTP API 返回安全的逐组件状态
3. Console 页面尚未接入；后续接入时复用现有应用外壳和登录身份，不创建独立前端或登录入口

当前只交付这条新运行骨架；对话、诊断、审批和自动执行尚未接入。旧会话 API、Redis Stream、进程内租约执行器和相关数据库表不再属于当前运行链路。Assistant 或其依赖不可用时，不影响配置管理和业务流量。

## 组件职责

| 组件 | 依赖 | 职责 |
| --- | --- | --- |
| Console | Admin API、Assistant | 托管控制台并代理后端请求；Assistant 页面尚未接入 |
| Admin API | API Server、Analytics | 提供面向 Console 的产品 API 和业务校验 |
| Assistant | MySQL、Temporal、模型端点 | 提供运维助手 HTTP 外壳、Worker 生命周期和真实依赖状态 |
| API Server | etcd | 提供声明式资源 API，是 etcd 的唯一访问者 |
| Controller | API Server | Watch 资源、编译 Envoy 配置、提供 xDS、回写 Status |
| Envoy | Controller、Authz、AI ExtProc、ALS | 接收业务流量并执行数据面配置 |
| Authz | API Server、Redis | 校验调用方与 Route 权限，执行共享请求限流 |
| AI ExtProc | API Server、Redis | 检查与结算 Token 额度，选择模型线路并转换协议 |
| ALS | Kafka、本地 WAL | 接收请求记录并可靠投递 |
| Analytics | Kafka、ClickHouse | 写入请求事实并提供分析查询 |

## 数据归属

| 数据 | 持久化位置 | 写入者 |
| --- | --- | --- |
| 声明式资源与 Status | etcd | API Server |
| Gateway Certificate 密钥材料 | etcd | API Server |
| 当前 Envoy 有效配置 | Controller 内存 | Controller |
| 请求限流 GCRA 状态 | Redis | Authz |
| 当前周期 Token 额度计数 | Redis | AI ExtProc |
| Assistant 后续功能的持久状态 | MySQL | Assistant |
| Assistant 工作流历史与任务队列 | MySQL | Temporal |
| ALS 待投递记录 | ALS 本地 WAL | ALS |
| 请求明细与模型调用 | ClickHouse | Analytics |
| 流量与模型用量聚合 | ClickHouse | ClickHouse 物化视图 |

## 部署边界

服务二进制、YAML 配置、健康检查、结构化日志和优雅退出保持部署方式中立。Docker Compose 只是当前正式支持的安装与联调方式。

Compose 中 Controller、Envoy、Authz、AI ExtProc 与 ALS 共享网络命名空间。xDS、鉴权、AI Processing 和访问日志链路由 Envoy 通过 loopback 连接，Authz 与 ALS 也只监听 loopback；AI ExtProc 同时承载 Admin API 的额度查询，因此其端口仍在 Compose 内部网络可达。API Server 自动生成的服务端证书通过只读 Volume 提供给 Admin API、Controller、Authz 和 AI ExtProc 校验。其他部署方式必须提供等价的网络隔离和 API Server 证书信任，或为内部 gRPC 连接配置传输安全。
