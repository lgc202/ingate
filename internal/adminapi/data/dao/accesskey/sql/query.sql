-- name: ListAccessKeys :many
SELECT id, name, secret_hash, secret_prefix, secret_suffix, enabled,
       allowed_models, expires_at, created_at, updated_at
FROM access_keys
ORDER BY created_at DESC, id;

-- name: GetAccessKey :one
SELECT id, name, secret_hash, secret_prefix, secret_suffix, enabled,
       allowed_models, expires_at, created_at, updated_at
FROM access_keys
WHERE id = ?;

-- name: CountAccessKeysByName :one
SELECT COUNT(*)
FROM access_keys
WHERE name = ? AND id <> ?;

-- name: CreateAccessKey :exec
INSERT INTO access_keys (
    id, name, secret_hash, secret_prefix, secret_suffix, enabled,
    allowed_models, expires_at, created_at, updated_at
) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?);

-- name: UpdateAccessKey :execresult
UPDATE access_keys
SET name = ?, allowed_models = ?, expires_at = ?, updated_at = ?
WHERE id = ?;

-- name: SetAccessKeyEnabled :execresult
UPDATE access_keys
SET enabled = ?, updated_at = ?
WHERE id = ?;

-- name: RotateAccessKey :execresult
UPDATE access_keys
SET secret_hash = ?, secret_prefix = ?, secret_suffix = ?, updated_at = ?
WHERE id = ?;

-- name: DeleteAccessKey :execresult
DELETE FROM access_keys
WHERE id = ?;
