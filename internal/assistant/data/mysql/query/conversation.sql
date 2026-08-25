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

-- name: ListConversations :many
SELECT id, actor_id, title, created_at, updated_at
FROM assistant_conversations
WHERE actor_id = ?
  AND (updated_at < ? OR (updated_at = ? AND id < ?))
ORDER BY updated_at DESC, id DESC
LIMIT ?;

-- name: TouchConversation :exec
UPDATE assistant_conversations
SET updated_at = ?
WHERE id = ? AND actor_id = ?;

-- name: DeleteConversation :execrows
DELETE FROM assistant_conversations
WHERE id = ? AND actor_id = ?;

-- name: DeleteMessagesByConversation :exec
DELETE FROM assistant_messages
WHERE conversation_id = ?;

-- name: DeleteRunsByConversation :exec
DELETE FROM assistant_runs
WHERE conversation_id = ?;

-- name: CreateRun :exec
INSERT INTO assistant_runs (
    id, conversation_id, state, created_at
) VALUES (?, ?, 1, ?);

-- name: CountActiveRuns :one
SELECT COUNT(*)
FROM assistant_runs
WHERE conversation_id = ? AND state IN (1, 2);

-- name: GetRun :one
SELECT r.id, r.conversation_id, r.state, r.model, r.error_code,
       r.cancellation_requested, r.worker_id, r.lease_expires_at,
       r.created_at, r.started_at, r.finished_at
FROM assistant_runs AS r
JOIN assistant_conversations AS c ON c.id = r.conversation_id
WHERE r.id = ? AND c.actor_id = ?;

-- name: GetRunForUpdate :one
SELECT r.id, r.conversation_id, r.state, r.model, r.error_code,
       r.cancellation_requested, r.worker_id, r.lease_expires_at,
       r.created_at, r.started_at, r.finished_at
FROM assistant_runs AS r
JOIN assistant_conversations AS c ON c.id = r.conversation_id
WHERE r.id = ? AND c.actor_id = ?
FOR UPDATE;

-- name: ClaimNextRun :one
SELECT r.id, r.conversation_id, c.actor_id, r.created_at
FROM assistant_runs AS r
JOIN assistant_conversations AS c ON c.id = r.conversation_id
WHERE r.state = 1
ORDER BY r.created_at ASC, r.id ASC
LIMIT 1
FOR UPDATE SKIP LOCKED;

-- name: StartRun :execrows
UPDATE assistant_runs
SET state = 2, worker_id = ?, lease_expires_at = ?, started_at = ?
WHERE id = ? AND state = 1;

-- name: SetRunModel :execrows
UPDATE assistant_runs
SET model = ?
WHERE id = ? AND state = 2 AND worker_id = ?;

-- name: RenewRunLease :execrows
UPDATE assistant_runs
SET lease_expires_at = ?
WHERE id = ? AND state = 2 AND worker_id = ?;

-- name: RunCancellationRequested :one
SELECT cancellation_requested
FROM assistant_runs
WHERE id = ? AND state = 2 AND worker_id = ?;

-- name: CompleteRun :execrows
UPDATE assistant_runs
SET state = 3, worker_id = '', lease_expires_at = NULL, finished_at = ?
WHERE id = ? AND state = 2 AND worker_id = ?;

-- name: FailRun :execrows
UPDATE assistant_runs
SET state = 4, error_code = ?, worker_id = '', lease_expires_at = NULL, finished_at = ?
WHERE id = ? AND state = 2 AND worker_id = ?;

-- name: CancelQueuedRun :execrows
UPDATE assistant_runs
SET state = 5, finished_at = ?
WHERE id = ? AND state = 1;

-- name: RequestRunCancellation :execrows
UPDATE assistant_runs
SET cancellation_requested = TRUE
WHERE id = ? AND state = 2;

-- name: FinishRunCancellation :execrows
UPDATE assistant_runs
SET state = 5, worker_id = '', lease_expires_at = NULL, finished_at = ?
WHERE id = ? AND state = 2 AND worker_id = ? AND cancellation_requested = TRUE;

-- name: FailExpiredRuns :execrows
UPDATE assistant_runs
SET state = 4, error_code = ?, worker_id = '', lease_expires_at = NULL, finished_at = ?
WHERE state = 2 AND lease_expires_at < ?;

-- name: CreateMessage :exec
INSERT INTO assistant_messages (
    id, conversation_id, run_id, role, content, reasoning_content, created_at
) VALUES (?, ?, ?, ?, ?, ?, ?);

-- name: ListMessages :many
SELECT id, conversation_id, run_id, role, content, reasoning_content, created_at
FROM assistant_messages
WHERE conversation_id = ?
  AND (created_at > ? OR (created_at = ? AND id > ?))
ORDER BY created_at ASC, id ASC
LIMIT ?;

-- name: ListRecentMessages :many
SELECT id, conversation_id, run_id, role, content, reasoning_content, created_at
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
