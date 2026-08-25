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

-- name: DeleteRunItemsByConversation :exec
DELETE i
FROM assistant_run_items AS i
JOIN assistant_runs AS r ON r.id = i.run_id
WHERE r.conversation_id = ?;

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

-- name: GetRunForWorkerUpdate :one
SELECT id
FROM assistant_runs
WHERE id = ? AND state = 2 AND worker_id = ?
FOR UPDATE;

-- name: NextRunItemSequence :one
SELECT CAST(COALESCE(MAX(sequence), 0) + 1 AS UNSIGNED)
FROM assistant_run_items
WHERE run_id = ?;

-- name: CreateRunItem :exec
INSERT INTO assistant_run_items (
    id, run_id, sequence, kind, state, name, call_id, summary,
    error_code, created_at, started_at, finished_at
) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?);

-- name: ListRunItems :many
SELECT i.id, i.run_id, i.sequence, i.kind, i.state, i.name, i.call_id,
       i.summary, i.error_code, i.created_at, i.started_at, i.finished_at
FROM assistant_run_items AS i
JOIN assistant_runs AS r ON r.id = i.run_id
JOIN assistant_conversations AS c ON c.id = r.conversation_id
WHERE i.run_id = ? AND c.actor_id = ?
ORDER BY i.sequence ASC;

-- name: CompleteRunningRunItems :exec
UPDATE assistant_run_items
SET state = 3, finished_at = ?
WHERE run_id = ? AND state = 2;

-- name: FailRunningRunItems :exec
UPDATE assistant_run_items
SET state = 4, error_code = ?, finished_at = ?
WHERE run_id = ? AND state = 2;

-- name: CancelRunningRunItems :exec
UPDATE assistant_run_items
SET state = 5, finished_at = ?
WHERE run_id = ? AND state = 2;

-- name: FailExpiredRunItems :exec
UPDATE assistant_run_items AS i
JOIN assistant_runs AS r ON r.id = i.run_id
SET i.state = 4, i.error_code = ?, i.finished_at = ?
WHERE i.state = 2 AND r.state = 2 AND r.lease_expires_at < ?;

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
