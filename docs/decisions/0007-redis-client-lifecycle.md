# 0007：UniversalClient 初始化与登录态存储的职责分离

- 状态：已接受（用户已确认提交）
- 日期：2026-10-07

后续修订：[0017](0017-client-startup-probes.md) 为组件启动 Ping 增加显式期限；下文保留原阶段记录，原生客户端模式和业务存储边界不变。运行配置需补齐 `ping_timeout`，完整示例以 configs/admin-api.yaml 为准。

重构前，`internal/adminapi/repository/redis.Store` 同时解析连接 URL、构造客户端、执行启动 Ping、关闭连接池和读写登录态。启动策略或资源归属的变化因此需要修改业务存储包。初始重构仅移动这些职责；用户随后确认按原生 UniversalClient 扩展，并明确不兼容旧 URL 配置。本决策记录更新后的范围，登录态业务方法仍保留。

`internal/pkg/redis.New` 接收普通 Go Config，校验后构造 `redis.UniversalOptions`，调用 `redis.NewUniversalClient`，返回原生 `redis.UniversalClient`。不自建客户端类型、接口或探测方法，也不执行应用层启动 Ping；配置最小空闲连接数后，驱动可能后台建连。Admin API 的 `Run` 仍在 MySQL 初始化之后、初始管理员 Bootstrap 之前执行 Ping；探测失败时关闭客户端，清理失败不覆盖探测错误。探测成功后立即安排关闭，覆盖后续构造失败、服务运行失败和正常退出。`repository/redis.New` 接收原生通用接口，Store 不管理客户端生命周期。

依据为 GoCanon 已接受的 ADR 0048，以及 `go-redis/v9 v9.22.0` 的 `UniversalOptions`、`NewUniversalClient` 和 Sentinel 实现。模式选择交给原生工厂，不另建项目拓扑判断：

| 配置 | 原生客户端 |
| --- | --- |
| 单个地址，无 master_name，无 is_cluster_mode | 单节点 Client |
| master_name 非空，无 is_cluster_mode | Sentinel 发现主节点的 Client；addrs 是 Sentinel 地址 |
| 无 master_name，多地址或 is_cluster_mode | Redis Cluster 的 ClusterClient；addrs 是种子地址 |
| master_name 非空，is_cluster_mode 为 true | Sentinel 支持的 FailoverClusterClient；不是将 Sentinel 地址当 Redis Cluster 种子 |

配置直接使用 addrs、master_name、is_cluster_mode、db、Redis/Sentinel 认证、连接池、连接超时和 TLS 字段。旧 url 字段及编号保留为 protobuf reserved，旧 YAML 会因未知字段而拒绝，使用者必须更新配置；不提供隐式转换、旧 URL 构造函数或 Unix Socket 路径。空地址、负数配置、真实 Redis Cluster 的非零 db，以及会被忽略的 Sentinel 认证和 TLS ServerName 组合在构造前拒绝。Sentinel 支持的 FailoverClusterClient 可以使用非零 db，其上游连接钩子执行 SELECT。

数值及 Duration 零值沿用驱动默认值；显式 Duration 必须为正数，不开放禁用超时的负数哨兵值。TLS 使用系统可信根、证书验证与至少 TLS 1.2，不增加跳过验证选项。按 GoCanon 参考实现启用 ContextTimeoutEnabled，使网络读写期限考虑调用方 Context 的 deadline；这是相对旧客户端默认值的明确变化，不增加新的启动 Ping 超时策略。地址切片在选项边界复制，运行中的客户端不借用调用方可变配置。

业务 Store 仍负责原有 key 前缀、JSON 编码、绝对过期时间、缺失错误和删除语义；Repository、Service、鉴权、公开业务协议与依赖版本不变。新配置不开放 ReadOnly 或副本路由，会话仍从主节点读取。Sentinel 和 Cluster 的异步复制不能保证故障转移保留所有已确认写入，也不能据此保证 Logout 删除永不回退；改密后的账户版本校验仍按原有协议执行，不以 Redis 高可用替代该校验。

验证包括原生客户端选择、选项传递、地址切片隔离、非法配置拒绝、配置文件加载，以及现有会话创建、读取、删除和过期。配置 protobuf 重新生成并核对可重复性。真实组件测试由 `INGATE_REDIS_TEST_CONFIG` 提供普通 Go Config 的 JSON，运行 `go test -tags=integration -count=1 -v ./internal/adminapi/repository/redis`；未配置时跳过，不作为通过证据。测试只清理自身随机命名的键，不清库。单节点和 Sentinel 验证 JSON、删除、过去的绝对截止时间及实际过期；Redis Cluster 还要求测试键跨至少两个主节点，验证创建、读取和删除。负向测试检查错误的数据节点/Sentinel 密码被拒绝，阻塞读测试检查 Context deadline 生效。

本地 Redis 8.0.1 的单节点、带独立密码认证的 Sentinel、Sentinel FailoverClusterClient，以及三主节点 Cluster 的单/多种子接入已执行会话与路由检查。测试拓扑无副本，未验证主节点故障、自动切换、复制丢失或切换后的会话撤销；TLS 握手、命名 ACL 用户、MySQL 联合启动和组件退出也仍需独立验证。miniredis、客户端类型断言及本地连接成功均不能替代这些检查。

例如，默认示例保持单地址与 db 0。改为 Sentinel 时使用 Sentinel 地址和 master_name，而不是在 addrs 中填写主从 Redis 地址；改为 Redis Cluster 时使用种子地址，单种子必须设置 is_cluster_mode：

```yaml
# Sentinel：Redis 数据节点与 Sentinel 使用各自的认证。
redis:
  addrs: [sentinel-1:26379, sentinel-2:26379]
  master_name: ingate
  db: 0
  password: change-me-data
  sentinel_password: change-me-sentinel
```

```yaml
# Redis Cluster：可以只提供一个可发现其余节点的种子地址。
redis:
  addrs: [redis-seed:6379]
  is_cluster_mode: true
  db: 0
  password: change-me-data
```

参考：[锁定版本的 UniversalClient](https://github.com/redis/go-redis/blob/v9.22.0/universal.go)、[Sentinel 实现](https://github.com/redis/go-redis/blob/v9.22.0/sentinel.go)、[Redis Sentinel 的复制与故障边界](https://redis.io/docs/latest/operate/oss_and_stack/management/sentinel/)。
