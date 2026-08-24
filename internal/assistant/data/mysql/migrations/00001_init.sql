-- +goose Up
CREATE TABLE assistant_conversations (
    id CHAR(36) NOT NULL,
    actor_id VARCHAR(128) NOT NULL,
    title VARCHAR(160) NOT NULL,
    version BIGINT UNSIGNED NOT NULL DEFAULT 1,
    next_message_sequence BIGINT UNSIGNED NOT NULL DEFAULT 1,
    created_at DATETIME(6) NOT NULL,
    updated_at DATETIME(6) NOT NULL,
    PRIMARY KEY (id),
    KEY idx_assistant_conversations_actor_updated (actor_id, updated_at DESC, id DESC)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;

CREATE TABLE assistant_messages (
    id CHAR(36) NOT NULL,
    conversation_id CHAR(36) NOT NULL,
    sequence BIGINT UNSIGNED NOT NULL,
    role VARCHAR(16) NOT NULL,
    content MEDIUMTEXT NOT NULL,
    created_at DATETIME(6) NOT NULL,
    PRIMARY KEY (id),
    UNIQUE KEY uk_assistant_messages_conversation_sequence (conversation_id, sequence),
    CONSTRAINT fk_assistant_messages_conversation
        FOREIGN KEY (conversation_id) REFERENCES assistant_conversations (id) ON DELETE CASCADE
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;

CREATE TABLE assistant_executions (
    id CHAR(36) NOT NULL,
    conversation_id CHAR(36) NOT NULL,
    user_message_id CHAR(36) NOT NULL,
    assistant_message_id CHAR(36) NULL,
    state VARCHAR(16) NOT NULL,
    model VARCHAR(160) NOT NULL,
    failure_code VARCHAR(64) NOT NULL DEFAULT '',
    started_at DATETIME(6) NOT NULL,
    finished_at DATETIME(6) NULL,
    PRIMARY KEY (id),
    KEY idx_assistant_executions_conversation_started (conversation_id, started_at DESC, id DESC),
    CONSTRAINT fk_assistant_executions_conversation
        FOREIGN KEY (conversation_id) REFERENCES assistant_conversations (id) ON DELETE CASCADE,
    CONSTRAINT fk_assistant_executions_user_message
        FOREIGN KEY (user_message_id) REFERENCES assistant_messages (id),
    CONSTRAINT fk_assistant_executions_assistant_message
        FOREIGN KEY (assistant_message_id) REFERENCES assistant_messages (id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;

-- +goose Down
DROP TABLE assistant_executions;
DROP TABLE assistant_messages;
DROP TABLE assistant_conversations;
