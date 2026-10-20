# 0017：客户端初始化与组件启动探测对齐

- 状态：已接受并实现，提交为 `fd124145`；下文保留本轮检查与测试修复记录。
- 日期：2026-10-19
- 规则来源：运行中的 GoCanon MCP，服务与规则均为 `880007d44098493410c4ce8e507cd383ff1a660c`，规则源 clean。

## 范围与原因

`internal/pkg/mysql.Open(dsn)` 同时配置 GORM、查询服务端版本和自动 Ping，组件无法用自己的 Context 控制这些探测；Admin API 的 Redis Ping 也没有独立的启动期限。本次按 MYSQL-001/002/003、REDIS-003 和 RUNTIME-008/009 分离初始化、探测与关闭，不新增客户端接口、运行框架或通用探测包。

本决策替代 [0006](0006-mysql-client-lifecycle.md) 中“保留 DSN-only、版本查询与自动 Ping”的阶段性边界，以及 [0007](0007-redis-client-lifecycle.md) 中“不增加启动 Ping 超时策略”的阶段性边界。两份决策的业务存储归属、原生客户端、DSN 规范化、Redis 模式、TLS 与关闭责任继续有效。

## 实现边界

- 复用 MCP 组件参考中的 MySQL Config、原生连接池配置与 GORM 初始化方式。`Open(Config)` 不探测数据库，不执行迁移；失败清理由共享包负责，成功后连接池交给组件 Run。
- 保留 `ParseDSN`，由客户端初始化和 Goose 迁移共同使用，继续统一 parseTime、UTC、禁止多语句及既有驱动选项；不复制第二套解析逻辑。
- Admin API 与 Controller Manager 分别把自己的 protobuf 配置转换为共享 Config。两者仍要求 MySQL 在启动前可用，取得连接池并安排关闭后，用各自配置的 `ping_timeout` 派生 Context 执行 PingContext。这里保留 Ingate 的既有启动策略，不照搬 GoCanon Controller Manager 不探测数据库的产品选择。
- Admin API 在 Redis 创建后立即安排关闭，再以 `redis.ping_timeout` 执行启动 Ping。超时、取消或探测失败都拒绝启动；不新增重试、降级或运行期健康策略。
- MySQL 配置显式包含连接池上限、空闲数、存活/空闲期限及启动探测期限；Redis 只新增启动探测期限，不改原生连接选项。结构约束由配置 proto 声明，公共 Go Config 保留独立调用边界的校验。探测期限不进入共享客户端 Config。

## 配置变化

两个组件的 MySQL 配置新增 `max_open_connections`、`max_idle_connections`、`connection_max_lifetime`、`connection_max_idle_time` 和 `ping_timeout`；Redis 新增 `ping_timeout`。部署配置须显式补齐，不做旧配置隐式回退。示例中的 20/10 连接、30 分钟/5 分钟存活期限和 3 秒探测期限是起点，部署时按数据库容量、组件实例数和启动预算调整，不作为通用容量结论。

配置通过 YAML 转 JSON 后使用 ProtoJSON 加载，Duration 必须写为 `1800s`、`300s`、`3s`，不能使用 Go 的 `30m`、`5m` 缩写。接入测试发现规则源中 MySQL 与可选认证模板也使用了不兼容的缩写；本次在 GoCanon 同步修复这两处模板，并用真实 ProtoJSON 解析覆盖 User、Book、认证三个生成分支，不修改运行中的 MCP。

不改变公开资源 API、管理员与 Session 行为、MySQL 表结构、迁移 SQL、查询投影或 Envoy 业务。现有进行中的 Envoy 改动与 docs/README.md 不在本次写入范围。

## 验证边界

使用原生驱动的独立 Dial 注册验证初始化不拨号、显式 Ping 才访问驱动及保留错误原因；配置测试验证示例、连接池边界和探测期限。运行既有格式、lint、Vet、普通测试与构建，并在隔离副本重生成配置 protobuf、核对可重复性。不连接真实 MySQL、Redis 或 Kafka；这些结果不代替部署中的启动、故障恢复和容量验证。

全仓库验证另发现 Backend 投影测试的 SQL 预期未包含现有 `tls` 字段；在尚未写入本次改动的原工作区可独立复现 `TestReconcileProjectsVersionAfterStatusOnlyUpdate/Backend`。该既有问题不归因于本次客户端变更，也不以本轮检查通过替代修复。

后续按用户“继续”的要求单独修复这一测试阻断：对齐 Backend 的插入列、更新列顺序及参数，并让现有 Backend 样例携带非空 TLS，以发现 API 到 MySQL 转换时丢失配置的回归。已有存储测试继续覆盖 TLS 设置与清空；未修改生产代码、SQL 匹配强度或 Envoy 工作区改动，不新增文件、接口或测试框架。
