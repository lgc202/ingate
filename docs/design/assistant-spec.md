# Ingate AI 助手技术规格

## 文档状态

- 状态：已确认的完整目标架构
- 对应 PRD：[Ingate AI 助手产品需求文档](assistant-prd.md)
- 实施基线：`codex/assistant-skeleton` 分支的 `768bb593`
- 当前实现：[当前范围](../src/content/docs/reference/current-scope.md)与[运维助手运行说明](../src/content/docs/operations/assistant.md)

> 本规格是后续 22 个实施阶段的共同契约，不是当前实现清单。基线只完成单进程 Kratos HTTP、Temporal Worker、MySQL/Temporal/模型就绪检查和 Compose 接线；Console、业务表和完整工作流尚未交付。

## 1. 摘要

### 1.1 覆盖范围

本规格覆盖 Assistant 的进程边界、依赖方向、持久任务、Agent、工具、证据、知识、计划、风险、审批、执行、验证、补偿、记忆、审计、API、安全和验证策略。

Assistant 是可选控制面辅助组件。它可以跨多套独立 Ingate 安装查询和执行受控动作，但不能改变以下系统边界：

- 一套 Ingate 仍对应一个环境和配置域；
- Envoy 仍是唯一数据平面；
- 声明式资源仍是配置事实来源；
- 所有变更仍经目标 Ingate 的 Admin API、API Server、Controller 和 xDS 链路；
- Assistant 故障不影响配置管理和业务流量。

### 1.2 PRD 追踪

