-- +goose Up
-- 运维助手只使用一份当前生效的模型连接。singleton_id 固定为 1，避免为了单例配置
-- 引入无意义的名称、启用状态和默认项；后续需要多连接或故障切换时再建立独立资源模型。
CREATE TABLE assistant_model_connections (
    singleton_id TINYINT UNSIGNED NOT NULL COMMENT '单例键，固定为 1',
    connection_mode TINYINT UNSIGNED NOT NULL COMMENT '连接方式：1 表示直连模型厂商，2 表示通过 Ingate AI 路由',
    protocol TINYINT UNSIGNED NOT NULL COMMENT '模型协议：1 表示 OpenAI 兼容协议，2 表示 Anthropic 原生协议',
    endpoint VARCHAR(2048) NOT NULL COMMENT '模型 API 根地址；不同协议使用各自的官方地址格式',
    api_key VARCHAR(4096) NOT NULL COMMENT '访问模型厂商或 Ingate 调用方的凭据，接口永不返回原文',
    model VARCHAR(160) NOT NULL COMMENT '直连时是厂商模型名，通过 Ingate 时是 AI 路由发布的客户端模型名',
    timeout_ms INT UNSIGNED NOT NULL COMMENT '单次模型调用超时时间，单位毫秒',
    max_output_tokens INT UNSIGNED NOT NULL COMMENT '单次回复允许的最大输出 Token 数',
    reasoning_budget_tokens INT UNSIGNED NOT NULL COMMENT 'Anthropic 扩展思考 Token 预算，0 表示不主动开启',
    updated_at DATETIME(6) NOT NULL COMMENT '配置最后更新时间，统一使用 UTC',
    PRIMARY KEY (singleton_id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci
  COMMENT='运维助手当前模型连接';

-- 会话是运维助手的持久化根对象。关联关系由应用事务维护，不使用数据库外键，
-- 避免删除、恢复和后续在线迁移被外键约束耦合。
CREATE TABLE assistant_conversations (
    id CHAR(36) NOT NULL COMMENT '会话 ID，使用 UUID',
    actor_id VARCHAR(128) NOT NULL COMMENT '会话所属用户的稳定标识',
    title VARCHAR(160) NOT NULL COMMENT '控制台展示的会话标题',
    created_at DATETIME(6) NOT NULL COMMENT '会话创建时间，统一使用 UTC',
    updated_at DATETIME(6) NOT NULL COMMENT '最近一次用户消息或执行终态时间，用于会话排序和游标分页',
    PRIMARY KEY (id),
    KEY idx_assistant_conversations_actor_updated (actor_id, updated_at DESC, id DESC)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci
  COMMENT='运维助手会话';

CREATE TABLE assistant_agent_executions (
    id CHAR(36) NOT NULL COMMENT '一次 Agent 执行 ID，使用 UUID',
    conversation_id CHAR(36) NOT NULL COMMENT '所属会话 ID，由应用事务保证关联有效',
    state TINYINT UNSIGNED NOT NULL COMMENT '执行状态：1 排队中，2 运行中，3 成功，4 失败，5 已取消',
    model VARCHAR(160) NOT NULL DEFAULT '' COMMENT '本次调用实际使用的模型名称；领取前为空',
    error_code VARCHAR(64) NOT NULL DEFAULT '' COMMENT '失败原因的稳定代码，非失败状态为空',
    cancellation_requested BOOLEAN NOT NULL DEFAULT FALSE COMMENT '运行中的执行是否已收到取消请求',
    worker_id VARCHAR(128) NOT NULL DEFAULT '' COMMENT '当前持有执行租约的 Assistant 进程及执行槽标识',
    lease_expires_at DATETIME(6) NULL COMMENT '执行租约到期时间；仅运行中使用',
    created_at DATETIME(6) NOT NULL COMMENT '执行创建时间，统一使用 UTC',
    started_at DATETIME(6) NULL COMMENT '模型调用开始时间；排队中为空',
    finished_at DATETIME(6) NULL COMMENT '模型调用结束时间；未到终态时为空',
    PRIMARY KEY (id),
    KEY idx_assistant_agent_executions_conversation_state (conversation_id, state),
    KEY idx_assistant_agent_executions_claim (state, created_at, id),
    KEY idx_assistant_agent_executions_expired_lease (state, lease_expires_at),
    KEY idx_assistant_agent_executions_conversation_created (conversation_id, created_at DESC, id DESC)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci
  COMMENT='运维助手 Agent 执行记录';

-- Execution Step 是一次 Agent 执行中可独立追踪的持久步骤。工具调用和结果共用同一条记录，
-- 通过状态与脱敏摘要表达生命周期，不复制保存原始工具输出。
CREATE TABLE assistant_agent_execution_steps (
    id CHAR(36) NOT NULL COMMENT '执行步骤 ID，使用 UUID',
    execution_id CHAR(36) NOT NULL COMMENT '所属 Agent 执行 ID，由应用事务保证关联有效',
    sequence INT UNSIGNED NOT NULL COMMENT '步骤在所属执行内的稳定顺序，从 1 开始递增',
    kind TINYINT UNSIGNED NOT NULL COMMENT '步骤类型：1 模型调用，2 工具调用',
    state TINYINT UNSIGNED NOT NULL COMMENT '步骤状态：1 执行中，2 完成，3 失败，4 已取消',
    name VARCHAR(160) NOT NULL COMMENT '模型或工具的稳定名称',
    call_id VARCHAR(128) NOT NULL COMMENT '关联一次调用及其结果的稳定标识',
    summary VARCHAR(1024) NOT NULL DEFAULT '' COMMENT '经过脱敏且可向用户展示的执行摘要',
    error_code VARCHAR(64) NOT NULL DEFAULT '' COMMENT '失败原因的稳定代码，非失败状态为空',
    started_at DATETIME(6) NOT NULL COMMENT '步骤开始执行时间',
    finished_at DATETIME(6) NULL COMMENT '步骤结束时间；未到终态时为空',
    PRIMARY KEY (id),
    UNIQUE KEY uk_assistant_agent_execution_steps_sequence (execution_id, sequence),
    UNIQUE KEY uk_assistant_agent_execution_steps_call_kind (execution_id, call_id, kind)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci
  COMMENT='运维助手执行步骤';

CREATE TABLE assistant_messages (
    id CHAR(36) NOT NULL COMMENT '消息 ID，使用 UUID',
    conversation_id CHAR(36) NOT NULL COMMENT '所属会话 ID，由应用事务保证关联有效',
    execution_id CHAR(36) NOT NULL COMMENT '产生该消息的 Agent 执行 ID，由应用事务保证关联有效',
    role TINYINT UNSIGNED NOT NULL COMMENT '消息角色：1 表示用户，2 表示助手',
    content MEDIUMTEXT NOT NULL COMMENT '用户输入或模型最终回复，不保存流式增量事件',
    reasoning_content MEDIUMTEXT NOT NULL COMMENT '模型明确返回的推理内容；用户消息为空，且不会作为后续模型上下文',
    created_at DATETIME(6) NOT NULL COMMENT '消息创建时间，统一使用 UTC',
    PRIMARY KEY (id),
    KEY idx_assistant_messages_conversation_created (conversation_id, created_at, id),
    KEY idx_assistant_messages_execution (execution_id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci
  COMMENT='运维助手会话消息';

-- +goose Down
DROP TABLE assistant_messages;
DROP TABLE assistant_agent_execution_steps;
DROP TABLE assistant_agent_executions;
DROP TABLE assistant_conversations;
DROP TABLE assistant_model_connections;
