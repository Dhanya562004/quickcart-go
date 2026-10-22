-- Core Inventory and Reservation Queries (Zero-Overselling Engine)

-- name: GetStoreInventory :one
SELECT store_id, product_id, available_quantity, reserved_quantity, version, created_at, updated_at
FROM store_inventories
WHERE store_id = $1 AND product_id = $2;

-- name: ReserveInventoryAtomic :one
-- Atomically decrements available_quantity and increments reserved_quantity.
-- Condition available_quantity >= $3 prevents overselling at database tuple level.
UPDATE store_inventories
SET 
    available_quantity = available_quantity - $3,
    reserved_quantity = reserved_quantity + $3,
    version = version + 1,
    updated_at = NOW()
WHERE 
    store_id = $1 
    AND product_id = $2 
    AND available_quantity >= $3
RETURNING store_id, product_id, available_quantity, reserved_quantity, version, updated_at;

-- name: CreateInventoryReservation :one
INSERT INTO inventory_reservations (
    order_id,
    store_id,
    product_id,
    quantity,
    status,
    expires_at
) VALUES (
    $1, $2, $3, $4, 'RESERVED', $5
) RETURNING id, order_id, store_id, product_id, quantity, status, expires_at, created_at, updated_at;

-- name: ConfirmReservation :one
-- Transition from RESERVED -> CONFIRMED guarded by status check
UPDATE inventory_reservations
SET 
    status = 'CONFIRMED',
    updated_at = NOW()
WHERE 
    id = $1 
    AND status = 'RESERVED'
RETURNING id, order_id, store_id, product_id, quantity, status, updated_at;

-- name: ReleaseReservation :one
-- Transition from RESERVED -> RELEASED guarded by status check
UPDATE inventory_reservations
SET 
    status = 'RELEASED',
    updated_at = NOW()
WHERE 
    id = $1 
    AND status = 'RESERVED'
RETURNING id, order_id, store_id, product_id, quantity, status, updated_at;

-- name: RestoreInventoryAtomic :one
-- Releases reservation back to available stock
UPDATE store_inventories
SET 
    available_quantity = available_quantity + $3,
    reserved_quantity = reserved_quantity - $3,
    version = version + 1,
    updated_at = NOW()
WHERE 
    store_id = $1 
    AND product_id = $2 
    AND reserved_quantity >= $3
RETURNING store_id, product_id, available_quantity, reserved_quantity, version, updated_at;

-- name: DeductReservedInventoryAtomic :one
-- Finalizes stock consumption upon order fulfillment
UPDATE store_inventories
SET 
    reserved_quantity = reserved_quantity - $3,
    version = version + 1,
    updated_at = NOW()
WHERE 
    store_id = $1 
    AND product_id = $2 
    AND reserved_quantity >= $3
RETURNING store_id, product_id, available_quantity, reserved_quantity, version, updated_at;

-- name: PickExpiredReservationsForUpdate :many
-- Expiry worker claims up to $1 expired reservations safely using FOR UPDATE SKIP LOCKED
SELECT id, order_id, store_id, product_id, quantity, status, expires_at
FROM inventory_reservations
WHERE 
    status = 'RESERVED' 
    AND expires_at <= NOW()
ORDER BY expires_at ASC
LIMIT $1
FOR UPDATE SKIP LOCKED;

-- name: MarkReservationExpired :one
-- Transition from RESERVED -> EXPIRED guarded by status check
UPDATE inventory_reservations
SET 
    status = 'EXPIRED',
    updated_at = NOW()
WHERE 
    id = $1 
    AND status = 'RESERVED'
RETURNING id, order_id, store_id, product_id, quantity, status, updated_at;
