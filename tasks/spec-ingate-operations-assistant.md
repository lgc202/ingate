# SPEC：Ingate 只读运维助手

> Technical specification derived from: `tasks/prd-ingate-operations-assistant.md`
> Generated: 2026-09-01 | Target branch: `codex/assistant-skeleton` | Commit: `63899720`

## 1. 概要

### 1.1 本 SPEC 的范围

本 SPEC 定义 Ingate 只读运维助手的后端架构、模型连接、会话持久化、Agent 运行、只读工具、流式协议、安全边界和验证方式。首版允许管理员通过 Assistant API 配置直连 OpenAI-compatible 模型，或通过 Ingate Gateway 和 AI Route 调用模型；用户随后可以在当前 Ingate 内创建共享会话，诊断 Gateway、Route 和 Service 的配置与生效状态。

Assistant 是可选控制面组件，不参与 `Resource -> Compiler -> Delivery -> xDS -> Envoy` 配置链路，不创建、修改、删除或发布资源。

### 1.2 PRD 对应关系

- 来源：`tasks/prd-ingate-operations-assistant.md`
- 覆盖用户故事：US-001 至 US-010
- 覆盖功能需求：FR-1 至 FR-42
- 延后范围：Console 页面可以在后端 API 完成后单独接入，但仍属于本 SPEC 的最终交付范围

### 1.3 设计决策

| 决策 | 选择 | 原因 |
| --- | --- | --- |
| 服务形态 | 独立 `ingate-assistant` Kratos HTTP 服务 | 与配置发布和业务流量隔离，复用仓库现有装配方式 |
| Agent Runtime | Eino v0.9.18 稳定版 | 已验证工具调用和流式输出；只使用模型、Runner 和 Tool 能力 |
| Eino 边界 | 仅允许出现在 `internal/assistant/data/eino` | 公共协议、数据库和业务层不绑定框架 |
| 模型连接 | 通过 Assistant API 持久化 | 模型选择是产品状态，不是进程启动参数 |
| 连接模式 | `direct` 和 `ingate` | 同时满足直接连接模型与复用 Ingate AI Route 的需求 |
| 模型协议 | OpenAI-compatible Chat Completions | 直连和 Ingate AI Route 可以共用一个模型适配器 |
| 会话存储 | MySQL `sessions`、`items` | 保存最小产品事实，不保存框架执行历史 |
| 模型连接存储 | MySQL 单例 `model_connection` | 连接独立于会话存在，字段与两种模式一一对应 |
| Run 存储 | 不创建 `runs` 表 | 首版 Run 不脱离 HTTP 请求，不需要后台查询或恢复 |
| 实时协议 | POST SSE | 同时支持结构化请求体和单向流式响应 |
| 文档检索 | 编译时只嵌入 Markdown 文档 | 与发布版本一致，无外部知识库和运行时挂载 |
| 工作流 | 删除 Temporal | 首版没有后台运行、暂停恢复、审批或补偿 |
| 身份 | Assistant 不识别用户身份 | 当前 Ingate 内共享配置和会话，外部入口由 Console 保护 |
| 数据关系 | 应用层保障，不创建外键 | 符合仓库约定，也便于直接重写错误设计 |

Eino v0.9.18 的最小原型已经在 Go 1.27 下验证了类型化工具调用、工具结果和流式最终回答。其 `sonic` 依赖在 Go 1.27 下会回退到标准库 JSON，因此必须保持框架隔离，便于升级或替换。

框架选择依据：

