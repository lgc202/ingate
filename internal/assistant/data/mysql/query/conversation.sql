-- name: CreateConversation :exec
INSERT INTO assistant_conversations (
    id, actor_id, title, version, next_message_sequence, created_at, updated_at
) VALUES (?, ?, ?, 1, 1, ?, ?);

-- name: GetConversation :one
SELECT id, actor_id, title, version, next_message_sequence, created_at, updated_at
FROM assistant_conversations
WHERE id = ? AND actor_id = ?;

-- name: GetConversationForUpdate :one
SELECT id, actor_id, title, version, next_message_sequence, created_at, updated_at
FROM assistant_conversations
WHERE id = ? AND actor_id = ?
FOR UPDATE;

-- name: ListConversations :many
SELECT id, actor_id, title, version, next_message_sequence, created_at, updated_at
FROM assistant_conversations
WHERE actor_id = ?
  AND (updated_at < ? OR (updated_at = ? AND id < ?))
ORDER BY updated_at DESC, id DESC
LIMIT ?;

-- name: DeleteConversation :execrows
DELETE FROM assistant_conversations
WHERE id = ? AND actor_id = ? AND version = ?;

-- name: AllocateMessageSequence :execresult
UPDATE assistant_conversations
SET next_message_sequence = LAST_INSERT_ID(next_message_sequence + 1),
    version = version + 1,
    updated_at = ?
WHERE id = ? AND actor_id = ?;

-- name: CreateMessage :exec
INSERT INTO assistant_messages (
    id, conversation_id, sequence, role, content, created_at
) VALUES (?, ?, ?, ?, ?, ?);

-- name: ListMessages :many
SELECT id, conversation_id, sequence, role, content, created_at
FROM assistant_messages
WHERE conversation_id = ? AND sequence > ?
ORDER BY sequence ASC
LIMIT ?;

-- name: CreateExecution :exec
INSERT INTO assistant_executions (
    id, conversation_id, user_message_id, state, model, failure_code, started_at
) VALUES (?, ?, ?, ?, ?, '', ?);

-- name: CountRunningExecutions :one
SELECT COUNT(*)
FROM assistant_executions
WHERE conversation_id = ? AND state = 'running';

-- name: GetExecution :one
SELECT e.id, e.conversation_id, e.user_message_id, e.assistant_message_id, e.state, e.model,
       e.failure_code, e.started_at, e.finished_at
FROM assistant_executions AS e
JOIN assistant_conversations AS c ON c.id = e.conversation_id
WHERE e.id = ? AND c.actor_id = ?;

-- name: GetExecutionForUpdate :one
SELECT e.id, e.conversation_id, e.user_message_id, e.assistant_message_id, e.state, e.model,
       e.failure_code, e.started_at, e.finished_at
FROM assistant_executions AS e
JOIN assistant_conversations AS c ON c.id = e.conversation_id
WHERE e.id = ? AND c.actor_id = ?
FOR UPDATE;

-- name: CompleteExecution :execrows
UPDATE assistant_executions
SET state = 'succeeded', assistant_message_id = ?, finished_at = ?
WHERE id = ? AND state = 'running';

-- name: FailExecution :execrows
UPDATE assistant_executions
SET state = 'failed', failure_code = ?, finished_at = ?
WHERE id = ? AND state = 'running';
