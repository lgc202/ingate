# 0006：MySQL 客户端初始化与业务存储的职责分离

- 状态：已接受（用户已确认提交）
- 日期：2026-10-07

后续修订：[0017](0017-client-startup-probes.md) 将 DSN-only 和自动探测调整为完整 Config 与组件有界探测；下文保留原阶段记录，客户端归属和共享 DSN 解析结论继续有效。

重构前，Admin API 与 Controller Manager 都通过 `internal/adminapi/repository/mysql.Open` 创建连接。这个包同时保存账户和资源查询模型的表操作，连接初始化、业务访问和连接池关闭由同一个 Store 承担。

按已确认的重构计划，将客户端初始化移至 `internal/pkg/mysql`，返回原生 `*gorm.DB`。`repository/mysql.New` 只保存已构造的客户端，直接返回业务 Store；Store 不再持有独立的 SQL 连接池或提供 Open、Close。两个组件的运行入口通过 `db.DB()` 取得原生连接池，在继续组装依赖前安排关闭，覆盖后续构造失败、运行失败和正常退出。关闭错误仍由运行入口记录，保持当前返回语义。

本轮保留 Ingate 当前的 DSN 配置与初始化行为：驱动启用 `parseTime`、使用 UTC 并关闭 `multiStatements`；GORM 保留错误翻译、静默 SQL 日志和 UTC 微秒精度的时间函数。初始化仍由锁定版本的 MySQL Dialector 查询服务端版本，随后由 GORM 执行自动 Ping。版本查询或 Ping 失败时，GORM 负责关闭其创建的连接池；成功后连接池移交给组件运行入口。依据是 `gorm.io/gorm v1.31.2` 的 `Open` 与 `gorm.io/driver/mysql v1.6.0` 的 `Initialize`，两者版本由 `go.mod` 固定。

GoCanon 的已接受 ADR 0048 提供职责分离依据，但其参考组件采用显式探测和完整连接池配置。Ingate 本轮仅迁移已有初始化逻辑，不改变启动探测、增加配置字段或调整驱动默认值。Controller Manager 对查询模型业务 Store 的复用继续保留。数据库迁移仍由独立的 migration 包及 `-migrate` 入口执行；公开协议、Schema、查询投影与事务边界均沿用当前实现。

验证包括 DSN 规范化的输入输出、无效 DSN 的拒绝、受影响的账户与资源读取测试，以及阶段 1 的投影恢复测试。格式、静态检查和普通测试只能证明本次实际覆盖的部分，真实 MySQL 连接、事务和两个组件的运行退出仍需组件验证。

## DSN 解析统一

共享客户端与迁移程序原先各自解析 DSN，并分别设置 parseTime、UTC 和禁止多语句。这是同一组连接约束的两个维护点，目前行为一致；本次只消除重复维护，不调整连接策略。

将解析及这三项规范化集中到 `internal/pkg/mysql.ParseDSN`，返回驱动原生 `*mysql.Config`。GORM 初始化使用其 FormatDSN 结果，迁移程序直接交给原生 NewConnector；不新增配置结构、包装类型、客户端接口或依赖。解析错误保留原来的上下文和错误原因，解析不打开连接。迁移 SQL、Goose Provider、Ping、Up 和关闭错误聚合均保持不变。

测试覆盖默认及冲突配置、TCP/Unix Socket 选项、驱动公开字段及规范 DSN 的保留、FormatDSN 往返、返回配置的独立性，以及三类无效输入的原始原因。迁移入口的无效 DSN 测试在移动逻辑前后均通过。临时 Go overlay 分别移除三项规范化赋值，对应测试均失败；另一次 overlay 注入解析错误，确认 Open 和 migration.Up 都使用共享解析结果并保留错误原因，不修改仓库实现。真实 MySQL 连接和迁移 SQL 执行未验证，不据此声称 Schema 迁移或服务联合启动已通过。
