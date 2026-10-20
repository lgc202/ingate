# 0005：Admin API 管理网关资源

> 历史决策保留当时的 Route/Upstream 契约和 Domain 布局；HTTPRoute/Backend 替换见 [0011](0011-http-resource-contract.md)，当前资源名称、查询表及版本化写入/Repository 查询视图边界见 [0012](0012-resource-names-and-query-schema.md)。

- 状态：已实现；下文保留首个三资源切片的历史记录。
- 日期：2026-09-26

Admin API 对 Gateway、Route、Upstream 分别提供创建、读取、列表、更新和删除接口，HTTP 路径位于 `/api/admin/v1` 下；同一协议也提供 gRPC。调用方使用 Admin API 的管理员 Session。创建、更新和删除通过生成的客户端写入声明式 API Server；Controller 将资源同步到 MySQL，Admin API 的读取和列表只查 MySQL。API Server 是期望状态的唯一写入源，MySQL 是可重建的查询模型，不接受 Admin API 直接修改资源。

公开资源只包含当前管理界面需要的名称、可编辑配置、创建时间和 `version`。`version` 对应 API Server 的 `metadata.resourceVersion`，Controller 连同配置一起投影到 MySQL。调用方执行 `PUT` 时必须带回读取到的版本。Repository 从 API Server 重新读取现有对象并核对版本，只替换 `spec`，再以该对象执行更新：这样既能保留 Admin API 未暴露的元数据，也能由 API Server 检查读取之后发生的并发更新。版本不一致返回冲突；客户端应等待投影追上或重新读取后决定是否重试。Controller 更新 Status 也会改变资源版本，因此这类冲突不一定意味着另一个管理员改了配置。为了让查询版本最终追上 API Server，投影 Controller 也处理 Status 更新；这是 Ingate 的更新契约相对 GoCanon Gateway 查询模型的明确扩展。

`PUT` 是可编辑配置的完整替换，不是部分修改。资源名称不能通过更新改变；Controller 的 Status 不在公开写入契约里。删除按名称执行，不附带版本前提，也不尝试在 Admin API 内阻止删除正在被 Route 引用的 Gateway 或 Upstream；引用缺失由 Controller 按现有裁决规则反映在 Route 状态中。

查询模型分别保存三种资源的主记录及 Gateway Listener、Upstream Endpoint 子记录。主表和子表使用数据库内部 `id` 关联，不建立数据库外键；对外稳定名称仍是资源名。Listener 顺序没有业务含义，按名称返回；Endpoint 以地址和端口确定身份，同一 Upstream 内不允许重复。API Server 写入前按地址和端口整理 Endpoint，MySQL 和公开响应也按相同顺序返回；单纯调换端点顺序不改变 `generation`。MySQL 的版本字段（当时名为 `resource_version`，现为 `version`）只服务 Ingate 的产品更新令牌，不用于增量同步检查点。Repository 将 API Server 或 MySQL 的存储表示转换为不带 Kubernetes 元数据和 GORM 标签的 Domain 类型，Handler 再转换为公开 protobuf。

三个独立投影 Controller 分别按资源名同步。单个资源及其子项的写入在 MySQL 事务中完成；Manager 启动时先逐条补齐 API Server 的资源，再按名称删除 MySQL 中的残留。每个 Controller 只有一个 Worker，同进程内的启动对账与普通协调串行；失败时保留已完成的记录并由队列重试。不做整表删除重建，也不引入同步检查点或首次同步就绪标记。当前仍按单个 Controller Manager 实例部署，多副本写入不在本次支持范围。Controller Manager 通过 `--config` 加载自己的 protobuf Bootstrap；它与 Admin API 必须使用同一已迁移 MySQL 业务库。

列表使用 `page_size` 与从 1 开始的 `page_num`，返回 `total_count`。默认单页 50 条，上限 200 条，MySQL 按名称排序后按偏移分页。一次 Get 或 List 内的父子表查询使用可重复读事务；不同页之间不保持快照，翻页期间若资源发生增删，可能重复或跳过记录。对管理界面而言接受这一边界；需要稳定遍历全量数据的后台任务不应使用此分页接口。

写入 API Server 成功后，Controller 异步更新 MySQL。因而创建后立即 Get/List 可能暂时看不到新资源，更新后可能读到旧版本，删除后可能暂时读到旧记录；Admin API 不从 API Server 回退查询，也不声称读后立即一致。Controller 暂停或 MySQL 不可用会延长延迟，投影协调报错后由工作队列重试。Manager readiness 仍表示 xDS 是否就绪，不承诺 MySQL 投影已经追平；需要另行监控同步错误与延迟。

当前 HTTP 合同测试使用生成的 fake client 和模拟查询模型验证路由、协议转换、版本冲突、元数据保留、删除和公开错误。投影包使用 controller-runtime fake client 与 sqlmock 直接调用三种 Reconciler，验证 Status 单独更新后的资源版本投影、启动补齐与残留清理、空资源集合、读取失败，以及写入或部分清理失败后再次协调的结果。SQL 期望同时检查资源字段和 Gateway Listener、Upstream Endpoint 子记录；这些测试不执行 API Server 的真实字段校验，也不证明真实 MySQL 事务、Manager 的首次入队、Watch 过滤或工作队列自动重试。三种资源经真实 API Server、Controller、MySQL、Admin API 和 Envoy 的端到端联动仍需验证。
