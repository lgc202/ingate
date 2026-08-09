-- name: ListAccessKeys :many
SELECT id, version, name, secret_hash, secret_prefix, secret_suffix, enabled,
       allowed_models, expires_at, created_at, updated_at
FROM access_keys
ORDER BY created_at DESC, id;

-- name: ListAccessKeysPage :many
SELECT id, version, name, secret_hash, secret_prefix, secret_suffix, enabled,
       allowed_models, expires_at, created_at, updated_at
FROM access_keys
ORDER BY created_at DESC, id
LIMIT ?;

-- name: ListAccessKeysAfter :many
SELECT id, version, name, secret_hash, secret_prefix, secret_suffix, enabled,
       allowed_models, expires_at, created_at, updated_at
FROM access_keys
WHERE created_at < sqlc.arg(cursor_created_at)
   OR (created_at = sqlc.arg(cursor_created_at) AND id > sqlc.arg(cursor_id))
ORDER BY created_at DESC, id
LIMIT ?;

-- name: GetAccessKey :one
SELECT id, version, name, secret_hash, secret_prefix, secret_suffix, enabled,
       allowed_models, expires_at, created_at, updated_at
FROM access_keys
WHERE id = ?;

-- name: CountAccessKeysByName :one
SELECT COUNT(*)
FROM access_keys
WHERE name = ? AND id <> ?;

-- name: CreateAccessKey :exec
INSERT INTO access_keys (
    id, version, name, secret_hash, secret_prefix, secret_suffix, enabled,
    allowed_models, expires_at, created_at, updated_at
) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?);

-- name: UpdateAccessKey :execresult
UPDATE access_keys
SET version = version + 1,
    name = sqlc.arg(name),
    allowed_models = sqlc.arg(allowed_models),
    expires_at = sqlc.arg(expires_at),
    updated_at = sqlc.arg(updated_at)
WHERE id = sqlc.arg(id) AND version = sqlc.arg(expected_version);

-- name: SetAccessKeyEnabled :execresult
UPDATE access_keys
SET version = version + 1,
    enabled = sqlc.arg(enabled),
    updated_at = sqlc.arg(updated_at)
WHERE id = sqlc.arg(id) AND version = sqlc.arg(expected_version);

-- name: RotateAccessKey :execresult
UPDATE access_keys
SET version = version + 1,
    secret_hash = sqlc.arg(secret_hash),
    secret_prefix = sqlc.arg(secret_prefix),
    secret_suffix = sqlc.arg(secret_suffix),
    updated_at = sqlc.arg(updated_at)
WHERE id = sqlc.arg(id) AND version = sqlc.arg(expected_version);

-- name: DeleteAccessKey :execresult
DELETE FROM access_keys
WHERE id = sqlc.arg(id) AND version = sqlc.arg(expected_version);
