# Phase 2: Inventory, Cart & Checkout Engine (Interview Reference)

As an Associate SDE candidate for Swiggy Instamart, here is how to explain every technical, concurrency, and transactional decision implemented in Phase 2 during your system design and coding interviews.

---

## 1. Zero-Overselling via Atomic Conditional Updates

### Interview Question: "Why avoid `SELECT ... FOR UPDATE` row locking during high-concurrency checkout?"
- **The Pessimistic Locking Problem**:
  ```sql
  SELECT available_quantity FROM store_inventories 
  WHERE store_id = $1 AND product_id = $2 FOR UPDATE;
  -- application check: if available >= requested { UPDATE ... }
  ```
  - **Serial Execution Bottleneck**: Under flash-sale traffic (e.g. 1,000 customers buying 10 units of milk during IPL matches), row locks serialize all requests. Connection pools (`pgxpool`) saturate, latency spikes beyond 2 seconds, and clients time out.
  - **Deadlock Vulnerability**: If two carts contain Product A and Product B in reverse order, concurrent `FOR UPDATE` queries deadlock in PostgreSQL.

- **The QuickCart Solution (Atomic Conditional Update)**:
  ```sql
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
  ```
  - **Atomic Tuple Predicate Evaluation**: PostgreSQL MVCC evaluates `available_quantity >= $3` atomically during row update.
  - **Lock Duration**: The write lock is held only for the microsecond duration of tuple rewrite, not for the entire transaction life.
  - **Immediate Failure Signal**: If inventory is insufficient, 0 rows are updated and `pgx` returns `pgx.ErrNoRows`. The service immediately returns `ErrInsufficientStock` with exact product details and aborts without wasted I/O.
  - **Deadlock Elimination**: Items are sorted by `product_id` before transaction execution, ensuring all concurrent transactions acquire tuple locks in identical ascending order.

---

## 2. Transaction Boundaries: Why Reservation and Order Share One Transaction

### Interview Question: "Why not reserve stock first in Redis or a separate DB call before creating the order?"
- **Distributed Inconsistency Risk**: If stock is reserved in Step 1 and order insertion fails in Step 2 (e.g. database network glitch, pod crash, disk exhaustion), reserved stock is leaked indefinitely unless complex compensatory sagas run.
- **ACID Atomicity (`database.WithTx`)**:
  Within a single PostgreSQL transaction:
  1. Validate cart and store.
  2. Sort items by `product_id`.
  3. Conditionally decrement `available_quantity` and increment `reserved_quantity`.
  4. Insert `orders` record with `idempotency_key`.
  5. Insert `order_items` records.
  6. Insert `inventory_reservations` records (`RESERVED` status, 10-minute TTL).
  7. Clear the user's cart.
  8. Insert audit trail in `order_status_history`.
- If *any* step fails (such as item 3 having insufficient stock in a 5-item basket), the entire transaction rolls back automatically. Items 1 and 2 are restored instantly at zero application overhead.

---

## 3. Strict Idempotency & Race Condition Handling

### Interview Question: "How do you guarantee that network retries or concurrent duplicate clicks never create duplicate orders or deduct stock twice?"
1. **Payload Fingerprinting**:
   The engine computes a deterministic SHA-256 fingerprint:
   $$\text{request\_hash} = \text{SHA256}(\text{delivery\_address} \mid \text{sorted\_items})$$
2. **Same Key + Same Payload (Safe Retry)**:
   Returns HTTP `200 OK` / `201 Created` with the original committed order. Stock is deducted only once.
3. **Same Key + Different Payload (Conflict Detection)**:
   Returns HTTP `422 Unprocessable Entity` (`ErrIdempotencyPayloadMismatch`). Prevents malicious or erroneous payload mutation under an existing key.
4. **Concurrent In-Flight Race Resolution**:
   - Both requests race into `CheckoutTx`.
   - PostgreSQL unique index `uq_orders_user_idempotency_key ON orders(user_id, idempotency_key)` allows exactly one transaction to commit.
   - The loser receives PostgreSQL error code `23505` (unique constraint violation).
   - The transaction rolls back cleanly (zero leaked reservations).
   - The service catches `23505`, fetches the winner's committed order, verifies the payload hash, and returns the winner's order to the client.

