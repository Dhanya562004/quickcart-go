package order

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/Dhanya562004/quickcart-go/internal/cart"
	"github.com/Dhanya562004/quickcart-go/internal/catalog"
	"github.com/Dhanya562004/quickcart-go/internal/config"
	"github.com/Dhanya562004/quickcart-go/internal/platform/database"
)

// TestHeadline_1000UsersReserve10Units proves zero-overselling inventory under extreme concurrent contention.
//
// Scenario:
// A dark store holds EXACTLY 10 units of an item.
// 1,000 distinct customer goroutines attempt to check out 1 unit each simultaneously.
//
// Concurrency note:
// The startBarrier unblocks all 1,000 goroutines to submit checkout transactions.
// While PostgreSQL serializes and queues transactions across the bounded connection pool (maxConns = 25),
// this validates zero-overselling without deadlocks, phantom reads, or orphaned reservations.
//
// Strict assertions:
// 1. Exactly 10 checkouts succeed (201 Created / Order Placed).
// 2. Exactly 990 checkouts receive ErrInsufficientInventory (HTTP 409 Conflict).
// 3. Final available stock is exactly 0 (never negative).
// 4. Final reserved stock is exactly 10.
// 5. Exactly 10 reservations exist, all linked to valid successful orders (no orphaned reservations).
// 6. Zero unexpected database errors (connection starvation, deadlocks, or timeouts).
func TestHeadline_1000UsersReserve10Units(t *testing.T) {
	dbURL := os.Getenv("DATABASE_URL")
	if dbURL == "" {
		t.Skip("skipping PostgreSQL headline race test; DATABASE_URL not set")
	}

	// Bounded connection pool (25 max connections) with realistic timeout
	cfg := &config.Config{
		DatabaseURL:        dbURL,
		DBMaxConns:         25,
		DBMinConns:         5,
		DBHealthCheckDelay: time.Minute,
	}

	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	dbPool, err := database.NewPool(context.Background(), cfg, logger)
	require.NoError(t, err)
	t.Cleanup(func() { dbPool.Close() })

	pool := dbPool.Pool
	orderRepo := NewRepository(pool)
	cartRepo := cart.NewRepository(orderRepo.(*pgxRepository).queries)
	catalogRepo := catalog.NewRepository(orderRepo.(*pgxRepository).queries)

	cartSvc := cart.NewService(cartRepo)
	orderSvc := NewService(orderRepo, cartSvc)

	// Realistic test timeout to allow 1,000 transactions to queue through 25 connections
	ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
	defer cancel()

	// 1. Seed Dark Store, Category, and Product with EXACTLY 10 units available
	suffix := uuid.New().String()[:8]
	store, err := catalogRepo.CreateDarkStore(ctx, catalog.CreateDarkStoreRequest{
		StoreCode:       "HEADLINE-STORE-" + suffix,
		Name:            "Headline Concurrency Store",
		Address:         "1000 Contention Way, Bengaluru",
		Latitude:        12.9716,
		Longitude:       77.5946,
		MaxActiveOrders: 2000,
	})
	require.NoError(t, err)

	cat, err := catalogRepo.CreateCategory(ctx, catalog.CreateCategoryRequest{
		Name: "Headline Cat " + suffix,
		Slug: "headline-cat-" + suffix,
	})
	require.NoError(t, err)

	prod, err := catalogRepo.CreateProduct(ctx, catalog.CreateProductRequest{
		CategoryID: cat.ID,
		SKU:        "HEADLINE-SKU-" + suffix,
		Name:       "Flash Sale Mangoes",
		Unit:       "1kg",
		PricePaise: 14900,
	})
	require.NoError(t, err)

	// Seed exactly 10 units in stock
	const initialStock = 10
	_, err = catalogRepo.UpsertStoreInventory(ctx, store.ID, prod.ID, initialStock)
	require.NoError(t, err)

	// 2. Efficiently seed 1,000 distinct customers with 1 unit in their carts
	const customerCount = 1000
	userIDs := make([]uuid.UUID, customerCount)
	for i := 0; i < customerCount; i++ {
		userIDs[i] = uuid.New()
	}

	seedBatchCustomersAndCarts(t, pool, userIDs, store.ID, prod.ID)

	// 3. Execute 1,000 concurrent checkout transactions
	startBarrier := make(chan struct{})
	var wg sync.WaitGroup
	wg.Add(customerCount)

	var successCount atomic.Int32
	var conflictCount atomic.Int32

	var mu sync.Mutex
	successfulOrders := make(map[uuid.UUID]bool)
	unexpectedErrors := make([]error, 0)

	for i := 0; i < customerCount; i++ {
		go func(u uuid.UUID) {
			defer wg.Done()

			// Block until all 1,000 goroutines are spawned
			<-startBarrier

			idempKey := fmt.Sprintf("idemp-headline-%s", u.String())
			ord, cErr := orderSvc.Checkout(ctx, u, idempKey, CheckoutRequest{
				DeliveryAddress: "Flat 101, Flash Sale Residency, Bengaluru",
			})
			var stockErr *ErrInsufficientStock
			if cErr == nil {
				successCount.Add(1)
				mu.Lock()
				successfulOrders[ord.ID] = true
				mu.Unlock()
			} else if errors.As(cErr, &stockErr) {
				conflictCount.Add(1)
			} else {
				mu.Lock()
				unexpectedErrors = append(unexpectedErrors, fmt.Errorf("user %s checkout unexpected error: %w", u, cErr))
				mu.Unlock()
			}
		}(userIDs[i])
	}

	// Release all 1,000 customer goroutines simultaneously
	close(startBarrier)
	wg.Wait()

	// 4. Verification and Assertions
	require.Empty(t, unexpectedErrors, "unexpected database or connection errors occurred")
	assert.Equal(t, int32(initialStock), successCount.Load(), "strictly exactly 10 orders must succeed")
	assert.Equal(t, int32(customerCount-initialStock), conflictCount.Load(), "strictly exactly 990 orders must receive 409 ErrInsufficientInventory")

	// 5. Database State Invariant Checks
	var availableQty, reservedQty int32
	err = pool.QueryRow(ctx, `
		SELECT available_quantity, reserved_quantity
		FROM store_inventories
		WHERE store_id = $1 AND product_id = $2
	`, store.ID, prod.ID).Scan(&availableQty, &reservedQty)
	require.NoError(t, err)

	assert.Equal(t, int32(0), availableQty, "available inventory must end at exactly 0; never negative")
	assert.Equal(t, int32(initialStock), reservedQty, "reserved inventory must end at exactly 10")

	// 6. Verify No Orphaned Reservations
	var reservationCount int
	err = pool.QueryRow(ctx, `
		SELECT COUNT(*)
		FROM inventory_reservations
		WHERE store_id = $1 AND product_id = $2 AND status = 'RESERVED'
	`, store.ID, prod.ID).Scan(&reservationCount)
	require.NoError(t, err)
	assert.Equal(t, initialStock, reservationCount, "exactly 10 active reservations must exist")

	rows, err := pool.Query(ctx, `
		SELECT order_id
		FROM inventory_reservations
		WHERE store_id = $1 AND product_id = $2 AND status = 'RESERVED'
	`, store.ID, prod.ID)
	require.NoError(t, err)
	defer rows.Close()

	for rows.Next() {
		var resOrderID uuid.UUID
		require.NoError(t, rows.Scan(&resOrderID))
		mu.Lock()
		isMatched := successfulOrders[resOrderID]
		mu.Unlock()
		assert.True(t, isMatched, "every reservation must correspond to a valid successful order (no orphans)")
	}

	// 7. Verify exactly 10 orders exist in orders table
	var dbOrderCount int
	err = pool.QueryRow(ctx, `
		SELECT COUNT(*)
		FROM orders
		WHERE store_id = $1 AND status = 'PLACED'
	`, store.ID).Scan(&dbOrderCount)
	require.NoError(t, err)
	assert.Equal(t, initialStock, dbOrderCount, "exactly 10 orders must be recorded in orders table")
}

