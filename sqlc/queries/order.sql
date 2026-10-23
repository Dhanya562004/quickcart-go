-- Order Domain Queries

-- name: CreateOrder :one
INSERT INTO orders (
    user_id,
    store_id,
    status,
    total_amount_paise,
    delivery_address,
    idempotency_key
) VALUES (
    $1, $2, $3, $4, $5, $6
) RETURNING id, order_number, user_id, store_id, status, total_amount_paise, delivery_address, idempotency_key, cancellation_reason, created_at, updated_at;

-- name: GetOrderByID :one
SELECT id, order_number, user_id, store_id, status, total_amount_paise, delivery_address, idempotency_key, cancellation_reason, created_at, updated_at
FROM orders
WHERE id = $1;

-- name: GetOrderByUserAndIdempotencyKey :one
SELECT id, order_number, user_id, store_id, status, total_amount_paise, delivery_address, idempotency_key, cancellation_reason, created_at, updated_at
FROM orders
WHERE user_id = $1 AND idempotency_key = $2;

-- name: ListOrdersByUserID :many
SELECT id, order_number, user_id, store_id, status, total_amount_paise, delivery_address, idempotency_key, cancellation_reason, created_at, updated_at
FROM orders
WHERE user_id = $1
ORDER BY created_at DESC
LIMIT $2 OFFSET $3;

-- name: CreateOrderItem :one
INSERT INTO order_items (
    order_id,
    product_id,
    quantity,
    unit_price_paise,
    total_price_paise
) VALUES (
    $1, $2, $3, $4, $5
) RETURNING id, order_id, product_id, quantity, unit_price_paise, total_price_paise, created_at;

-- name: GetOrderItemsByOrderID :many
SELECT 
    oi.id,
    oi.order_id,
    oi.product_id,
    oi.quantity,
    oi.unit_price_paise,
    oi.total_price_paise,
    oi.created_at,
    p.name AS product_name,
    p.sku AS product_sku,
    p.unit AS product_unit
FROM order_items oi
INNER JOIN products p ON oi.product_id = p.id
WHERE oi.order_id = $1
ORDER BY oi.created_at ASC;

-- name: GetReservationsByOrderID :many
SELECT id, order_id, store_id, product_id, quantity, status, expires_at, created_at, updated_at
FROM inventory_reservations
WHERE order_id = $1;

-- name: UpdateOrderStatus :one
UPDATE orders
SET 
    status = $2,
    cancellation_reason = COALESCE($3, cancellation_reason),
    updated_at = NOW()
WHERE id = $1
RETURNING id, order_number, user_id, store_id, status, total_amount_paise, delivery_address, idempotency_key, cancellation_reason, created_at, updated_at;

-- name: CancelOrderAtomic :one
UPDATE orders
SET 
    status = 'CANCELLED',
    cancellation_reason = COALESCE($2, cancellation_reason),
    updated_at = NOW()
WHERE id = $1 AND status IN ('PLACED', 'CONFIRMED')
RETURNING id, order_number, user_id, store_id, status, total_amount_paise, delivery_address, idempotency_key, cancellation_reason, created_at, updated_at;

-- name: ReleaseReservationsByOrderID :many
UPDATE inventory_reservations
SET 
    status = 'RELEASED',
    updated_at = NOW()
WHERE order_id = $1 AND status = 'RESERVED'
RETURNING id, order_id, store_id, product_id, quantity, status, expires_at, created_at, updated_at;

-- name: InsertOrderStatusHistory :one
INSERT INTO order_status_history (
    order_id,
    from_status,
    to_status,
    actor_type,
    actor_id,
    notes
) VALUES (
    $1, $2, $3, $4, $5, $6
) RETURNING id, order_id, from_status, to_status, actor_type, actor_id, notes, created_at;

-- name: GetOrderStatusHistoryByOrderID :many
SELECT id, order_id, from_status, to_status, actor_type, actor_id, notes, created_at
FROM order_status_history
WHERE order_id = $1
ORDER BY created_at ASC;
