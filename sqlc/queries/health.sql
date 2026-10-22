-- Health and Diagnostics Queries

-- name: CheckDatabaseHealth :one
SELECT 1 as healthy;

-- name: GetDatabaseVersion :one
SELECT version();
