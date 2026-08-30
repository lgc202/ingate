-- name: CurrentTime :one
SELECT CURRENT_TIMESTAMP(6) AS database_now;

-- name: CreateConversation :exec
INSERT INTO assistant_conversations (
    id, actor_id, title, created_at, updated_at
) VALUES (?, ?, ?, ?, ?);

-- name: GetConversation :one
SELECT id, actor_id, title, created_at, updated_at
FROM assistant_conversations
WHERE id = ? AND actor_id = ?;

-- name: GetConversationForUpdate :one
SELECT id, actor_id, title, created_at, updated_at
FROM assistant_conversations
WHERE id = ? AND actor_id = ?
FOR UPDATE;

-- name: UpdateConversationTitle :execrows
UPDATE assistant_conversations
SET title = ?, updated_at = CURRENT_TIMESTAMP(6)
WHERE id = ? AND actor_id = ?;

-- name: ListConversations :many
SELECT id, actor_id, title, created_at, updated_at
FROM assistant_conversations
WHERE actor_id = ?
  AND (updated_at < ? OR (updated_at = ? AND id < ?))
ORDER BY updated_at DESC, id DESC
LIMIT ?;

-- name: TouchConversation :exec
UPDATE assistant_conversations
SET updated_at = CURRENT_TIMESTAMP(6)
WHERE id = ? AND actor_id = ?;

-- name: DeleteConversation :execrows
DELETE FROM assistant_conversations
WHERE id = ? AND actor_id = ?;

-- name: DeleteMessagesByConversation :exec
DELETE FROM assistant_messages
WHERE conversation_id = ?;

-- name: DeleteExecutionStepsByConversation :exec
DELETE i
FROM assistant_agent_execution_steps AS i
JOIN assistant_agent_executions AS e ON e.id = i.execution_id
WHERE e.conversation_id = ?;

-- name: DeleteExecutionsByConversation :exec
DELETE FROM assistant_agent_executions
WHERE conversation_id = ?;

-- name: CreateExecution :exec
INSERT INTO assistant_agent_executions (
    id, conversation_id, state, created_at
) VALUES (?, ?, 1, ?);

-- name: CountActiveExecutions :one
SELECT COUNT(*)
FROM assistant_agent_executions
WHERE conversation_id = ? AND state IN (1, 2);

-- name: GetExecution :one
SELECT r.id, r.conversation_id, r.state, r.model, r.error_code,
       r.cancellation_requested, r.worker_id, r.lease_expires_at,
       r.created_at, r.started_at, r.finished_at
FROM assistant_agent_executions AS r
JOIN assistant_conversations AS c ON c.id = r.conversation_id
WHERE r.id = ? AND c.actor_id = ?;

-- name: GetExecutionForUpdate :one
SELECT r.id, r.conversation_id, r.state, r.model, r.error_code,
       r.cancellation_requested, r.worker_id, r.lease_expires_at,
       r.created_at, r.started_at, r.finished_at
FROM assistant_agent_executions AS r
JOIN assistant_conversations AS c ON c.id = r.conversation_id
WHERE r.id = ? AND c.actor_id = ?
FOR UPDATE;

-- name: ClaimNextExecution :one
SELECT r.id, r.conversation_id, c.actor_id
FROM assistant_agent_executions AS r
JOIN assistant_conversations AS c ON c.id = r.conversation_id
WHERE r.state = 1
ORDER BY r.created_at ASC, r.id ASC
LIMIT 1
FOR UPDATE SKIP LOCKED;

-- name: StartExecution :execrows
UPDATE assistant_agent_executions
SET state = 2,
    worker_id = ?,
    lease_expires_at = TIMESTAMPADD(
        MICROSECOND,
        CAST(sqlc.arg(lease_duration_microseconds) AS SIGNED),
        CURRENT_TIMESTAMP(6)
    ),
    started_at = CURRENT_TIMESTAMP(6)
