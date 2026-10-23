-- name: CreatePaymentAttempt :one
INSERT INTO payments (
    order_id,
    attempt,
    idempotency_key,
    amount_paise,
    currency,
    status,
    payment_method,
    transaction_reference,
    failure_reason
) VALUES (
    $1, $2, $3, $4, $5, $6, $7, $8, $9
) RETURNING id, order_id, attempt, idempotency_key, amount_paise, currency, status, payment_method, transaction_reference, failure_reason, created_at, updated_at;

-- name: UpdatePaymentStatus :one
UPDATE payments
SET 
    status = $2,
    transaction_reference = $3,
    failure_reason = $4,
    updated_at = NOW()
WHERE id = $1
RETURNING id, order_id, attempt, idempotency_key, amount_paise, currency, status, payment_method, transaction_reference, failure_reason, created_at, updated_at;

-- name: GetPaymentByOrderAndAttempt :one
SELECT id, order_id, attempt, idempotency_key, amount_paise, currency, status, payment_method, transaction_reference, failure_reason, created_at, updated_at
FROM payments
WHERE order_id = $1 AND attempt = $2;

-- name: GetPaymentByOrderAndIdempotencyKey :one
SELECT id, order_id, attempt, idempotency_key, amount_paise, currency, status, payment_method, transaction_reference, failure_reason, created_at, updated_at
FROM payments
WHERE order_id = $1 AND idempotency_key = $2;

-- name: GetLatestPaymentForOrder :one
SELECT id, order_id, attempt, idempotency_key, amount_paise, currency, status, payment_method, transaction_reference, failure_reason, created_at, updated_at
FROM payments
WHERE order_id = $1
ORDER BY attempt DESC
LIMIT 1;

-- name: CreateRefund :one
INSERT INTO refunds (
    payment_id,
    amount_paise,
    reason,
    status
) VALUES (
    $1, $2, $3, $4
) RETURNING id, payment_id, amount_paise, reason, status, created_at;

-- name: GetRefundsByPaymentID :many
SELECT id, payment_id, amount_paise, reason, status, created_at
FROM refunds
WHERE payment_id = $1
ORDER BY created_at ASC;
