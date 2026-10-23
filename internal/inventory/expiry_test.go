package inventory_test

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/Dhanya562004/quickcart-go/internal/catalog"
	"github.com/Dhanya562004/quickcart-go/internal/config"
	"github.com/Dhanya562004/quickcart-go/internal/db"
	"github.com/Dhanya562004/quickcart-go/internal/inventory"
	"github.com/Dhanya562004/quickcart-go/internal/platform/database"
)

func setupDB(t *testing.T) *pgxpool.Pool {
	t.Helper()
	dbURL := os.Getenv("DATABASE_URL")
	if dbURL == "" {
		t.Skip("skipping PostgreSQL expiry integration test; DATABASE_URL not set")
	}

	cfg := &config.Config{
		DatabaseURL:        dbURL,
		DBMaxConns:         15,
		DBMinConns:         2,
		DBHealthCheckDelay: time.Minute,
	}

	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	dbConn, err := database.NewPool(context.Background(), cfg, logger)
	require.NoError(t, err)
	t.Cleanup(func() { dbConn.Close() })

	return dbConn.Pool
}

func TestExpiryWorker_TwoWorkersReclaimExactlyOnce(t *testing.T) {
	pool := setupDB(t)
	ctx := context.Background()
	queries := db.New(pool)
	catRepo := catalog.NewRepository(queries)

	suffix := uuid.New().String()[:8]

	// 1. Seed Dark Store, Category, and Product
	store, err := catRepo.CreateDarkStore(ctx, catalog.CreateDarkStoreRequest{
		StoreCode:       "EXP-STORE-" + suffix,
		Name:            "Expiry Dark Store",
		Address:         "123 Expiry Lane",
		Latitude:        12.9716,
		Longitude:       77.5946,
		MaxActiveOrders: 100,
	})
	require.NoError(t, err)

	cat, err := catRepo.CreateCategory(ctx, catalog.CreateCategoryRequest{
		Name: "Expiry Cat " + suffix,
		Slug: "exp-cat-" + suffix,
	})
	require.NoError(t, err)

	prod, err := catRepo.CreateProduct(ctx, catalog.CreateProductRequest{
		CategoryID: cat.ID,
		SKU:        "EXP-SKU-" + suffix,
		Name:       "Milk Carton 1L",
		Unit:       "1L",
		PricePaise: 6500,
	})
	require.NoError(t, err)

	// Seed inventory: available = 0, reserved = 10
	const initialReserved = 10
	_, err = pool.Exec(ctx, `
		INSERT INTO store_inventories (id, store_id, product_id, available_quantity, reserved_quantity, created_at, updated_at)
		VALUES (gen_random_uuid(), $1, $2, 0, $3, NOW(), NOW())
	`, store.ID, prod.ID, initialReserved)
	require.NoError(t, err)

	// Seed 10 test users, orders, and already-expired reservations (expires_at = NOW() - 2 minutes)
	orderIDs := make([]uuid.UUID, initialReserved)
	resIDs := make([]uuid.UUID, initialReserved)

	for i := 0; i < initialReserved; i++ {
		uID := uuid.New()
		_, err = pool.Exec(ctx, `
			INSERT INTO users (id, email, password_hash, full_name, phone, role)
			VALUES ($1, $2, 'hash123', 'Customer', '9876543210', 'customer')
		`, uID, fmt.Sprintf("exp-%s@test.com", uID.String()[:8]))
		require.NoError(t, err)

		ordID := uuid.New()
		orderIDs[i] = ordID
		_, err = pool.Exec(ctx, `
			INSERT INTO orders (id, user_id, store_id, order_number, status, total_amount_paise, delivery_address, created_at, updated_at)
			VALUES ($1, $2, $3, $4, 'PLACED', 6500, '123 Expiry Street', NOW(), NOW())
		`, ordID, uID, store.ID, fmt.Sprintf("ORD-EXP-%d-%s", i, suffix))
		require.NoError(t, err)

		resID := uuid.New()
		resIDs[i] = resID
		_, err = pool.Exec(ctx, `
			INSERT INTO inventory_reservations (id, order_id, store_id, product_id, quantity, status, expires_at, created_at, updated_at)
			VALUES ($1, $2, $3, $4, 1, 'RESERVED', NOW() - INTERVAL '2 minutes', NOW(), NOW())
		`, resID, ordID, store.ID, prod.ID)
		require.NoError(t, err)
	}

	// 2. Launch TWO concurrent workers executing ReclaimBatch on the same expired items
	workerCfg := inventory.ExpiryWorkerConfig{
		BatchSize: 10,
	}
	worker1 := inventory.NewExpiryWorker(pool, workerCfg, nil)
	worker2 := inventory.NewExpiryWorker(pool, workerCfg, nil)

	var wg sync.WaitGroup
	wg.Add(2)

	var res1, res2 inventory.ReclaimResult
	var err1, err2 error

	startBarrier := make(chan struct{})

	go func() {
		defer wg.Done()
		<-startBarrier
		res1, err1 = worker1.ReclaimBatch(ctx)
	}()

	go func() {
		defer wg.Done()
		<-startBarrier
		res2, err2 = worker2.ReclaimBatch(ctx)
	}()

	close(startBarrier)
	wg.Wait()

	require.NoError(t, err1)
	require.NoError(t, err2)

	// Invariant 1: Total reclaimed count between both workers must be EXACTLY 10
	totalReclaimed := res1.ReclaimedCount + res2.ReclaimedCount
	assert.Equal(t, initialReserved, totalReclaimed, "two workers together must reclaim each reservation exactly once")

	// Invariant 2: Inventory restored to available_quantity = 10, reserved_quantity = 0 (No double-restoration)
	var availableQty, reservedQty int32
	err = pool.QueryRow(ctx, `
		SELECT available_quantity, reserved_quantity
		FROM store_inventories
		WHERE store_id = $1 AND product_id = $2
	`, store.ID, prod.ID).Scan(&availableQty, &reservedQty)
	require.NoError(t, err)

	assert.Equal(t, int32(initialReserved), availableQty, "available inventory must be restored to exactly 10")
	assert.Equal(t, int32(0), reservedQty, "reserved inventory must be restored to exactly 0")

	// Invariant 3: All 10 reservations must now be in EXPIRED status
	var expiredCount int
	err = pool.QueryRow(ctx, `
		SELECT COUNT(*)
		FROM inventory_reservations
		WHERE store_id = $1 AND product_id = $2 AND status = 'EXPIRED'
	`, store.ID, prod.ID).Scan(&expiredCount)
	require.NoError(t, err)
	assert.Equal(t, initialReserved, expiredCount, "all 10 reservations must have status EXPIRED")

	// Invariant 4: All 10 orders must have status CANCELLED
	var cancelledCount int
	err = pool.QueryRow(ctx, `
		SELECT COUNT(*)
		FROM orders
		WHERE store_id = $1 AND status = 'CANCELLED'
	`, store.ID).Scan(&cancelledCount)
	require.NoError(t, err)
	assert.Equal(t, initialReserved, cancelledCount, "all 10 orders must have status CANCELLED")
}