WHERE id = ? AND state = 1;

-- name: SetExecutionModel :execrows
UPDATE assistant_agent_executions
SET model = ?
WHERE id = ? AND state = 2 AND worker_id = ?;

-- name: GetExecutionForWorkerUpdate :one
SELECT id
FROM assistant_agent_executions
WHERE id = ? AND state = 2 AND worker_id = ?
FOR UPDATE;

-- name: NextExecutionStepSequence :one
SELECT CAST(COALESCE(MAX(sequence), 0) + 1 AS UNSIGNED)
FROM assistant_agent_execution_steps
WHERE execution_id = ?;

-- name: CreateExecutionStep :exec
INSERT INTO assistant_agent_execution_steps (
    id, execution_id, sequence, kind, state, name, call_id, summary,
    error_code, started_at, finished_at
) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, CURRENT_TIMESTAMP(6), NULL);

-- name: ListExecutionSteps :many
SELECT i.id, i.execution_id, i.sequence, i.kind, i.state, i.name, i.call_id,
       i.summary, i.error_code, i.started_at, i.finished_at
FROM assistant_agent_execution_steps AS i
JOIN assistant_agent_executions AS r ON r.id = i.execution_id
JOIN assistant_conversations AS c ON c.id = r.conversation_id
WHERE i.execution_id = ? AND c.actor_id = ?
ORDER BY i.sequence ASC;

-- name: CompleteExecutionStep :execrows
UPDATE assistant_agent_execution_steps AS i
JOIN assistant_agent_executions AS r ON r.id = i.execution_id
SET i.state = 2, i.summary = ?, i.finished_at = CURRENT_TIMESTAMP(6)
WHERE i.execution_id = ? AND i.call_id = ? AND i.kind = ? AND i.state = 1
  AND r.state = 2 AND r.worker_id = ?;

-- name: FailExecutionStep :execrows
UPDATE assistant_agent_execution_steps AS i
JOIN assistant_agent_executions AS r ON r.id = i.execution_id
SET i.state = 3, i.error_code = ?, i.finished_at = CURRENT_TIMESTAMP(6)
WHERE i.execution_id = ? AND i.call_id = ? AND i.kind = ? AND i.state = 1
  AND r.state = 2 AND r.worker_id = ?;

-- name: FailRunningExecutionSteps :exec
UPDATE assistant_agent_execution_steps
SET state = 3, error_code = ?, finished_at = CURRENT_TIMESTAMP(6)
WHERE execution_id = ? AND state = 1;

-- name: CancelRunningExecutionSteps :exec
UPDATE assistant_agent_execution_steps
SET state = 4, finished_at = CURRENT_TIMESTAMP(6)
WHERE execution_id = ? AND state = 1;

-- name: TouchExpiredExecutionConversations :exec
UPDATE assistant_conversations AS c
JOIN assistant_agent_executions AS r ON r.conversation_id = c.id
SET c.updated_at = CURRENT_TIMESTAMP(6)
WHERE r.state = 2 AND r.lease_expires_at < CURRENT_TIMESTAMP(6);

-- name: FailExpiredExecutionSteps :exec
UPDATE assistant_agent_execution_steps AS i
JOIN assistant_agent_executions AS r ON r.id = i.execution_id
SET i.state = 3, i.error_code = ?, i.finished_at = CURRENT_TIMESTAMP(6)
WHERE i.state = 1 AND r.state = 2 AND r.lease_expires_at < CURRENT_TIMESTAMP(6);

-- name: RenewExecutionLease :execrows
UPDATE assistant_agent_executions
SET lease_expires_at = TIMESTAMPADD(
    MICROSECOND,
    CAST(sqlc.arg(lease_duration_microseconds) AS SIGNED),
    CURRENT_TIMESTAMP(6)
)
WHERE id = ? AND state = 2 AND worker_id = ?;

-- name: ExecutionCancellationRequested :one
SELECT cancellation_requested
FROM assistant_agent_executions
WHERE id = ? AND state = 2 AND worker_id = ?;