| 候选 | 结论 |
| --- | --- |
| [Eino v0.9](https://github.com/cloudwego/eino/releases) | 采用稳定版 Runner、ChatModel 和 Tool；产品会话与持久化仍由 Ingate 管理 |
| [Eino v0.10](https://github.com/cloudwego/eino/discussions/1159) | Session 和后台运行方向合适，但当前仍是预发布能力，且会与首版 MySQL 产品历史重叠 |
| [Google ADK Go](https://github.com/google/adk-go/blob/main/runner/runner.go) | Runner、Session、Event 模型成熟，但模型类型与 Google GenAI 结合更深，不适合作为当前 OpenAI-compatible 首选 |
| [OpenAI Go SDK](https://github.com/openai/openai-go) | 模型客户端可靠，但直接采用意味着自行实现完整工具循环和流式事件转换 |
| LangChainGo | 当前需求没有必须依赖其 Chain、Memory 或生态组件的能力，不额外引入 |

产品概念参考成熟实现普遍采用的追加式层级：[OpenAI Codex](https://github.com/openai/codex/blob/main/codex-rs/app-server/README.md) 使用 Thread、Turn 和 Item，[Google ADK](https://github.com/google/adk-go/blob/main/session/service.go) 使用 Session、Invocation 和 Event。Ingate 对外保留 Session、Run 和 Item，但不复制任一框架的内部事件格式。

## 2. 架构

### 2.1 系统上下文

```text
Browser
   |
   | existing Console access boundary
   v
Console reverse proxy
   |
   | strips Cookie and Authorization
   v
Assistant HTTP/SSE
   |
   v
service.Service
   |
   v
biz.Usecase
   |-----------------------> MySQL
   |
   +--> biz.Agent ----------> Eino Runner
                                |
                                +--> read-only tools
                                |      +--> Admin API gRPC
                                |      +--> embedded product docs
                                |
                                +--> selected model connection
                                       +--> direct model endpoint
                                       +--> Ingate Gateway -> AI Route
```

Assistant 故障不得影响 Console 其他页面、Controller、xDS、Envoy 或任何业务流量。模型或 Admin API 暂时不可用时，历史会话仍然可以读取。

### 2.2 Kratos 分层

#### server

- 装配 Kratos HTTP Server；
- 注册生成的会话和模型连接 API；
- 注册自定义 POST SSE Handler；
- 提供 `/healthz` 和 `/readyz`；
- 处理 request ID、恢复和禁止缓存；
- 不包含 Agent、数据库或模型业务规则。

#### service

- 实现 Proto 服务；
- 校验请求格式、字段长度和互斥字段；
- 将 Proto 类型转换为 biz 类型；
- 将 biz 错误转换为稳定 API 错误；
- 不直接访问 MySQL、Admin API 或 Eino。

#### biz

- `Usecase` 编排会话、模型连接和一次 Run；
- 定义 `Agent` 与 `Store` 等消费者侧接口；
- 定义框架无关的 Session、Item、Source 和 AgentEvent；
- 保证 Item 追加顺序、Run 终态和模型连接快照语义；
- 不导入 Eino、Kratos Transport、SQL 或 Admin API 生成客户端。

#### data

- `mysql` 实现模型连接、Session 和 Item 的持久化；
- `adminapi` 只读访问 Gateway、Route 和 Service；
- `productdocs` 构建并查询当前版本文档索引；
- `eino` 实现 Agent、模型和四个只读工具；
- 只在真实外部边界定义或实现接口。

### 2.3 核心接口

```go
type Agent interface {
    Run(
        ctx context.Context,
        input AgentInput,
        emit func(AgentEvent) error,
    ) error
}

type Store interface {
    GetModelConnection(ctx context.Context) (ModelConnection, error)
    SaveModelConnection(ctx context.Context, connection ModelConnection) error
    CreateSession(ctx context.Context, session Session) error
    ListSessions(ctx context.Context, page SessionPage) ([]Session, string, error)
    GetSession(ctx context.Context, id string) (Session, error)
    ListItems(ctx context.Context, sessionID string, afterID uint64, limit int) ([]Item, error)
    AppendItem(ctx context.Context, item Item) (uint64, error)
}
```

`Agent.Run` 使用同步回调而不是后台 goroutine 或公共 channel。回调返回前，Usecase 可以完成持久化和 SSE 输出；回调失败会立即取消模型运行。

`Store` 是 biz 对 MySQL 这一真实边界的最小需求，不继续拆成只含一个方法的小接口。

### 2.4 文件结构

```text
api/assistant/v1/
└── conversation.proto

cmd/ingate-assistant/
└── main.go

internal/assistant/
├── app.go
├── wire.go
├── wire_gen.go
├── conf/
│   ├── assistant.proto
│   └── validate.go
├── server/
│   ├── server.go
│   ├── http.go
│   ├── sse.go
│   └── http_filter.go
├── service/
│   ├── service.go
│   ├── model_connection.go
│   └── conversation.go
├── biz/
│   ├── biz.go
│   ├── model_connection.go
│   ├── conversation.go
│   ├── agent.go
│   └── usecase.go
└── data/
    ├── data.go
    ├── mysql/
    │   ├── database.go
    │   ├── store.go
    │   ├── migrate.go
    │   └── migrations/
    │       └── 00001_initial.sql
    ├── adminapi/
    │   └── client.go
    ├── productdocs/
    │   └── search.go
    └── eino/
        ├── agent.go
        ├── model.go
        └── tools.go

docs/
└── embed.go

test/e2e/assistant/
├── model/
│   └── main.go
└── verify.sh
```

删除以下现有骨架：

- `internal/assistant/server/worker.go`；
- `internal/assistant/data/temporal`；
- Temporal 配置、Wire Provider 和 Compose 服务依赖；
- `--role` 命令参数；
- 模型健康 URL 配置；
- `api/assistant/v1/system.proto` 及对应生成代码；
- `biz/system` 和 `service/system`；
- 与旧设计冲突的 Assistant PRD、SPEC 和产品文档内容。

## 3. 数据模型

### 3.1 model_connection

当前 Ingate 只保存一个共享模型连接。无记录表示尚未配置。

```sql
CREATE TABLE model_connection (
    id TINYINT UNSIGNED NOT NULL
        COMMENT '单例记录 ID，固定为 1',
    mode VARCHAR(16) CHARACTER SET ascii COLLATE ascii_bin NOT NULL
        COMMENT '连接方式：direct 或 ingate',
    base_url VARCHAR(2048) NOT NULL
        COMMENT 'OpenAI-compatible API 基础地址，不包含凭据、查询参数和片段',
    gateway_id CHAR(36) CHARACTER SET ascii COLLATE ascii_bin NULL
        COMMENT 'ingate 模式使用的 Gateway ID，direct 模式为空',
    route_id CHAR(36) CHARACTER SET ascii COLLATE ascii_bin NULL
        COMMENT 'ingate 模式使用的 AI Route ID，direct 模式为空',
    model_name VARCHAR(256) NOT NULL
        COMMENT '直连模型名或 AI Route 发布的逻辑模型名',
    credential_ciphertext VARBINARY(4096) NULL
        COMMENT '直连 API Key 或 Ingate Caller Access Key 的版本化加密信封',
    PRIMARY KEY (id),
    CONSTRAINT chk_model_connection_singleton CHECK (id = 1),
    CONSTRAINT chk_model_connection_mode CHECK (
        (mode = 'direct' AND gateway_id IS NULL AND route_id IS NULL)
        OR
        (mode = 'ingate' AND gateway_id IS NOT NULL AND route_id IS NOT NULL)
    )
) ENGINE=InnoDB
  DEFAULT CHARSET=utf8mb4
  COLLATE=utf8mb4_0900_ai_ci
  COMMENT='运维助手当前使用的模型连接';
```

字段必要性：

- `id`：数据库层确保只存在一条连接；
- `mode`：决定字段语义和运行时解析方式；
- `base_url`：Gateway 资源不包含 Assistant 一定可访问的网络地址，两种模式都需要实际入口；
- `gateway_id`、`route_id`：Ingate 模式校验资源关系并为界面展示当前选择；
- `model_name`：直连端点和 AI Route 都可能提供多个模型；
- `credential_ciphertext`：保存直连 API Key 或受保护 Route 的 Caller Access Key。

不增加名称、状态、创建时间、更新时间或版本字段，因为首版只有一个共享设置，没有列表、审计或并发编辑需求。

### 3.2 sessions

```sql
CREATE TABLE sessions (
    id CHAR(36) CHARACTER SET ascii COLLATE ascii_bin NOT NULL
        COMMENT '会话 UUID，由应用生成',
    title VARCHAR(200) NOT NULL
        COMMENT '会话展示标题，首次提问后由问题文本生成',
    created_at DATETIME(6) NOT NULL
        COMMENT '会话创建时间，使用 UTC',
    updated_at DATETIME(6) NOT NULL
        COMMENT '最近一次追加内容项的时间，使用 UTC',
    PRIMARY KEY (id),
    KEY idx_sessions_updated_at (updated_at DESC, id)
) ENGINE=InnoDB
  DEFAULT CHARSET=utf8mb4
  COLLATE=utf8mb4_0900_ai_ci
  COMMENT='运维助手会话';
```

`title` 是会话列表所需的稳定展示信息。新会话标题为“新会话”，首次提问后使用问题前 60 个 Unicode 字符生成；连续空白折叠为一个空格。

### 3.3 items

```sql
CREATE TABLE items (
    id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT
        COMMENT '全局递增内容项 ID，同时确定会话内展示顺序',
    session_id CHAR(36) CHARACTER SET ascii COLLATE ascii_bin NOT NULL
        COMMENT '所属会话 UUID，由应用层保证引用有效',
    run_id CHAR(36) CHARACTER SET ascii COLLATE ascii_bin NOT NULL
        COMMENT '产生该内容项的一次运行 UUID',
    kind VARCHAR(32) CHARACTER SET ascii COLLATE ascii_bin NOT NULL
        COMMENT '内容类型：user_message、tool_result、assistant_message 或 run_error',
    payload JSON NOT NULL
        COMMENT '与 kind 对应的版本化内容，不保存 token delta 和框架事件',
    created_at DATETIME(6) NOT NULL
        COMMENT '内容项完成时间，使用 UTC',
    PRIMARY KEY (id),
    KEY idx_items_session_id_id (session_id, id)
) ENGINE=InnoDB
  DEFAULT CHARSET=utf8mb4
  COLLATE=utf8mb4_0900_ai_ci
  COMMENT='会话中已经完成并可恢复的语义内容项';
```

`id` 使用自增整数是为了无歧义地确定展示顺序和分页位置。`created_at` 只用于展示，不承担排序正确性。

### 3.4 Item Payload

| kind | Payload | 说明 |
| --- | --- | --- |
| `user_message` | `text` | 用户提交的完整问题 |
| `tool_result` | `tool`、`succeeded`、`summary`、`sources` | 已完成且经过裁剪的工具活动 |
| `assistant_message` | `markdown`、`citation_source_ids` | 完整回答和真实来源引用 |
| `run_error` | `code`、`message`、`retryable` | 可安全展示的运行终态 |

数据库使用 JSON 是因为不同 Item 具有不同结构；biz 和 Proto 必须使用明确结构体和 `oneof`，不得以 `map[string]any` 作为公共契约。

### 3.5 关系与一致性

- 不创建外键；
- 追加 Item 前必须确认 Session 存在；
- 插入 Item 与更新 `sessions.updated_at` 在同一事务完成；
- `kind` 与 Payload 的对应关系由应用层校验；
- 模型连接更新使用单行 upsert；
- Ingate 连接保存前通过 Admin API 验证 Gateway、Route 和模型关系；
- 删除或变更被引用的 Route 不自动删除模型连接，下一次 Run 返回明确错误；
- 首版不提供 Session 删除，因此不会主动产生孤立 Item。

### 3.6 Run 状态

首版不创建 `runs` 表，状态由同一 `run_id` 下的 Item 推导：

- 存在 `assistant_message`：completed；
- 存在 `run_error`：failed；
- 只有用户消息或工具结果，没有终态：interrupted。

如果以后需要后台运行、运行列表、恢复或多实例调度，再增加 `runs` 表，并使用已有 `run_id` 关联历史 Item。

### 3.7 迁移

- 使用仓库已有 Goose v3；
- `ingate-assistant --migrate` 执行迁移后退出；
- 正常服务不隐式执行 DDL；
- 启动时检查 schema 版本；
- Compose 增加一次性 `assistant-migrate`，成功后启动 Assistant；
- 回滚迁移只删除三张尚无生产兼容要求的新表；
- 不导入旧 Assistant 表和数据。

## 4. API 设计

### 4.1 端点

| Method | Path | 说明 | Assistant 内部认证 |
| --- | --- | --- | --- |
| GET | `/assistant/v1/model-connection` | 获取当前模型连接或未配置状态 | 无 |
| PUT | `/assistant/v1/model-connection` | 创建或替换当前模型连接 | 无 |
| POST | `/assistant/v1/sessions` | 创建会话 | 无 |
| GET | `/assistant/v1/sessions` | 分页列出共享会话 | 无 |
| GET | `/assistant/v1/sessions/{id}` | 获取会话元数据 | 无 |
| GET | `/assistant/v1/sessions/{id}/items` | 顺序读取完成后的 Item | 无 |
| POST | `/assistant/v1/sessions/{id}:run` | 提交问题并接收 SSE | 无 |

Assistant 只部署在 Console 可访问的内部网络。Console 继续保护 `/assistant/v1`，但不得把 Cookie、Authorization 或用户身份转发给 Assistant。

普通 API 由一个 `ConversationService` 提供：

```proto
service ConversationService {
  rpc GetModelConnection(GetModelConnectionRequest)
      returns (GetModelConnectionResponse);
  rpc PutModelConnection(PutModelConnectionRequest)
      returns (ModelConnection);
  rpc CreateSession(CreateSessionRequest)
      returns (Session);
  rpc ListSessions(ListSessionsRequest)
      returns (ListSessionsResponse);
  rpc GetSession(GetSessionRequest)
      returns (Session);
  rpc ListItems(ListItemsRequest)
      returns (ListItemsResponse);
}
```

Run 的请求、Item 和 Event 也定义在同一 Proto 中，但 `Run` 不声明为生成的 RPC。`server/sse.go` 使用 ProtoJSON 解析 `RunSessionRequest`，再将 `RunEvent` 编码为 SSE data，从而避免把 SSE 生命周期强行映射为 Kratos 生成的普通 HTTP Handler。

核心协议类型：

```proto
message ModelConnection {
  oneof connection {
    DirectModelConnection direct = 1;
    IngateModelConnection ingate = 2;
  }
}

message Item {
  string id = 1;
  string session_id = 2;
  string run_id = 3;
  google.protobuf.Timestamp created_at = 4;
  oneof content {
    UserMessage user_message = 5;
    ToolResult tool_result = 6;
    AssistantMessage assistant_message = 7;
    RunError run_error = 8;
  }
}

message Source {
  string id = 1;
  oneof source {
    ResourceSource resource = 2;
    DocumentSource document = 3;
  }
}

message RunEvent {
  string run_id = 1;
  oneof event {
    RunStarted run_started = 2;
    ItemStarted item_started = 3;
    ItemCompleted item_completed = 4;
    AssistantDelta assistant_delta = 5;
    RunCompleted run_completed = 6;
    RunFailed run_failed = 7;
  }
}
```

### 4.2 模型连接协议

#### DirectModelConnection

```json
{
  "direct": {
    "baseURL": "https://api.example.com/v1",
    "model": "example-model",
    "apiKey": "write-only"
  }
}
```

#### IngateModelConnection

```json
{
  "ingate": {
    "baseURL": "http://controller:8080/v1",
    "gatewayID": "418c2c32-646a-4ef2-8b31-5a2f08c58fc3",
    "routeID": "a71f5f69-69e4-43ea-b678-27d0f2d784cc",
    "model": "operations-assistant",
    "accessKey": "write-only"
  }
}
```

请求使用 Proto `oneof` 保证两种模式互斥。`api_key` 和 `access_key` 是可选、只写字段，并分别提供 `clear_api_key` 和 `clear_access_key`。

读取响应只返回非敏感字段和 `credential_configured`：

```json
{
  "configured": true,
  "connection": {
    "ingate": {
      "baseURL": "http://controller:8080/v1",
      "gatewayID": "418c2c32-646a-4ef2-8b31-5a2f08c58fc3",
      "routeID": "a71f5f69-69e4-43ea-b678-27d0f2d784cc",
      "model": "operations-assistant",
      "credentialConfigured": true
    }
  }
}
```

更新语义：

- 同模式更新且凭据字段缺失：保留已有凭据；
- 同模式提供新凭据：替换已有凭据；
- 同模式设置 clear：删除已有凭据；
- 同时提供凭据和 clear：`400 invalid_argument`；
- 切换模式：旧模式凭据始终删除，只有本次请求明确提供的新凭据会保存；
- 更新完成前开始的 Run 使用旧快照，之后开始的 Run 使用新配置。

Ingate 模式保存前必须验证：

- Gateway 和 Route 均存在；
- Route 的 `gateway_ids` 包含所选 Gateway；
- Route 是 AI Route；
- Route 发布了所选逻辑模型；
- Route 和 Gateway 已启用；
-受保护 Route 在没有已有或新 Caller Access Key 时拒绝保存。

资源当前未生效不阻止保存，但响应返回其当前状态；这样管理员可以先配置并继续修复资源。

### 4.3 会话协议

#### 创建

`POST /assistant/v1/sessions` 无请求字段，返回：

```json
{
  "id": "session-uuid",
  "title": "新会话",
  "createdAt": "2026-09-01T08:00:00Z",
  "updatedAt": "2026-09-01T08:00:00Z"
}
```

#### 列表

- `page_size` 默认 20，最大 100；
- `page_token` 是基于 `updated_at` 和 `id` 的不透明游标；
- 按 `updated_at DESC, id` 排序；
- 不返回 Item。

#### Item 列表

- `after_item_id` 为空表示从头读取；
- `page_size` 默认 100，最大 200；
- 查询条件为 `session_id = ? AND id > ? ORDER BY id`；
- API 中 Item ID 使用字符串表达，避免 JavaScript 整数精度问题。

### 4.4 Run 请求

```json
{
  "question": "为什么 Route checkout-api 没有生效？"
}
```

校验：

- trim 后不能为空；
- 必须是有效 UTF-8；
- 最大 16 KiB；
- Session 必须存在；
- 必须已经配置模型连接；
- 同一 Session 只能有一个在途 Run。

### 4.5 SSE

响应头：

```http
Content-Type: text/event-stream
Cache-Control: no-cache, no-transform
Connection: keep-alive
X-Accel-Buffering: no
```

事件类型：

| event | 是否持久化 | Payload |
| --- | --- | --- |
| `run.started` | 用户消息已持久化 | `run_id`、`user_item` |
| `item.started` | 否 | 临时 `activity_id`、工具类型和安全说明 |
| `item.completed` | 是 | 已写入数据库的 `Item` |
| `assistant.delta` | 否 | 增量文本 |
| `run.completed` | 回答已持久化 | `run_id` |
| `run.failed` | 错误已持久化 | `run_id`、安全错误 |

顺序约束：

1. `run.started` 是第一个事件；
2. 同一工具的 `item.started` 先于 `item.completed`；
3. `item.completed` 中的 Item 已经写入数据库；
4. 最终 `assistant_message` 的 `item.completed` 先于 `run.completed`；
5. `run.failed` 前尽力写入并发送 `run_error`；
6. `run.completed` 和 `run.failed` 互斥。

首版不支持 `Last-Event-ID`、断线重放和断点续跑。

### 4.6 Breaking Changes

当前 Assistant 只有未被 Console 消费的健康骨架，因此允许：

- 删除 `system.proto`；
- 删除 `--role`；
- 删除 Temporal 和模型 health URL 配置；
- 替换 Assistant 配置文件结构；
- 删除旧 PRD、SPEC 和运维助手文档中的骨架说明。

生成代码必须通过现有 Make/Buf 流程产生，不手工编辑。

## 5. 业务逻辑

### 5.1 保存模型连接

1. service 校验 `oneof` 和字段格式；
2. 校验 URL 只使用 HTTP 或 HTTPS，禁止 userinfo、query 和 fragment；
3. direct 模式校验模型名，凭据可选；
4. ingate 模式调用 Admin API 校验 Gateway、AI Route 和逻辑模型；
5. 根据更新语义决定保留、替换或清除凭据；
6. 使用 AES-256-GCM 生成新的随机 nonce 并加密新凭据；
7. 单事务 upsert `model_connection`；
8. 返回不含明文凭据的模型连接。

模型连接加密主密钥来自专用环境变量，必须是 32 字节随机值的 Base64 表达。密文使用版本化信封，包含版本、nonce 和 ciphertext；当前不提供在线密钥轮换。更换主密钥前必须重新录入模型凭据。

### 5.2 创建和读取会话

- 创建 Session 时使用 UUID；
-标题初始化为“新会话”；
-列表按最近活动排序；
- Item 严格按递增 ID 展示；
-没有身份过滤；
-不把以前会话自动加入新会话上下文。

### 5.3 开始 Run

1. 校验请求；
2. 获取 Session；
3. 获取进程内 Session 运行锁；
4. 读取并解密模型连接，形成不可变 Run 快照；
5. 生成 `run_id`；
6. 在事务中追加 `user_message`，首次问题同时更新标题；
7. 发送 `run.started`；
8. 加载当前问题之前最近 20 条用户和助手消息，总文本不超过 64 KiB；
9. 调用 Agent；
10. 按 AgentEvent 持久化完成 Item 并发送 SSE；
11. 写入回答或安全失败终态；
12. 释放 Session 运行锁。

以前的 `tool_result` 不作为伪造的模型工具调用重放。后续问题需要当前资源事实时，Agent 必须重新调用工具。

### 5.4 模型连接解析

#### direct

- `base_url`、`model_name` 和解密后的 API Key 直接构造 OpenAI-compatible ChatModel；
- API Key 缺失时仍允许调用不要求认证的本地端点；
-禁止 HTTP Redirect，限制响应体大小；
-不把完整模型请求和响应写入日志。

#### ingate

- 每次 Run 开始时重新读取 Gateway 和 Route，防止使用已经删除或改变的关系；
-将 `model_name` 作为 AI Route 的客户端逻辑模型；
-使用 `base_url` 调用 Ingate 数据平面；
-可选 Caller Access Key 作为 Bearer 凭据；
-供应商 API Key 由 Model Service 和 AI ExtProc 注入，Assistant 不读取；
- Ingate 配置失效时返回 `model_route_unavailable`，不自动回退到直连。

两种模式最终都构造同一 Eino OpenAI-compatible ChatModel，不维护两套 Agent。

### 5.5 Agent 与工具循环

Agent 使用 Eino v0.9.18 的 ChatModel Agent 和 Runner，固定最大 12 次工具调用、120 秒总运行时间。

首版工具：

| 工具 | 输入 | 最大输出 | 说明 |
| --- | --- | --- | --- |
| `find_gateways` | `query`、`limit` | 10 | 按 UUID Get，否则 List |
| `find_routes` | `query`、`limit` | 10 | 返回 Gateway 和 Service 引用 |
| `find_services` | `query`、`limit` | 10 | 返回服务类型、端点摘要和状态 |
| `search_product_docs` | `query`、`limit` | 5 | 查询当前版本文档章节 |

所有工具都是只读的。Eino Tool 注册表中不得出现 Create、Update、Delete、Publish、Shell、HTTP Fetch 或互联网搜索能力。

### 5.6 工具结果与来源

资源来源包含：

- `source_id`；
-资源类型；
-资源 ID；
-展示名称；
-读取版本；
-状态和可直接展示的状态说明；
-读取时间。

文档来源包含：

- `source_id`；
-标题；
-仓库内路径；
-章节；
-安全摘录。

每次 Run 内按首次出现顺序生成 `R1`、`R2`、`D1` 等来源 ID。相同资源版本或相同文档章节复用来源 ID。

工具结果必须先经过字段白名单：

- Gateway：ID、名称、启用状态、Listener 的协议、端口、hostname、状态、版本；
- Route：ID、名称、启用状态、Gateway ID、匹配条件、Service ID、逻辑模型、超时、重试、状态、版本；
- Service：ID、名称、类型、端点的 scheme、host、port、模型名称、API Key 是否配置、状态、版本；
-删除 URL userinfo、query、fragment；
-不返回任何密钥、请求 Header、完整内部错误或原始响应。

### 5.7 回答契约

系统提示要求回答包含：

1. 结论；
2. 已观察到的事实；
3. 推断；
4. 建议；
5. 限制。

资源和产品规则事实必须引用本次 Run 已产生的来源 ID。最终回答包含未知来源 ID 时，不得保存为成功回答，运行以 `invalid_model_output` 失败。不得展示或保存思维链。

### 5.8 文档嵌入和检索

`docs/embed.go` 只嵌入当前目录层级的 Markdown：

```go
//go:embed src/content/docs/*.md src/content/docs/*/*.md
var FS embed.FS
```

不得嵌入图片、视频、Node 依赖或构建产物。当前 25 个文档原始内容约 67 KiB，二进制增量可忽略。

启动时：

1. 读取 frontmatter 标题；
2. 按标题和二级章节切分；
3. 保留仓库相对路径；
4. 对标题、章节和正文执行不区分大小写的词项匹配；
5. 标题权重大于章节，章节大于正文；
6. 返回得分最高的五个片段。

不引入向量数据库、Embedding 服务、互联网搜索或文档副本。

### 5.9 客户端断开和进程退出

- SSE 写入失败立即取消模型和工具；
-使用 `context.WithoutCancel` 派生最多 2 秒的上下文，尽力追加 `run_error(interrupted)`；
-如果进程来不及写入，历史读取根据缺少终态推导为 interrupted；
-不声称恢复已经丢失的 token；
-用户重新运行会创建新的 `run_id`，不会覆盖原记录。

## 6. 错误处理

### 6.1 错误分类

| 错误码 | HTTP/SSE | 条件 | 用户消息 |
| --- | --- | --- | --- |
| `invalid_argument` | 400 | 请求字段非法 | 请求内容不符合要求 |
| `model_not_configured` | 409 | 没有模型连接 | 请先配置 Assistant 模型连接 |
| `invalid_model_connection` | 400 | URL、模式或资源关系非法 | 模型连接配置无效 |
| `model_connection_unavailable` | 503/SSE | 凭据无法解密或端点不可用 | 模型连接暂时不可用 |
| `model_route_unavailable` | SSE | Ingate Gateway、Route 或逻辑模型已失效 | Assistant 模型线路不可用 |
| `session_not_found` | 404 | Session 不存在 | 会话不存在 |
| `run_in_progress` | 409 | 同一 Session 已有 Run | 当前会话仍在处理中 |
| `storage_unavailable` | 503/SSE | MySQL 不可用 | 暂时无法保存会话 |
| `resource_unavailable` | SSE | Admin API 不可用 | 暂时无法读取 Ingate 资源 |
| `model_unavailable` | SSE | 模型调用失败 | 模型服务暂时不可用 |
| `invalid_model_output` | SSE | 回答结构或引用非法 | 未能生成可核验回答 |
| `interrupted` | SSE/历史推导 | 客户端断开或进程退出 | 本次运行已中断 |
| `internal_error` | 500/SSE | 未分类内部错误 | 运行失败，请稍后重试 |

普通 API 在响应头发送前使用 HTTP 状态码。SSE 已经开始后，业务失败必须通过持久化 `run_error` 和 `run.failed` 表达，不再修改 HTTP 状态。

### 6.2 重试

- 不自动重新运行完整 Run；
- 不对模型调用进行隐藏重试，避免重复费用；
- 不对工具基础设施错误进行隐藏重试；
-数据库驱动可以使用自身连接恢复；
-用户重试产生新的 `run_id` 和新的历史 Item；
-未找到资源是成功的空结果，不属于重试错误。

### 6.3 降级

- MySQL 不可用：普通 API 失败，`/readyz` 返回 503；
-模型未配置或不可用：历史 API 正常，Run 失败；
- Admin API 不可用：历史 API 正常，需要资源证据的 Run 失败；
-内置文档索引初始化失败：Assistant 启动失败；
- Assistant 整体不可用：Console 其他功能、配置发布和流量转发不受影响。

## 7. 安全

### 7.1 身份边界

- Assistant 不实现登录、用户授权和会话归属；
-模型连接和所有会话在当前 Ingate 内共享；
- Console 可以继续使用现有单管理员访问控制；
- Console 代理不得把身份凭据转发给 Assistant；
- Compose 不向宿主机暴露 Assistant 端口；
-后续多用户身份属于独立 PRD。

### 7.2 模型凭据

- API Key 和 Caller Access Key 只允许写入，不允许读回；
-使用标准库 AES-256-GCM 加密后写入 MySQL；
-加密主密钥只通过专用环境变量注入，不写入 YAML、数据库或日志；
-每次加密使用新的随机 nonce；
-切换连接模式删除旧密文；
-解密后的凭据只存在于当前 Run 内存中；
-日志、SSE、错误和工具结果不得包含明文或密文。

### 7.3 外部输入

- Proto/Buf Validate 校验长度、UUID、枚举和 oneof；
- URL 只允许 HTTP/HTTPS，禁止 userinfo、query 和 fragment；
-模型和 Admin API 响应大小有上限；
-不跟随模型端点 Redirect；
-文档路径来自嵌入 FS，不接受用户路径；
-工具查询和结果数量有固定上限；
-所有展示内容按普通文本或 Markdown 处理，Console 不执行其中 HTML。

### 7.4 日志

每次错误只在 Assistant HTTP 责任边界记录一次，可以记录：

- request ID；
- session ID；
- run ID；
-连接模式；
-工具名称；
-稳定错误类别。

不得记录：

-完整用户问题；
-完整模型请求或响应；
-工具原始响应；
-任何凭据；
-内部提示词；
-思维链。

健康检查和正常轮询不写 INFO 日志。

## 8. 性能与容量

### 8.1 首版目标

- 单个 Ingate Assistant 实例；
-最多 10 个并发 SSE Run；
-同一 Session 最多 1 个在途 Run；
-单次问题最大 16 KiB；
-上下文最多 20 条用户和助手消息、64 KiB 文本；
-单次 Run 最多 12 次工具调用；
-单次 Run 最长 120 秒；
-资源工具结果最多 10 条；
-文档结果最多 5 条。

这些是防止失控的产品上限，不是多租户容量承诺。

### 8.2 数据库查询

- Session 列表使用 `idx_sessions_updated_at`；
- Item 使用 `idx_items_session_id_id` 顺序分页；
-不按未建立索引的 `run_id` 单独查询；
-模型连接始终按单例主键读取；
-每个 Item 使用独立短事务；
-不缓存会话历史；
-内置文档索引只在进程内保存一份。

### 8.3 保留期限

首版不自动删除会话或 Item，也不提供删除 API。分页和请求上限防止单次读取失控。只有出现明确的数据保留要求后，才设计清理策略和对应产品行为。

## 9. 验证策略

仓库当前不维护单元测试，因此不新增 `*_test.go` 或前端自动化测试。

### 9.1 编译和生成

- 使用现有 Make/Buf 流程生成 Proto 和 Wire；
-编译 `ingate-assistant`；
-执行迁移并验证 schema 版本；
-不手工编辑生成文件。

### 9.2 后端端到端验证

`test/e2e/assistant/model` 提供只用于验证的确定性 OpenAI-compatible 服务，按固定请求顺序返回工具调用和最终回答，不进入生产镜像。

一个脚本覆盖：

1. 迁移空数据库；
2. 配置 direct 模式并验证 GET 不返回 API Key；
3. 创建 Session；
4. 提交问题并验证 SSE 顺序；
5. 验证资源工具和文档工具 Item；
6. 验证最终回答引用真实来源；
7. 重启 Assistant 后读取完整历史；
8. 配置 ingate 模式并通过真实 Ingate AI Route 调用确定性模型；
9. 验证错误模型、Admin API 不可用和 MySQL 不可用；
10. 验证失败 Run 重新运行不会覆盖原记录；
11. 搜索日志和响应，确认不包含测试凭据。

### 9.3 Console 验证

Console 接入完成后通过浏览器人工验证：

-查看和切换两种模型连接；
-创建和打开会话；
-观察工具活动和流式回答；
-展开资源和文档依据；
-刷新页面恢复历史；
- Assistant 不可用时其他 Console 页面正常。

### 9.4 验收映射

| PRD | 验证 |
| --- | --- |
| US-001 / FR-3 至 FR-6 | Session 创建、列表、详情和 Item 分页 |
| US-002 / FR-7 至 FR-14 | Run 请求、SSE 和完成内容持久化 |
| US-003 / FR-15、FR-16 | 工具活动、完成摘要和不展示推理 |
| US-004 / FR-17 至 FR-20 | Gateway、Route、Service 只读查询和资源来源 |
| US-005 / FR-21 | 当前版本文档检索和文档来源 |
| US-006 / FR-22、FR-25 至 FR-28 | 回答结构、引用校验和只读边界 |
| US-007 / FR-23、FR-24 | 失败终态和新 Run 重试 |
| US-008 / FR-1、FR-2 | Console 集成和单 Ingate 边界 |
| US-009 / FR-29、FR-30 | 确定性 E2E、重启恢复和故障隔离 |
| US-010 / FR-31 至 FR-42 | 模型连接 API、两种模式和凭据安全 |

## 10. 实施计划

### 10.1 小步交付

| 候选 Issue | 内容 | 依赖 |
| --- | --- | --- |
| A-01 | 删除 Temporal、Worker、旧 readiness 和错误骨架配置，保留最小 Kratos HTTP/MySQL 进程 | 无 |
| A-02 | 增加三张表、显式迁移命令和 schema 检查 | A-01 |
| A-03 | 实现模型连接 API、加密和 direct/ingate 校验 | A-02 |
| A-04 | 实现 Session 与 Item 普通 API | A-02 |
| A-05 | 实现 Admin API 只读客户端和内置文档检索 | A-01 |
| A-06 | 接入 Eino 和 direct 模型工具循环 | A-03、A-04、A-05 |
| A-07 | 接入 ingate 模式并复用同一个 Agent | A-03、A-06 |
| A-08 | 实现 Run Usecase、SSE、来源和失败语义 | A-04、A-06、A-07 |
| A-09 | 增加确定性模型和后端端到端验证，更新部署与产品文档 | A-08 |
| A-10 | 接入现有 Console 页面并完成浏览器验证 | A-03、A-04、A-08 |

实际 GitHub Issue 编号由后续 `to-issues` 流程创建。每个 Issue 单独 review，确认后再提交；不得提前创建后续 Issue 才需要的表、接口、包或空目录。

### 10.2 首轮主线

用户已明确 Console 可以延后，因此首轮实现到 A-09 即形成可通过 `curl` 验收的完整后端闭环。A-10 不阻塞后端主线。

### 10.3 Temporal 演进点

首次出现以下任一已确认需求时，重新评估 Temporal：

- Run 必须脱离 HTTP 请求继续执行；
-浏览器重新连接后需要继续订阅；
-需要暂停等待人工审批；
-需要定时巡检；
-需要可靠写操作、补偿或跨进程调度。

届时增加真实 `runs` 表、Workflow 和 Activity，并将 Worker 作为独立进程评估。当前不保留空 Worker、占位接口或 Temporal 配置。

## 11. 风险与假设

### 11.1 无阻塞问题

本 SPEC 没有实现前必须再次确认的技术问题。未来身份、后台执行、自动修复、多 Agent 和数据清理由各自的新需求决定。

### 11.2 风险

| 风险 | 影响 | 缓解 |
| --- | --- | --- |
| Eino 对 Go 1.27 使用 JSON 回退 | 日志提示和轻微性能损失 | 限制在单包，固定版本，编译和 E2E 验证 |
| OpenAI-compatible 端点实现差异 | 工具调用或流式格式不兼容 | 首版声明 Chat Completions + Tool Calling 契约，保存前做结构校验，运行时安全失败 |
| Ingate 模式依赖正在诊断的数据平面 | Gateway 故障时 Assistant 无法调用模型 | 允许切换 direct 模式；不实现自动 fallback，避免隐藏故障 |
| 加密主密钥丢失或变化 | 已保存凭据无法解密 | 启动校验密钥格式，明确重新录入流程，不返回密文 |
| 无 Run 表 | 进程崩溃时只能推导 interrupted | 符合首版不恢复要求；后台运行出现后再增加 Run 表 |
| 单实例 Session 锁 | 多副本可能同时运行 | 首版明确单实例；多副本需求出现后使用 Run 表和持久租约 |
| 文档简单词项检索召回不足 | 部分规则无法找到 | 回答明确证据不足；有真实失败样本后再优化检索 |

### 11.3 假设

- 当前部署只有一个 Assistant 实例；
- Console 是 Assistant 唯一外部入口；
- direct 模式端点支持 OpenAI-compatible Chat Completions、流式输出和 Tool Calling；
- Ingate AI Route 保持当前 OpenAI Chat Completions 产品入口；
-管理员可以为 Ingate 模式提供 Assistant 可访问的 Gateway Base URL；
-当前没有需要迁移的生产 Assistant 数据；
-当前不要求保存用户身份、配置修改人或审计历史；
-模型连接只有一份，不需要列表、名称和版本控制。
