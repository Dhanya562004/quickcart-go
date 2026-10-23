-- User and Authentication Queries

-- name: CreateUser :one
INSERT INTO users (
    id, email, password_hash, full_name, phone, role, created_at, updated_at
) VALUES (
    gen_random_uuid(), $1, $2, $3, $4, $5, NOW(), NOW()
)
RETURNING *;

-- name: GetUserByEmail :one
SELECT * FROM users
WHERE email = $1;

-- name: GetUserByID :one
SELECT * FROM users
WHERE id = $1;

-- name: CreateRefreshToken :one
INSERT INTO refresh_tokens (
    id, user_id, token_hash, revoked, expires_at, created_at, updated_at
) VALUES (
    gen_random_uuid(), $1, $2, false, $3, NOW(), NOW()
)
RETURNING *;

-- name: GetRefreshTokenByHash :one
SELECT * FROM refresh_tokens
WHERE token_hash = $1;

-- name: RevokeRefreshToken :one
UPDATE refresh_tokens
SET revoked = true, updated_at = NOW()
WHERE token_hash = $1 AND revoked = false
RETURNING *;

-- name: RevokeAllUserRefreshTokens :exec
UPDATE refresh_tokens
SET revoked = true, updated_at = NOW()
WHERE user_id = $1 AND revoked = false;

-- name: PickActiveRefreshTokenForRotation :one
UPDATE refresh_tokens
SET revoked = true, updated_at = NOW()
WHERE token_hash = $1 AND revoked = false AND expires_at > NOW()
RETURNING *;
