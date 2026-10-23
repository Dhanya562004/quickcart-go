-- Catalog Domain Queries

-- name: ListCategories :many
SELECT id, name, slug, description, created_at
FROM categories
ORDER BY name ASC;

-- name: GetCategoryByID :one
SELECT id, name, slug, description, created_at
FROM categories
WHERE id = $1;

-- name: CreateCategory :one
INSERT INTO categories (
    name, slug, description
) VALUES (
    $1, $2, $3
) RETURNING id, name, slug, description, created_at;

-- name: CreateProduct :one
INSERT INTO products (
    category_id, sku, name, description, unit, price_paise, is_active
) VALUES (
    $1, $2, $3, $4, $5, $6, $7
) RETURNING id, category_id, sku, name, description, unit, price_paise, is_active, created_at, updated_at;

-- name: GetProductByID :one
SELECT id, category_id, sku, name, description, unit, price_paise, is_active, created_at, updated_at
FROM products
WHERE id = $1;

-- name: UpdateProduct :one
UPDATE products
SET 
    category_id = COALESCE($2, category_id),
    name = COALESCE($3, name),
    description = COALESCE($4, description),
    unit = COALESCE($5, unit),
    price_paise = COALESCE($6, price_paise),
    is_active = COALESCE($7, is_active),
    updated_at = NOW()
WHERE id = $1
RETURNING id, category_id, sku, name, description, unit, price_paise, is_active, created_at, updated_at;

-- name: CreateDarkStore :one
INSERT INTO dark_stores (
    store_code, name, address, latitude, longitude, is_active, max_active_orders
) VALUES (
    $1, $2, $3, $4, $5, $6, $7
) RETURNING id, store_code, name, address, latitude, longitude, is_active, max_active_orders, created_at, updated_at;

-- name: GetDarkStoreByID :one
SELECT id, store_code, name, address, latitude, longitude, is_active, max_active_orders, created_at, updated_at
FROM dark_stores
WHERE id = $1;

-- name: ListDarkStores :many
SELECT id, store_code, name, address, latitude, longitude, is_active, max_active_orders, created_at, updated_at
FROM dark_stores
WHERE is_active = true
ORDER BY name ASC;

-- name: ListProductsByStore :many
SELECT 
    p.id AS product_id,
    p.category_id,
    p.sku,
    p.name,
    p.description,
    p.unit,
    p.price_paise,
    p.is_active,
    COALESCE(si.available_quantity, 0)::INT AS available_quantity,
    COALESCE(si.reserved_quantity, 0)::INT AS reserved_quantity,
    si.store_id
FROM products p
INNER JOIN store_inventories si ON p.id = si.product_id
WHERE si.store_id = $1 AND p.is_active = true
ORDER BY p.name ASC
LIMIT $2 OFFSET $3;

-- name: UpsertStoreInventory :one
INSERT INTO store_inventories (
    store_id, product_id, available_quantity, reserved_quantity, version
) VALUES (
    $1, $2, $3, 0, 0
)
ON CONFLICT (store_id, product_id) DO UPDATE
SET 
    available_quantity = EXCLUDED.available_quantity,
    updated_at = NOW()
RETURNING store_id, product_id, available_quantity, reserved_quantity, version, created_at, updated_at;