-- name: CompleteExecution :execrows
UPDATE assistant_agent_executions
SET state = 3, worker_id = '', lease_expires_at = NULL, finished_at = ?
WHERE id = ? AND state = 2 AND worker_id = ?;

-- name: FailExecution :execrows
UPDATE assistant_agent_executions
SET state = 4, error_code = ?, worker_id = '', lease_expires_at = NULL,
    finished_at = CURRENT_TIMESTAMP(6)
WHERE id = ? AND state = 2 AND worker_id = ?;

-- name: CancelQueuedExecution :execrows
UPDATE assistant_agent_executions
SET state = 5, finished_at = ?
WHERE id = ? AND state = 1;

-- name: RequestExecutionCancellation :execrows
UPDATE assistant_agent_executions
SET cancellation_requested = TRUE
WHERE id = ? AND state = 2;

-- name: FinishExecutionCancellation :execrows
UPDATE assistant_agent_executions
SET state = 5, worker_id = '', lease_expires_at = NULL,
    finished_at = CURRENT_TIMESTAMP(6)
WHERE id = ? AND state = 2 AND worker_id = ? AND cancellation_requested = TRUE;

-- name: FailExpiredExecutions :execrows
UPDATE assistant_agent_executions
SET state = 4, error_code = ?, worker_id = '', lease_expires_at = NULL,
    finished_at = CURRENT_TIMESTAMP(6)
WHERE state = 2 AND lease_expires_at < CURRENT_TIMESTAMP(6);

-- name: CreateMessage :exec
INSERT INTO assistant_messages (
    id, conversation_id, execution_id, role, content, reasoning_content, created_at
) VALUES (?, ?, ?, ?, ?, ?, ?);

-- name: ListMessages :many
SELECT id, conversation_id, execution_id, role, content, reasoning_content, created_at
FROM assistant_messages
WHERE conversation_id = ?
  AND (created_at > ? OR (created_at = ? AND id > ?))
ORDER BY created_at ASC, id ASC
LIMIT ?;

-- name: ListRecentMessageSizes :many
SELECT OCTET_LENGTH(content) AS content_bytes
FROM assistant_messages
WHERE conversation_id = ?
ORDER BY created_at DESC, id DESC
LIMIT ?;

-- name: ListRecentMessages :many
SELECT role, content
FROM assistant_messages
WHERE conversation_id = ?
ORDER BY created_at DESC, id DESC
LIMIT ?;

-- name: GetAssistantModelConnection :one
SELECT singleton_id, connection_mode, protocol, endpoint, api_key, model,
       timeout_ms, max_output_tokens, reasoning_budget_tokens, updated_at
FROM assistant_model_connections
WHERE singleton_id = 1;

-- name: GetAssistantModelConnectionForUpdate :one
SELECT singleton_id, connection_mode, protocol, endpoint, api_key, model,
       timeout_ms, max_output_tokens, reasoning_budget_tokens, updated_at
FROM assistant_model_connections
WHERE singleton_id = 1
FOR UPDATE;

-- name: UpsertAssistantModelConnection :exec
INSERT INTO assistant_model_connections (
    singleton_id, connection_mode, protocol, endpoint, api_key, model,
    timeout_ms, max_output_tokens, reasoning_budget_tokens, updated_at
) VALUES (1, ?, ?, ?, ?, ?, ?, ?, ?, ?)
ON DUPLICATE KEY UPDATE
    connection_mode = VALUES(connection_mode),
    protocol = VALUES(protocol),
    endpoint = VALUES(endpoint),
    api_key = VALUES(api_key),
    model = VALUES(model),
    timeout_ms = VALUES(timeout_ms),
    max_output_tokens = VALUES(max_output_tokens),
    reasoning_budget_tokens = VALUES(reasoning_budget_tokens),
    updated_at = VALUES(updated_at);