本规格实现 [US-001–US-019](assistant-prd.md#4-用户故事) 和 [FR-1–FR-64](assistant-prd.md#5-功能需求)。各实施阶段在第 12 节映射到对应故事、需求和章节。

### 1.3 核心决策

1. 使用 Greenfield 实现，不兼容旧 Assistant API、Redis Stream、进程内状态机和旧数据表。
2. 默认运行形态是单个 `ingate-assistant --role=all` 进程，同时承载 Kratos HTTP、Temporal Worker 和必要后台循环。
3. MySQL 是产品业务数据的权威来源；Temporal 是唯一持久任务编排器，进程内状态不得决定任务进度。
4. Eino 用于 Agent Activity 内部的模型与工具循环，Eino 类型不得越过 `agent` 包边界；首版不引入 LangChainGo。
5. Workflow 只编排确定性值和 Activity，不直接访问模型、MySQL、网络、时钟随机源或凭证。
6. 所有查询和动作通过服务端注册的版本化工具完成；模型不能提交任意 URL、命令或动态工具实现。
7. 风险、权限、审批、幂等、网络和秘密边界由模型之外的确定性代码执行。
8. R0–R3 取所有风险来源的最高值；未知属性 fail closed，R2 单人批准，R3 双人不同角色批准。
9. 写入结果不确定时进入 `UNKNOWN` 并 Inspect，禁止盲目重试 Execute。
10. Console 复用现有 Shell、导航和登录身份；产品协议只使用 Gateway、Route、Service 等领域概念。
11. Qdrant 只是可选向量索引；MySQL 权限过滤后的关键词检索是降级路径。
12. 首版不拆 Kubernetes 角色、不支持用户自定义 MCP，也不建立第二套前端或身份系统。

## 2. 架构

### 2.1 系统上下文与默认部署

```text
Browser
   |
   v
Console ── signed identity ──> Assistant HTTP
                                  |
                  +---------------+----------------+
                  |               |                |
                  v               v                v
                MySQL          Temporal         Model endpoint
                                  |
                                  v
                           Assistant Worker
                         /         |          \
                        v          v           v
                  Knowledge     Agent        Tool boundary
                     |          (Eino)          |
                  Qdrant?                         v
                                         registered Ingate
                                           Admin APIs
```

默认 Compose 启动一个 Assistant 实例。HTTP 和 Worker 共用配置、日志、依赖客户端和优雅退出生命周期。Temporal 与 MySQL 是必需依赖；模型端点是处理任务的必需依赖；Qdrant 是可选依赖。Assistant 不接入 Envoy 同步请求链路。

### 2.2 组件

| 组件 | 职责 | 禁止事项 |
| --- | --- | --- |
| Console | 会话、证据、计划、审批、执行和审计界面 | 不直接连接 Temporal、MySQL 或目标 Ingate |
| Assistant HTTP | 认证、协议校验、命令接收、查询和 SSE | 不同步运行长任务或直接执行工具 |
| Usecase | 环境授权、会话、计划、治理、审批、记忆和审计规则 | 不依赖 HTTP 或具体 SDK |
| Temporal Workflow | 持久编排任务、DAG、Timer、Signal 和 Child Workflow | 不进行网络、模型、数据库或秘密访问 |
| Activity | 调用 Usecase、Agent、Knowledge 与 Tool，并报告 Heartbeat | 不绕开幂等、风险和审计边界 |
| Agent | 在角色配置内运行模型/工具循环并返回结构化结果 | 不返回 Eino 类型，不直接获得动作执行能力 |
| Tool Registry | 解析不可变工具版本、Schema、风险下限和启用状态 | 不接受模型动态注册工具 |
| Ingate Tool | 通过目标 Admin API 查询或变更产品资源 | 不直连 API Server、etcd、Controller 或 Envoy |
| Knowledge | 导入、分块、权限过滤、检索和索引同步 | 不把文档指令当作系统指令 |
| MySQL | 保存业务事实、事件、Outbox、幂等和审计 | 不保存明文外部凭证 |
| Qdrant | 保存可重建的向量索引 | 不决定权限，不成为内容事实来源 |
| Reconciler | 重试 Outbox，修复 MySQL 投影与 Temporal 的可确认差异 | 不猜测外部动作结果 |

### 2.3 主要交互

创建任务的链路如下：

1. HTTP 验证身份、环境范围、输入和 `Idempotency-Key`。
2. 一个 MySQL 事务写入用户消息、Task、首个持久事件和 Outbox。
3. Dispatcher 用确定性的 Workflow ID 启动或 Signal Temporal；重复投递由 Workflow 命令 ID 去重。
4. Workflow 调度 Activity，Agent Activity 在受限工具集合内运行 Eino。
5. Activity 把结构化结果和持久事件写入 MySQL；Workflow 只持有引用、Digest 和编排状态。
6. SSE 从 MySQL 补发持久事件，并把当前连接期间的临时 Token 增量转发给浏览器。
7. 最终消息持久化后，即使 Token 流中断也能完整恢复。

动作链路如下：

1. Planner 产生结构化计划，服务端校验 DAG、Schema、权限和工具版本。
2. Prepare 与 Dry Run 生成不可变 `Action Digest`，风险服务确定 R0–R3。
3. R1 仅在策略显式允许时继续；R2/R3 等待满足绑定条件的批准。
4. 执行前重新检查身份、权限、环境修订、资源版本、策略和前置条件。
5. 在关键审计和 attempt 持久化成功后，Activity 才能解析凭证引用并调用目标 Admin API。
6. Inspect 和 Verify 独立确认状态；失败生成受治理补偿，歧义结果进入 `UNKNOWN`。

Knowledge Activity 先在 MySQL 过滤可见文档，再使用 Qdrant 返回的候选 ID 做二次权限校验。Qdrant 不可用时直接走 MySQL 全文或关键词检索。任何片段只有在权限和来源字段完整时才能成为 Evidence。

### 2.4 依赖方向与代码组织

Assistant 使用 Kratos 处理进程、HTTP、配置和依赖装配，但不会把所有领域能力机械塞进 `service/biz/data` 三个泛化包。依赖方向固定如下：

```text
api -> service -> biz <- data
                  ^
workflow -> activity
                  |
       agent / knowledge / tool
                  |
      model / qdrant / ingate clients
```

- `service` 只做协议适配、输入格式校验和 DTO 转换。
- `biz` 持有聚合、用例、消费者侧接口和确定性业务规则。
- `data` 实现 MySQL、模型、Qdrant、目标 Ingate 和凭证解析等外部边界。
- `workflow` 只放确定性的 Temporal Workflow、Signal/Query 值和编排状态。
- `activity` 是 Workflow 到副作用边界的适配层，不复制业务规则。
- `agent` 封装 Eino；其公开输入输出是 Assistant 自有类型。
- `tool` 保存版本化工具契约和具体领域工具，不承载通用工作流。

目标目录按真实能力逐步出现，不预建空包：

```text
api/assistant/v1/
cmd/ingate-assistant/
internal/assistant/
  conf/
  server/
  service/
  biz/
  data/
  workflow/
  activity/
  agent/
  tool/
  knowledge/
web/console/src/features/assistant/
```

接口定义在消费者侧，只为真实外部边界存在。禁止只为测试抽接口，禁止创建无人使用的 `manager`、`runtime`、`engine` 或 `processor`。

## 3. 运行时与应用设计

### 3.1 Kratos 外壳

`cmd/ingate-assistant` 只负责解析 `--config` 与 `--role`、加载配置和运行应用。首版 `--role` 只接受 `all`。

Kratos App 同时注册 HTTP Server 与 Worker Server。启动顺序为配置校验、日志、MySQL、Temporal、模型端点、HTTP、Worker；退出时先停止接收新 HTTP 请求和 Worker 新任务，在安全期限内等待 Activity，再释放客户端。`worker_stop_timeout` 必须短于整体 `shutdown_timeout`。

`/healthz` 只表示进程存活；`/readyz` 和 `/assistant/v1/system/readiness` 分别检查 MySQL、Temporal、模型端点和进程内 Worker。对话阶段引入 Qdrant 后，其失败只标记检索降级，不让整体进程不可用。

### 3.2 配置、装配与生命周期

配置按职责分组：

- `server`：监听地址、请求与关闭期限、SSE 心跳；
- `mysql`：连接、连接池、事务期限；
- `temporal`：地址、Namespace、Task Queue、连接和 Worker 停止期限；
- `model`：模型 API 地址、健康地址、模型名、超时、凭证引用；
- `knowledge`：分块限制、Qdrant 可选连接、同步并发；
- `execution`：Workflow、Activity、Agent、DAG 和全局并发限制；
- `security`：身份断言、公钥、出站允许目标、脱敏规则；
- `retention`：消息、证据、事件、审计和导出保留期。

配置启动时一次性完整校验。秘密配置只接受环境变量或秘密引用，不允许在配置展示 API 中返回实际值。Wire ProviderSet 保持在各包入口，cleanup 由 Kratos 生命周期统一执行。

### 3.3 业务与副作用边界

HTTP 写请求只提交命令，不等待 Workflow 完成。Usecase 在 MySQL 事务中完成身份范围、版本、幂等和业务事实写入；Outbox 是开始或唤醒 Workflow 的唯一入口。

Activity 在每次外部访问前获取最新授权快照。长 Activity 使用 Heartbeat 记录可恢复位置，但 Heartbeat 不保存秘密或完整外部响应。只有 Tool adapter 能接触目标连接和短期凭证；只有 Agent adapter 能接触 Eino 与模型 SDK。

### 3.4 Temporal Workflow

根 `TaskWorkflow` 的输入只包含 `task_id`、`environment_id`、`principal_id`、初始命令 ID 和配置版本引用。Workflow 负责：

- 消费去重后的用户命令；
- 调度 Agent、知识、查询和动作 Activity；
- 保存当前计划版本、DAG frontier、等待条件和 Timer；
- 以 Child Workflow 运行专业 Agent；
- 在安全边界应用暂停、恢复和取消；
- 等待审批并在过期时使决定失效；
- 根据 Activity 的明确结果进入验证、补偿或未知核对。

Workflow 代码必须保持 Temporal 可重放确定性。版本升级使用 Temporal 的 Workflow versioning 或 Worker deployment versioning，不依赖包级变量和当前系统时间改变历史分支。

### 3.5 Worker 与 Activity

`server/worker.go` 只负责把 Temporal Worker 接入 Kratos 生命周期；Workflow 和 Activity 不属于 server。Worker 注册明确命名且有独立重试策略的 Activity：

- `PersistTaskEvent`
- `RunAgent`
- `SearchKnowledge`
- `QueryIngate`
- `PrepareAction`
- `ExecuteAction`
- `InspectAction`
- `VerifyAction`
- `PrepareCompensation`
- `PersistAuditEvent`

命名只是协议标识，具体实现按领域组织。查询 Activity 可在短暂网络错误后有界重试；写入 Activity 只有在能证明未提交或由外部幂等协议保证时才自动重试。所有返回值有大小上限，大结果写 MySQL 后只把引用返回 Workflow。

### 3.6 Agent 边界

Assistant 自有的 Agent 契约为：

```go
type Request struct {
	TaskID      string
	Role        Role
	Goal        string
	EvidenceIDs []string
	ToolRefs    []ToolRef
	Budget      Budget
}

type Result struct {
	Kind       ResultKind
	Content    string
	Data       json.RawMessage
	Citations  []EvidenceRef
	Usage      Usage
	Unknowns   []Unknown
}
```

具体字段以实现时的领域类型为准，但边界必须满足：Eino Message、Tool、Callback、Runnable 等类型不出 `agent` 包；输出必须通过角色对应的 JSON Schema；连续两次 Schema 失败后返回 `MODEL_OUTPUT_INVALID` 并保留已有证据。

模型流式 Token 只发给当前 SSE 订阅者，不写 Task Event；完成后的文本和结构化结果才写消息与事件。模型输入由可信系统指令、用户数据、权限过滤后的 Evidence 和工具定义分区构造，不把外部内容拼进系统指令。

### 3.7 Agent 角色与协作

六个角色共享一个 Agent 实现，通过不可由模型修改的配置限制 Prompt、工具、Schema 和预算：

| 角色 | 产出 | 可用工具 |
| --- | --- | --- |
| General | 对话分流、简单回答、复杂度判断 | 最小只读集合 |
| Planner | 版本化计划草案 | 工具元数据和只读证据，不含 Execute |
| Researcher | 知识与实时事实集合 | Knowledge 与查询工具 |
| Diagnostician | 候选原因、置信度、未知项 | 查询工具 |
| SecurityReviewer | 风险属性和安全异议 | 查询工具与只读策略 |
| Verifier | 冲突核对和独立验证建议 | 查询工具 |

专业角色由根 Workflow 按复杂度按需启动为 Child Workflow。单任务最多并发三个 Agent；每个 Agent 最多八轮工具调用，并分别应用 Token、时间和输出大小预算。子分支失败不抹除其他结果，无法消解的冲突以结构化异议返回用户。

### 3.8 工具契约

工具定义是不可变版本：

```go
type Definition struct {
	Name           string
	Version        string
	Kind           Kind
	InputSchema    json.RawMessage
	OutputSchema   json.RawMessage
	MinimumRisk    RiskLevel
	Recoverability Recoverability
}

type QueryTool interface {
	Definition() Definition
	Query(context.Context, QueryRequest) (Evidence, error)
}

type ActionTool interface {
	Definition() Definition
	Prepare(context.Context, ActionRequest) (PreparedAction, error)
	DryRun(context.Context, PreparedAction) (Preview, error)
	Execute(context.Context, ApprovedAction) (ExecutionResult, error)
	Inspect(context.Context, InspectionRequest) (InspectionResult, error)
	Compensate(context.Context, CompensationRequest) (ExecutionResult, error)
}
```

`PreparedAction` 必须包含规范化输入、目标身份、环境修订、资源版本、前置条件、预期影响、验证条件和确定性的 Action Digest。调用 `Execute` 的值必须是风险与审批服务产生的 `ApprovedAction`，Agent 不能构造。

首批查询工具覆盖 Gateway、Route、Service 读取与关系。首批动作工具依次交付：非生产未引用 Service 创建、生产 Route 的 Service 绑定更新、隔离生产环境空 Gateway 删除。工具只调用注册环境的 Admin API 产品协议。

## 4. 持久任务模型

### 4.1 聚合与实体

| 聚合或实体 | 关键职责 |
| --- | --- |
| Session | 归属用户与环境，聚合有序消息和 Task |
| Task | 保存目标、当前状态、Workflow 身份、当前 Plan 与最终结果 |
| TaskCommand | 表达用户输入、暂停、恢复、取消和审批等幂等命令 |
| TaskEvent | 单调序号的持久产品事件，用于查询、SSE 补发和审计关联 |
| Evidence | 保存来源、可信级别、版本、时间、权限和内容引用 |
| Plan | 保存不可变版本和 DAG；只有一个版本可处于当前有效状态 |
| Action | 计划步骤中的规范化副作用提案与 Digest |
| ActionAttempt | 一次外部执行尝试及其幂等、状态、证据和未知结果 |
| Approval | 对特定 Binding 的用户决定和有效期 |
| Verification | 对版本化断言的独立验证结果 |
| Compensation | 引用原动作、恢复目标和治理结果的普通动作 |

### 4.2 状态

Task 状态：

| 状态 | 含义 |
| --- | --- |
| `PENDING_DISPATCH` | MySQL 已接收，等待启动或唤醒 Workflow |
| `RUNNING` | 正在理解、检索、规划或执行可运行步骤 |
| `WAITING_INPUT` | 证据不足，等待用户补充 |
| `WAITING_APPROVAL` | 当前 DAG frontier 需要有效批准 |
| `PAUSED` | 已在安全边界暂停并保存来源阶段 |
| `VERIFYING` | 外部请求已有明确结果，正在独立验证 |
| `COMPENSATING` | 原目标失败，正在执行受治理补偿 |
| `SUCCEEDED` | 目标与验证均成功 |
| `FAILED` | 已明确失败且没有可继续分支 |
| `CANCELLED` | 已停止新工作；既有副作用和证据仍保留 |

ActionAttempt 状态：`PENDING`、`PREPARING`、`READY`、`WAITING_APPROVAL`、`EXECUTING`、`UNKNOWN`、`INSPECTING`、`VERIFYING`、`COMPENSATING`、`SUCCEEDED`、`FAILED`、`SKIPPED`、`CANCELLED`。

`UNKNOWN` 不是普通错误：它表示系统不能证明外部副作用发生或未发生。该状态阻塞所有依赖步骤，直到 Inspect 得出明确结论或任务安全暂停等待人工处理。

### 4.3 转换规则

- `PENDING_DISPATCH -> RUNNING` 只在确定性 Workflow ID 已启动或已存在时发生。
- `RUNNING -> WAITING_INPUT` 在证据不足且需要用户决定时发生；新输入创建命令并恢复原阶段。
- `RUNNING -> WAITING_APPROVAL` 在 DAG frontier 出现 R2/R3 动作时发生；不依赖该动作的安全分支仍可运行。
- `RUNNING/WAITING_* -> PAUSED` 只在安全边界生效，并保存暂停前阶段与 DAG frontier。
- `PAUSED -> previous state` 只接受授权恢复命令，不从任务开头重放。
- 任意非终态接收取消后停止调度新动作；不可安全取消的写入先形成明确或 `UNKNOWN` 结果。
- `EXECUTING -> VERIFYING` 只在 Execute 或 Inspect 明确确认动作已生效后发生。
- `EXECUTING -> UNKNOWN` 在超时、连接中断或响应非法且不能证明未产生副作用时发生。
- `VERIFYING -> COMPENSATING` 只在验证失败且已生成补偿提案后发生。
- 终态不可逆；后续补充工作创建新 Task，不改写历史终态。

每个转换同时写 Task 投影和 TaskEvent。事件具有 `(task_id, sequence)` 唯一约束，终态事件每个 Task 只能出现一次。

### 4.4 Workflow 消息

所有外部消息先写 MySQL `TaskCommand` 与 Outbox，再投递到 Temporal。消息包含稳定 `command_id`，Workflow 保存已消费命令集合或有界去重标记。

| 消息 | 载荷引用 | 行为 |
| --- | --- | --- |
| `UserMessageSubmitted` | message ID | 开始或继续理解目标 |
| `PlanRevisionRequested` | constraint message ID、base version | 生成新版本，使旧 Binding 失效 |
| `PlanExecutionRequested` | plan ID、version | 校验当前版本后进入风险与执行 |
| `ApprovalDecided` | approval ID、binding digest | 重新加载并验证批准条件 |
| `TaskPauseRequested` | command ID | 在下一个安全边界暂停 |
| `TaskResumeRequested` | command ID | 回到保存的来源状态 |
| `TaskCancelRequested` | command ID | 停止新动作并传播可取消上下文 |

Workflow Query 只用于内部运维确认实时编排状态，不是 Console 的产品数据源。Console 始终查询 MySQL 投影。Temporal 历史中只保存 ID、版本、Digest、枚举和有界错误分类，不保存凭证、完整文档、完整工具响应或用户敏感请求。

## 5. 数据与一致性

### 5.1 MySQL 模型

使用 MySQL 8.0 和 InnoDB。所有业务表使用不可变 ID、`created_at`、`updated_at` 与乐观并发 `version`；JSON 字段必须在写入前通过具体 Schema 校验，不能成为无约束数据袋。

| 表 | 关键字段与约束 |
| --- | --- |
| `assistant_environments` | id、name、Admin API 地址、credential_ref、sensitivity、revision、verified_at、status；地址唯一 |
| `assistant_environment_grants` | environment_id、principal_id、role、version；组合唯一 |
| `assistant_sessions` | id、owner_id、environment_id、title、version；环境不可原地切换 |
| `assistant_messages` | session_id、sequence、role、content_ref/content、task_id；序号唯一 |
| `assistant_tasks` | session_id、workflow_id、status、active_plan_version、source_state、version |
| `assistant_task_commands` | task_id、command_id、kind、payload_ref、actor_id；command_id 唯一 |
| `assistant_task_events` | task_id、sequence、kind、safe_payload、occurred_at；序号唯一 |
| `assistant_evidence` | source_kind、source_id、environment_id、resource_version、captured_at、trust、content_ref、acl_digest |
| `assistant_plans` | task_id、version、environment_revision、status、digest；任务版本唯一 |
| `assistant_plan_steps` | plan_id、step_id、dependencies、tool_ref、action_digest、verification_schema |
| `assistant_tool_definitions` | name、version、kind、schemas、minimum_risk、recoverability、enabled；名称版本唯一 |
| `assistant_policy_versions` | scope、version、rules、digest、created_by；范围版本唯一 |
| `assistant_risk_decisions` | action_id、inputs、level、reasons、binding_digest |
| `assistant_approvals` | action_id、binding_digest、actor_id、actor_role、decision、expires_at；决定幂等唯一 |
| `assistant_action_attempts` | action_id、attempt、idempotency_key、status、request_digest、result_ref、unknown_reason |
| `assistant_verifications` | action_id、condition_version、status、evidence_ids、observed_at |
| `assistant_compensations` | original_action_id、action_id、target_state、status |
| `assistant_knowledge_sources` | scope、connector、external_id、sync_status、version、acl |
| `assistant_documents` | source_id、external_id、version、checksum、source_uri、captured_at、acl |
| `assistant_document_chunks` | document_id、ordinal、content、search_text、acl_digest；文档序号唯一 |
| `assistant_memories` | scope、owner/scope_id、version、content_ciphertext、source_task_id、status |
| `assistant_memory_usages` | memory_id、version、task_id、used_at |
| `assistant_audit_streams` | environment_id、period、last_sequence、last_digest |
| `assistant_audit_events` | stream_id、sequence、kind、actor、safe_payload、previous_digest、digest |
| `assistant_exports` | requester、filter、status、object_ref、expires_at |
| `assistant_outbox` | event_id、aggregate、kind、payload_ref、attempts、available_at、delivered_at |

大内容可以放到受访问控制的对象存储；首个紧凑版本可继续放 MySQL，但业务记录只通过 `content_ref` 访问，大小与保留期一致。对象存储不是首版必需依赖。

### 5.2 知识与检索

知识同步状态为 `pending -> running -> succeeded|failed`。同步过程先写 MySQL 文档和分块，再异步更新 Qdrant。删除时先在 MySQL 标记不可检索，再清理索引，避免删除窗口继续命中。

检索顺序：

1. 根据当前主体、组织和环境得出可见文档集合；
2. 执行 MySQL 关键词检索，或将可见集合过滤条件传给 Qdrant；
3. 对 Qdrant 候选 ID 回查 MySQL 并再次验证 ACL、版本和删除状态；
4. 将片段规范化为 Evidence，应用大小、数量和 Token 预算；
5. 记录被采用的 Evidence ID，不记录无关全文。

初始分块以可重复的文本边界和版本化参数完成。嵌入模型、分块参数或 ACL 变化会创建新索引版本，切换成功前旧索引仍服务；MySQL 始终是内容和权限事实来源。

### 5.3 记忆与审计

记忆候选只来自终态任务中的已验证事实。保存前执行秘密、凭证、完整请求和无来源推测扫描，并向用户展示最终内容、来源、用途和作用域。组织记忆需要相应管理权限。

记忆编辑创建新版本；每次注入模型上下文写 `assistant_memory_usages`。删除先使记录不可检索，再物理删除密文或销毁独立数据密钥，并清理全文/向量索引。审计仅保留“谁在何时删除了哪条记忆”的元数据和 Digest，不保留已删除正文。

审计事件按环境和时间分片形成摘要链：

```text
digest = SHA-256(previous_digest || canonical_safe_event)
```

写入时锁定对应 `assistant_audit_streams` 行，分配序号并更新尾摘要。产品 API 不提供 Update/Delete。离线或导出校验从已知锚点重算 Digest；该机制用于发现篡改，不替代数据库访问控制和备份。

### 5.4 Outbox、幂等与 Reconciler

MySQL 与 Temporal 不做分布式事务。所有需要启动或唤醒 Workflow 的产品写入，在同一个 MySQL 事务中写业务事实和 Outbox。Dispatcher 至少一次投递，Temporal Workflow ID 和 command ID 实现接收端幂等。

动作幂等分三层：

1. HTTP 用 `Idempotency-Key` 去重用户命令；
2. Workflow 用 command ID、action ID 和 DAG 状态去重调度；
3. Tool 用稳定 action idempotency key 和 attempt 记录防止重复外部副作用。

Reconciler 周期扫描未投递 Outbox、长期 `PENDING_DISPATCH`、活跃 Task 与 Temporal Describe/Query 的可确认差异。它可以重新投递命令、重建缺失投影或标记需要人工检查，但不能根据超时猜测动作失败，也不能自动重放 `UNKNOWN` 的 Execute。

## 6. 执行治理

### 6.1 R0–R3 风险等级

| 等级 | 含义 | 默认决策 | 示例 |
| --- | --- | --- | --- |
| R0 | 只读且无外部副作用 | 无审批 | 查询 Gateway/Route/Service、知识检索 |
| R1 | 非生产、有限影响、可独立验证且可恢复 | 仅策略显式允许时自动 | 创建未被引用的测试 Service |
| R2 | 影响生产流量或具有中等范围副作用 | 一名授权审批人 | 修改生产 Route 的 Service 绑定 |
| R3 | 删除、不可逆、安全敏感、大范围或属性未知 | 两名不同用户，其中一名安全/安装管理员 | 删除生产 Gateway、未知可恢复性 |

工具风险下限和默认策略只能提高，不能由模型降低。安全管理员可以通过新策略版本进一步提高等级，但不能把 R3 类动作降为 R1。

### 6.2 确定性风险计算

风险输入为：

- 工具版本声明的最低等级；
- 动作类型；
- 环境敏感度；
- 资源敏感度；
- 影响范围；
- 可恢复性；
- 当前策略版本。

计算规则为所有已验证来源的最大值：

```text
finalRisk = max(toolFloor, actionRisk, environmentRisk,
                resourceRisk, blastRadiusRisk,
                recoverabilityRisk, policyFloor)
```

未知工具直接拒绝。动作必需属性、环境修订、可恢复性或策略版本未知时返回 `RISK_UNCLASSIFIED` 并按 R3 治理，不得默认放行。模型建议只能作为待验证属性输入。

### 6.3 动作生命周期

1. **Prepare**：规范化输入，读取当前对象，固定目标、版本、前置条件、验证条件和恢复目标；无副作用。
2. **Dry Run**：使用目标 API 的预检能力或本地 Schema/引用校验产生差异；无副作用。
3. **Risk Decision**：服务端计算等级与理由并生成 Binding。
4. **Approval**：R2/R3 等待符合角色、有效期和 Binding 的决定。
5. **Preconditions**：执行前重新校验身份、权限、环境修订、资源版本、策略和 Action Digest。
6. **Audit Intent**：持久化最小动作意图和 attempt；失败则 fail closed。
7. **Execute**：在工具边界解析短期凭证并调用目标 Admin API。
8. **Inspect**：使用独立 Get/查询核对目标身份、幂等键、版本和关键字段。
9. **Verify**：根据计划绑定的断言确认业务目标，而非只确认请求成功。
10. **Compensate**：验证失败时生成新的受治理动作；不执行隐式回滚。

DAG 调度只推进依赖已成功的步骤。失败后，只有显式 `continue_on` 规则允许的分支继续；其余不可达步骤标记 `SKIPPED`。

### 6.4 审批绑定

Binding 使用规范化编码计算 Digest，至少包含：

- Task ID；
- Plan ID 和版本；
- Action ID 和 Action Digest；
- Tool 名称和版本；
- Environment ID 和修订；
- 目标资源 ID 和版本；
- Policy ID 和版本；
- Risk Level 和理由 Digest；
- 验证条件版本；
- 过期时间。

任一字段变化都会创建新 Binding 并使旧批准失效。R2 需要一名当前具备目标环境审批权限的用户；R3 需要两个不同用户，其中至少一人具备 SecurityAdmin 或 InstallationAdmin。发起人批准可以记录，但不能单独满足 R3。R3 最长有效期为 15 分钟。

批准、拒绝和要求修改都使用幂等键与唯一约束。第二个批准到达后仍需重新验证两名审批人的当前权限和全部 Binding。要求修改回到规划阶段并创建新 Plan 版本。

### 6.5 未知结果与补偿

外部写入出现超时、连接中断或无法解析响应时：

1. 若工具能证明请求未发出，attempt 可明确失败并按策略重试。
2. 若可能已产生副作用，attempt 进入 `UNKNOWN`，阻塞依赖步骤。
3. Workflow 调用同一工具版本的 Inspect，以目标身份、幂等键、Action Digest 和预期状态核对。
4. Inspect 确认已生效时，原 attempt 转为明确成功并继续 Verify。
5. Inspect 确认未生效时，只有工具声明幂等且最新策略允许，才创建新 attempt。
6. Inspect 仍无法确认时，Task 进入 `PAUSED`，向用户展示已知事实、未知范围和人工建议。

验证失败不等同于 Execute 失败。系统创建 Compensation 提案，按普通动作重新执行 Prepare、Risk、Approval、Execute、Inspect 和 Verify。补偿成功的 Task 结果为 `FAILED_RECOVERED` 语义，不能显示原目标成功。

## 7. API

### 7.1 约定

- 产品路径统一以 `/assistant/v1` 开头。
- ID 是不可猜测的稳定标识；`name` 只表示展示名称。
- 写请求使用 `Idempotency-Key`；更新和决定请求携带 `version` 或 `If-Match`。
- 长任务创建返回 `202 Accepted` 和 Task 引用。
- 列表使用不透明 Cursor，不暴露数据库 offset。
- 错误使用稳定 `reason`、安全 `message` 和可选安全 metadata；内部 cause 只在责任边界记录一次。
- 未授权环境和对象返回一致的不可发现错误，不区分“不存在”和“无权限”。
- Console 代理携带短期、签名且有 audience 的身份断言；Assistant 不信任浏览器可伪造的普通转发 Header。

### 7.2 端点

| 方法与路径 | 用途 |
| --- | --- |
| `GET /healthz` | 进程存活 |
| `GET /readyz` | 必需依赖整体就绪 |
| `GET /assistant/v1/system/readiness` | 安全的逐组件就绪状态 |
| `POST /assistant/v1/environments` | 登记并验证 Ingate 环境 |
| `GET /assistant/v1/environments` | 列出当前主体可见环境 |
| `GET/PATCH/DELETE /assistant/v1/environments/{id}` | 查询、更新或停用环境 |
| `PUT/DELETE /assistant/v1/environments/{id}/grants/{principalId}` | 授予或撤销环境角色 |
| `POST /assistant/v1/sessions` | 创建绑定环境的会话 |
| `GET /assistant/v1/sessions` | 列出可见会话 |
| `GET /assistant/v1/sessions/{id}` | 查询会话与当前任务摘要 |
| `GET /assistant/v1/sessions/{id}/messages` | 分页读取持久消息 |
| `POST /assistant/v1/sessions/{id}/messages` | 提交目标并返回 Task |
| `GET /assistant/v1/tasks/{id}` | 查询 Task、Plan 和结果摘要 |
| `GET /assistant/v1/tasks/{id}/events` | SSE 补发与实时事件流 |
| `POST /assistant/v1/tasks/{id}:provideInput` | 提交等待中的补充信息 |
| `POST /assistant/v1/tasks/{id}:pause` | 请求在安全边界暂停 |
| `POST /assistant/v1/tasks/{id}:resume` | 从原阶段恢复 |
| `POST /assistant/v1/tasks/{id}:cancel` | 取消未开始工作 |
| `GET /assistant/v1/tasks/{id}/plans` | 读取所有计划版本 |
| `GET /assistant/v1/plans/{id}` | 读取计划 DAG、风险和状态 |
| `POST /assistant/v1/plans/{id}:revise` | 基于当前版本请求修改 |
| `POST /assistant/v1/plans/{id}:execute` | 请求执行当前有效版本 |
| `GET /assistant/v1/approvals` | 按授权范围列出待审批项 |
| `GET /assistant/v1/approvals/{id}` | 查看差异、证据和 Binding |
| `POST /assistant/v1/approvals/{id}:decide` | 批准、拒绝或要求修改 |
| `GET /assistant/v1/evidence/{id}` | 查看经过授权和脱敏的证据 |
| `POST /assistant/v1/knowledge/sources` | 创建知识源 |
| `GET /assistant/v1/knowledge/sources` | 列出有权管理的知识源 |
| `POST /assistant/v1/knowledge/sources/{id}:sync` | 启动同步 |
| `DELETE /assistant/v1/knowledge/documents/{id}` | 删除文档并清理索引 |
| `POST /assistant/v1/knowledge:previewSearch` | 权限过滤的检索预览 |
| `GET /assistant/v1/tools` | 查看可见工具定义和状态 |
| `PATCH /assistant/v1/tools/{name}/versions/{version}` | 启用或禁用工具版本 |
| `GET/POST /assistant/v1/policies` | 查询或创建治理策略版本 |
| `GET /assistant/v1/memories` | 查询当前主体可见记忆 |
| `POST /assistant/v1/memories` | 确认保存候选记忆 |
| `PATCH/DELETE /assistant/v1/memories/{id}` | 创建记忆新版本或删除记忆 |
| `GET /assistant/v1/audit/events` | 权限过滤的审计查询 |
| `POST /assistant/v1/audit/exports` | 创建异步导出 |
| `GET /assistant/v1/audit/exports/{id}` | 查询状态和短期下载授权 |

实际协议使用 Proto 定义并通过现有 Buf/Make 流程生成，禁止手改生成代码。请求格式由 service 校验；权限、版本、引用、计划和治理规则由 biz/usecase 校验。

### 7.3 幂等与并发

`Idempotency-Key` 的作用域是 `(principal_id, route, aggregate_id)`，服务端保存请求 Digest。相同键和相同 Digest 返回原结果；相同键和不同 Digest 返回 `IDEMPOTENCY_CONFLICT`。

资源更新、计划修改、策略更新、工具启停和审批决定使用版本条件。并发冲突返回稳定 `VERSION_CONFLICT`，不进行最后写入获胜。动作执行不向浏览器提供直接 Execute 端点；只能由 Workflow 在满足治理条件后调用 Tool。

### 7.4 SSE 事件契约

持久事件具有 `id`，值为 Task 内单调递增序号：

```text
id: 42
event: task.status.changed
data: {"taskId":"...","sequence":42,"status":"WAITING_APPROVAL","occurredAt":"..."}
```

临时 Token 增量没有 `id`，不承诺断线补发：

```text
event: assistant.token.delta
data: {"taskId":"...","streamId":"...","text":"..."}
```

客户端携带 `Last-Event-ID` 重连时，服务端先从 MySQL 返回所有更大序号的持久事件，再进入实时等待。最终回答通过持久 `message.completed` 事件提供。服务端每 15 秒发送 SSE comment 心跳；慢客户端只允许丢弃临时 Token，不得丢弃持久状态。任何事件 payload 都必须通过前端安全字段和脱敏策略。

## 8. 错误与重试

### 8.1 错误分类

| Reason | 语义 | 默认处理 |
| --- | --- | --- |
| `INVALID_ARGUMENT` | 请求格式或 Schema 不合法 | 用户修改输入 |
| `NOT_FOUND` | 已授权范围内对象不存在 | 用户刷新或修改目标 |
| `SCOPE_NOT_FOUND` | 不可发现或无权限范围 | 不泄露存在性 |
| `VERSION_CONFLICT` | 版本或 Binding 已漂移 | 重新读取并生成新版本 |
| `IDEMPOTENCY_CONFLICT` | 同一键对应不同请求 | 使用新键或原请求 |
| `TOOL_UNAVAILABLE` | 工具禁用、未知或依赖不可用 | 停止该分支 |
| `MODEL_OUTPUT_INVALID` | 两次输出均不符合 Schema | 保留证据并安全失败 |
| `RISK_UNCLASSIFIED` | 风险属性无法确定 | 按 R3 或拒绝执行 |
| `APPROVAL_REQUIRED` | 尚未满足审批席位 | 等待审批 |
| `APPROVAL_STALE` | 绑定、权限或有效期变化 | 重新审批 |
| `PRECONDITION_FAILED` | 环境或资源已漂移 | 重新 Prepare/规划 |
| `ACTION_RESULT_UNKNOWN` | 可能已有副作用 | Inspect，禁止盲重试 |
| `VERIFICATION_FAILED` | 请求成功但业务断言失败 | 生成补偿提案 |
| `DEPENDENCY_UNAVAILABLE` | MySQL、Temporal、模型或目标 API 暂时不可用 | 按边界有界重试 |

所有未知内部错误返回统一安全消息并在 HTTP/Worker 责任边界记录 cause。日志不重复记录同一错误，不返回数据库、SDK、网络响应正文或凭证。

### 8.2 重试矩阵

| 操作 | 自动重试 |
| --- | --- |
| MySQL 死锁/瞬时连接失败 | 在事务边界有界重试，幂等约束保持生效 |
| Temporal Outbox 投递 | 至少一次重试，Workflow ID/command ID 去重 |
| 模型调用 | 仅瞬时失败有界重试；Schema 最多生成两次 |
| 查询工具 | 在超时预算内有界重试，仍失败返回未知而非未找到 |
| Prepare/Dry Run/Inspect | 工具声明只读且幂等时有界重试 |
| Execute 写入 | 只有能证明未发送或目标支持同一幂等键时重试；否则 `UNKNOWN` |
| Approval Signal | 可重复投递，approval ID 和 Binding 去重 |
| Audit intent | 有界重试；最终失败时变更 fail closed |

指数退避和最大次数由 Activity 类型配置，不把同一套策略用于模型、数据库、查询和写入。

## 9. 安全

### 9.1 认证、授权与最小权限

Assistant 复用 Console 登录身份，但独立验证短期签名身份断言的签名、发行者、audience、过期时间和主体。直接访问 Assistant API 也必须使用同等可信认证，不能信任客户端自报角色。

环境授权至少包含 `User`、`PlatformEngineer`、`Approver`、`SecurityAdmin`、`InstallationAdmin`、`KnowledgeAdmin` 和 `Auditor`。授权检查发生在：列表过滤、对象读取、知识检索、工具解析、计划校验、批准、执行前置条件、Inspect、Verify、记忆使用和审计导出。

未授权环境请求在访问任何下游前失败，并使用与不存在一致的外部响应。撤权对后续工具调用立即生效；Workflow 历史中的旧授权快照不能继续授予权限。

### 9.2 提示注入与信任标签

输入分为四个信任层：系统规则、服务端工具定义、用户数据、外部数据。知识和工具输出始终是外部数据，即使其中包含“系统”“管理员”或“忽略审批”等文字，也只能作为引用内容。

结构化模型输出必须经过 Schema、工具注册表、权限、风险和计划校验。模型不能：

- 修改 System Prompt；
- 生成可直接执行的 URL 或命令；
- 注册或启用工具；
- 设置最终风险等级；
- 构造 `ApprovedAction`；
- 读取凭证引用对应的值；
- 将外部内容提升为可信系统事实。

### 9.3 数据保护、脱敏与删除

数据按公开产品字段、内部业务字段、敏感内容和秘密四级处理。秘密永不进入模型、Workflow 历史、消息、TaskEvent、Evidence 正文、普通日志、错误、SSE 或导出。

统一清洗器至少识别 Authorization、Cookie、API Key、Token、私钥、连接 DSN 和配置标记的敏感 JSON 路径。内容在进入日志、事件、模型和导出前分别执行清洗，不能依赖前端隐藏。

默认保留期：会话、消息、任务和证据 180 天；审计事件 365 天；导出文件 24 小时；临时 Token 不持久化；记忆保存到用户删除。部署可缩短保留期，延长需要明确安全评审。删除记忆和知识必须同步使索引不可用。

### 9.4 凭证与网络边界

环境、模型和知识连接只保存 `credential_ref`。凭证解析器只向具体 adapter 提供调用所需的短期值，并确保值不会进入返回对象或错误。凭证轮换不修改 Plan，但执行前会重新检查引用仍有效；审批绑定凭证引用的修订，不绑定秘密值。

出站访问根据管理员登记并验证的目标建立允许列表，至少绑定 scheme、host、port、用途和环境。禁止任意重定向逃离允许目标，禁止非 HTTP/gRPC 连接和 Shell。目标 Ingate 必须使用 TLS 并验证证书；不能用跳过证书校验换取便利。

## 10. 紧凑部署、性能与容量

### 10.1 首版部署

首版正式形态是单 Assistant 进程加外部 MySQL、Temporal 和模型端点；Qdrant 可选。API、Worker、Outbox Dispatcher、Reconciler 和知识同步器在同一进程中由 Kratos 生命周期管理。

单进程不等于进程内任务：所有业务事实进 MySQL，所有持久编排进 Temporal。进程重启不丢失审批、Timer、DAG frontier 或已完成动作。Assistant 整体不可用不会改变 Envoy 转发或 Ingate 配置发布。

### 10.2 硬限制与默认预算

| 项目 | 限制 |
| --- | --- |
| 用户目标长度 | 1–20,000 字符 |
| 单计划步骤 | 最多 100 |
| 单任务并行 DAG 步骤 | 最多 20 |
| 单任务并行 Agent | 最多 3 |
| 单 Agent 工具轮次 | 最多 8 |
| R3 批准有效期 | 最长 15 分钟 |
| SSE 心跳 | 15 秒 |
| 审计导出有效期 | 24 小时 |

全局 Workflow、Activity、模型和知识同步并发通过配置限制，Worker 必须应用 Temporal slot/concurrency 限制和模型端点配额。达到限制时排队或返回明确容量错误，不能无界创建 goroutine。

### 10.3 性能目标

- 不含外部依赖耗时的命令接收 p95 小于 500 ms。
- 已持久事件的 SSE 重连补发在 1,000 条以内 p95 小于 2 s。
- 暂停或取消命令在当前安全动作完成后 5 s 内阻止新动作调度。
- MySQL/Temporal 短暂恢复后，Outbox 与活跃 Task 在 60 s 内开始收敛。
- 任何预算或容量超限都必须显式降级或排队，不允许牺牲权限、审批、审计和幂等。

这些是首版验收目标，不代表尚未测量的生产 SLO。容量基线由第 22 阶段的可执行旅程采集；多实例、API/Worker 分角色和 Kubernetes 交付延期到出现真实负载需求后。

## 11. 验证策略

仓库当前不新增 `*_test.go` 或前端测试文件。Assistant 使用编译、静态检查、真实依赖联调、curl 和可执行端到端验证器验收。

### 11.1 每阶段验证

- 使用现有 Make/Buf 流程生成 Proto 和 Wire，禁止手改生成代码。
- 运行受影响 Go 包的 build/vet 和 Console 生产构建。
- 配置或 Compose 变化时检查 Compose 展开结果和组件状态。
- API 使用 curl 验证状态码、错误脱敏、幂等、版本冲突和权限过滤。
- 完整变更最终运行 `make verify`；若用户明确延期，提交说明必须列出未执行项。

### 11.2 必须覆盖的联调场景

1. 单进程启动、逐依赖就绪和优雅退出。
2. 未授权环境不可发现且不产生下游请求。
3. 会话持久化、SSE 断线补发和消息幂等。
4. Qdrant 停止后的权限过滤关键词降级。
5. 知识与实时配置冲突的证据化诊断。
6. 计划修改、循环拒绝和旧版本失效。
7. 提示注入、未知工具、任意 URL 和秘密泄漏拒绝。
8. R1 自动执行一次，重放不产生第二个 Service。
9. R2 单人审批、资源漂移失效和执行前撤权。
10. R3 双人不同角色审批和 15 分钟过期。
11. DAG 并行、依赖阻塞、失败跳过和任务控制。
12. Worker 在审批等待与执行中重启后恢复。
13. Execute 已生效但响应超时后进入 `UNKNOWN` 并 Inspect。
14. 验证失败、补偿批准、恢复确认和补偿失败证据。
15. 记忆确认、作用域、usage、编辑和删除。
16. 审计过滤、摘要校验、短期导出和写入失败 fail closed。

### 11.3 完整 E2E

最终验证器在隔离 Ingate 环境创建平台工程师、审批人、安全管理员、知识、资源和可观察故障，通过真实 Console API、Temporal、MySQL、本地模型和 Admin API 完成：

```text
目标 -> 知识/实时证据 -> 诊断 -> 计划 DAG -> R0/R1
     -> R2 等待/批准 -> Execute -> Inspect -> Verify -> 审计
```

同一验证器继续运行审批过期、重复幂等键、Worker 重启和写入后超时场景，并搜索模型输入捕获、日志、SSE、审计和导出，证明测试秘密没有明文泄漏。验证结束清理隔离数据并输出稳定报告。

## 12. 实施计划

每个阶段交付一条可验收纵向能力，不创建占位接口和空目录。依赖关系按 Issue 的 `Blocked by` 执行。

| # | 纵向能力 | 主要追踪 | 状态 |
| --- | --- | --- | --- |
| 01 | 单进程 Assistant 与真实就绪状态 | Section 1–3、10 | 后端骨架已提交；Console 页面延期 |
| 02 | Ingate 环境登记与范围授权 | US-002；FR-27、40、57–64 | 计划中 |
| 03 | 持久流式会话 | US-001；FR-1–8 | 计划中 |
| 04 | 权限过滤的组织知识 | US-003；FR-9、12–14 | 计划中 |
| 05 | 实时 Ingate 证据 | US-004、005；FR-10–16 | 计划中 |
| 06 | 证据化诊断 | US-008；FR-9–16、25–32 | 计划中 |
| 07 | 审阅和修改计划 | US-006；FR-17–19 | 计划中 |
| 08 | 工具与风险策略治理 | US-009；FR-25–28 | 计划中 |
| 09 | 确定性 R0–R3 风险预览 | US-010；FR-33–35 | 计划中 |
| 10 | R1 Service 创建恰好一次 | US-011；FR-29–36 | 计划中 |
| 11 | R2 单人审批 | US-012；FR-37–42 | 计划中 |
| 12 | R3 双人不同角色审批 | US-012；FR-37–42 | 计划中 |
| 13 | 多步骤 DAG 执行 | US-013；FR-29–32 | 计划中 |
| 14 | 独立验证与受控补偿 | US-014；FR-43–48 | 计划中 |
| 15 | 暂停、恢复和取消 | US-015；FR-2、5–8、29–32 | 计划中 |
| 16 | 跨重启恢复与 Reconciler | US-015；Section 4、5.4 | 计划中 |
| 17 | `UNKNOWN` 结果核对 | US-018；FR-43–48 | 计划中 |
| 18 | 受预算约束的多 Agent | US-007；FR-20–24 | 计划中 |
| 19 | 长期记忆治理 | US-016；FR-49–52 | 计划中 |
| 20 | 审计查询与导出 | US-017；FR-53–56 | 计划中 |
| 21 | 注入与秘密边界验证 | US-002、018；FR-57–64 | 计划中 |
| 22 | 完整 Assistant 旅程 | US-019；FR-1–64 | 已确认延期 |

Kubernetes 分角色部署和独立容量交付不属于当前 22 个阶段，继续延期。只有单进程容量或运维证据证明需要拆分时，才新增相应设计和 Issue。

## 13. 假设与已定边界

1. 当前没有生产数据兼容要求，可以直接重写错误设计并删除废弃代码。
2. 首版数据库是 MySQL 8.0；Temporal 使用独立数据库 Schema，但 Assistant 不直接读取 Temporal 数据库。
3. 首版接入一个管理员配置的 OpenAI 兼容聊天模型端点，包括本地兼容服务；模型 adapter 保留在 `data/model` 与 `agent` 边界。
4. Console 能向 Assistant 提供可验证的短期身份断言；Assistant 不建立用户密码或第二套登录。
5. 一个 Assistant 可以登记多套 Ingate，但每套目标 Ingate 自身仍只有一个环境和配置域。
6. Assistant 只通过目标 Admin API 使用产品协议；声明式存储和数据面实现细节不进入产品接口。
7. 长期记忆默认只保存用户明确确认的已验证事实；不存在隐式“自动记住”。
8. Qdrant、对象存储和多实例消息分发都是可替换基础设施能力，不能改变 MySQL 权威数据、权限和任务语义。
9. Eino 是实现细节，不是领域契约；如果未来替换框架，API、Workflow、Tool 和持久模型无需迁移。
10. 当前无阻塞开放问题。Console 完整页面、第 22 阶段 E2E、Kubernetes 角色拆分和容量专项按已确认顺序延期。