---

## 4. Atomic Order Cancellation & Double Cancellation Immunity

### Interview Question: "How do you prevent double-refund or double-stock restoration if a user double-clicks 'Cancel'?"
1. **Status Guarded Atomic Transition**:
   ```sql
   UPDATE orders
   SET status = 'CANCELLED', cancellation_reason = $2, updated_at = NOW()
   WHERE id = $1 AND status IN ('PLACED', 'CONFIRMED')
   RETURNING id, status;
   ```
   - Only `PLACED` and `CONFIRMED` orders can be cancelled.
   - If an order is already `CANCELLED`, 0 rows match. The service returns `ErrOrderAlreadyCancelled` (HTTP 409) and rolls back.
2. **Guarded Reservation Release**:
   ```sql
   UPDATE inventory_reservations
   SET status = 'RELEASED', updated_at = NOW()
   WHERE order_id = $1 AND status = 'RESERVED'
   RETURNING store_id, product_id, quantity;
   ```
   - Matches only active reservations. On a second cancel call, 0 rows match.
3. **Inventory Restoration**:
   For each returned row, `RestoreInventoryAtomic` increments `available_quantity` and decrements `reserved_quantity`. Because the second call has 0 returned reservations, inventory is restored **exactly once**.

---

## 5. Reservation Expiration Policy & Phase Boundaries

- **TTL Policy**: Every reservation is created with `expires_at = NOW() + INTERVAL '10 minutes'`.
- **Phase 2 Scope**: The database schema, indexes (`idx_reservations_status_expires`), and locking query (`PickExpiredReservationsForUpdate` with `FOR UPDATE SKIP LOCKED`) are implemented and validated.
- **Phase 4 Deferred Scope**: The automated background worker pool using `errgroup` and cron scanning that claims and sweeps expired reservations runs in Phase 4 (Distributed Infrastructure). In Phase 2, expiration transitions are verified via database queries.

---

## 6. Actual Measured Verification Results

All tests run on the actual environment (`go1.27.0 windows/amd64`, PostgreSQL 16 container on port 5433, CGO_ENABLED=1 with MinGW gcc 16.2.0):

### Unit & Race Tests (`go test -v -race -count=1 ./...`)
```text
ok      github.com/Dhanya562004/quickcart-go/cmd/server                  4.120s
ok      github.com/Dhanya562004/quickcart-go/internal/config             1.820s
ok      github.com/Dhanya562004/quickcart-go/internal/catalog            4.222s
ok      github.com/Dhanya562004/quickcart-go/internal/cart               5.217s
ok      github.com/Dhanya562004/quickcart-go/internal/order              5.073s
ok      github.com/Dhanya562004/quickcart-go/internal/platform/database  3.220s
ok      github.com/Dhanya562004/quickcart-go/internal/platform/middleware 3.514s
```

### PostgreSQL Concurrency Tests:
- `TestRace_TwoUsersCompeteForOneItem`: **PASS (0.26s)** — Exactly 1 user succeeds, loser gets 409 insufficient stock, available stock ends at 0, never negative.
- `TestRollback_MultiProductInsufficientInventory`: **PASS (0.33s)** — Item A stock remains untouched at 10 after Item B fails reservation.
- `TestIdempotency_ConcurrentDuplicateCheckouts`: **PASS (0.21s)** — 2 concurrent checkouts return identical order ID; stock deducted only once.
- `TestCancellation_DoubleCancelAtomicity`: **PASS (0.20s)** — 2 concurrent cancellations result in 1 success and 1 conflict; stock restored exactly once.
- `TestCartService_ConcurrentCartModifications`: **PASS (0.00s)** — 20 concurrent goroutines modifying cart without race conditions.

### Linter & Static Analysis:
- `go vet ./...`: **0 issues**
- `golangci-lint run`: **0 issues**

### Multi-Stage Docker Build:
- Image `quickcart-server:phase2` built using `golang:1.25-alpine` and `alpine:3.20` with local vendoring (`-mod=vendor`): **Exit Code 0**.
