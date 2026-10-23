-- Cart Domain Queries

-- name: GetCartByUserID :one
SELECT id, user_id, store_id, created_at, updated_at
FROM carts
WHERE user_id = $1;

-- name: CreateCart :one
INSERT INTO carts (
    user_id, store_id
) VALUES (
    $1, $2
) RETURNING id, user_id, store_id, created_at, updated_at;

-- name: UpdateCartStore :one
UPDATE carts
SET 
    store_id = $2,
    updated_at = NOW()
WHERE id = $1
RETURNING id, user_id, store_id, created_at, updated_at;

-- name: GetCartItems :many
SELECT 
    ci.id,
    ci.cart_id,
    ci.product_id,
    ci.quantity,
    ci.created_at,
    ci.updated_at,
    p.name AS product_name,
    p.sku AS product_sku,
    p.unit AS product_unit,
    p.price_paise AS unit_price_paise,
    (p.price_paise * ci.quantity)::BIGINT AS total_price_paise
FROM cart_items ci
INNER JOIN products p ON ci.product_id = p.id
WHERE ci.cart_id = $1
ORDER BY ci.created_at ASC;

-- name: UpsertCartItem :one
INSERT INTO cart_items (
    cart_id, product_id, quantity
) VALUES (
    $1, $2, $3
)
ON CONFLICT (cart_id, product_id) DO UPDATE
SET 
    quantity = cart_items.quantity + EXCLUDED.quantity,
    updated_at = NOW()
RETURNING id, cart_id, product_id, quantity, created_at, updated_at;

-- name: UpdateCartItemQuantity :one
UPDATE cart_items
SET 
    quantity = $3,
    updated_at = NOW()
WHERE cart_id = $1 AND product_id = $2
RETURNING id, cart_id, product_id, quantity, created_at, updated_at;

-- name: DeleteCartItem :exec
DELETE FROM cart_items
WHERE cart_id = $1 AND product_id = $2;

-- name: ClearCart :exec
DELETE FROM cart_items
WHERE cart_id = $1;