// seedBatchCustomersAndCarts performs bulk SQL insertion in chunks of 250 for speed and clean setup.
func seedBatchCustomersAndCarts(t *testing.T, pool *pgxpool.Pool, userIDs []uuid.UUID, storeID, prodID uuid.UUID) {
	t.Helper()
	ctx := context.Background()
	const chunkSize = 250

	for i := 0; i < len(userIDs); i += chunkSize {
		end := i + chunkSize
		if end > len(userIDs) {
			end = len(userIDs)
		}
		chunk := userIDs[i:end]

		// 1. Bulk insert users
		var userVals []string
		var userArgs []any
		for j, u := range chunk {
			p := j * 2
			userVals = append(userVals, fmt.Sprintf("($%d, $%d, 'hash123', 'Customer', '9876543210', 'customer')", p+1, p+2))
			userArgs = append(userArgs, u, fmt.Sprintf("cust-%s@test.com", u.String()[:8]))
		}
		userQuery := fmt.Sprintf("INSERT INTO users (id, email, password_hash, full_name, phone, role) VALUES %s ON CONFLICT DO NOTHING", strings.Join(userVals, ","))
		_, err := pool.Exec(ctx, userQuery, userArgs...)
		require.NoError(t, err)

		// 2. Bulk insert carts
		var cartVals []string
		var cartArgs []any
		cartIDs := make([]uuid.UUID, len(chunk))
		for j, u := range chunk {
			cID := uuid.New()
			cartIDs[j] = cID
			p := j * 3
			cartVals = append(cartVals, fmt.Sprintf("($%d, $%d, $%d, NOW(), NOW())", p+1, p+2, p+3))
			cartArgs = append(cartArgs, cID, u, storeID)
		}
		cartQuery := fmt.Sprintf("INSERT INTO carts (id, user_id, store_id, created_at, updated_at) VALUES %s", strings.Join(cartVals, ","))
		_, err = pool.Exec(ctx, cartQuery, cartArgs...)
		require.NoError(t, err)

		// 3. Bulk insert cart items
		var itemVals []string
		var itemArgs []any
		for j, cID := range cartIDs {
			p := j * 2
			itemVals = append(itemVals, fmt.Sprintf("(gen_random_uuid(), $%d, $%d, 1, NOW(), NOW())", p+1, p+2))
			itemArgs = append(itemArgs, cID, prodID)
		}
		itemQuery := fmt.Sprintf("INSERT INTO cart_items (id, cart_id, product_id, quantity, created_at, updated_at) VALUES %s", strings.Join(itemVals, ","))
		_, err = pool.Exec(ctx, itemQuery, itemArgs...)
		require.NoError(t, err)
	}
}
