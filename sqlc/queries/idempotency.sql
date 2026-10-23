-- Idempotency Domain Queries

-- name: GetIdempotencyRecord :one
SELECT id, key, user_id, request_path, request_hash, response_code, response_body, status, locked_until, created_at, updated_at
FROM idempotency_keys
WHERE user_id = $1 AND key = $2;

-- name: CreateIdempotencyRecord :one
INSERT INTO idempotency_keys (
    key, user_id, request_path, request_hash, status, locked_until
) VALUES (
    $1, $2, $3, $4, $5, $6
) RETURNING id, key, user_id, request_path, request_hash, response_code, response_body, status, locked_until, created_at, updated_at;

-- name: UpdateIdempotencyRecord :one
UPDATE idempotency_keys
SET 
    status = $3,
    response_code = $4,
    response_body = $5,
    updated_at = NOW()
WHERE user_id = $1 AND key = $2
RETURNING id, key, user_id, request_path, request_hash, response_code, response_body, status, locked_until, created_at, updated_at;
