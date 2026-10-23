package order

import (
	"context"
	"errors"
	"log/slog"
	"os"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/Dhanya562004/quickcart-go/internal/cart"
	"github.com/Dhanya562004/quickcart-go/internal/catalog"
	"github.com/Dhanya562004/quickcart-go/internal/config"
	"github.com/Dhanya562004/quickcart-go/internal/platform/database"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func setupRealDB(t *testing.T) (*pgxpool.Pool, catalog.Repository, Service, cart.Service) {
	t.Helper()

	dbURL := os.Getenv("DATABASE_URL")
	if dbURL == "" {
		t.Skip("skipping PostgreSQL concurrency integration test; DATABASE_URL not set")
	}

	cfg := &config.Config{
		DatabaseURL:        dbURL,
		DBMaxConns:         15,
		DBMinConns:         2,
		DBHealthCheckDelay: time.Minute,
	}
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	pool, err := database.NewPool(context.Background(), cfg, logger)
	require.NoError(t, err)
	t.Cleanup(func() { pool.Close() })

	orderRepo := NewRepository(pool.Pool)
	cartRepo := cart.NewRepository(orderRepo.(*pgxRepository).queries)
	catalogRepo := catalog.NewRepository(orderRepo.(*pgxRepository).queries)

	cartSvc := cart.NewService(cartRepo)
	orderSvc := NewService(orderRepo, cartSvc)

	return pool.Pool, catalogRepo, orderSvc, cartSvc
}

func createTestUser(t *testing.T, pool *pgxpool.Pool) uuid.UUID {
	t.Helper()
	u := uuid.New()
	email := "user-" + u.String()[:8] + "@quickcart.com"
	_, err := pool.Exec(context.Background(), `
		INSERT INTO users (id, email, password_hash, full_name, phone, role)
		VALUES ($1, $2, 'hash123', 'Test User', '9876543210', 'customer')
	`, u, email)
	require.NoError(t, err)
	return u
}

func TestRace_TwoUsersCompeteForOneItem(t *testing.T) {
	ctx := context.Background()
	pool, catRepo, orderSvc, cartSvc := setupRealDB(t)

	// 1. Seed store & product
	store, err := catRepo.CreateDarkStore(ctx, catalog.CreateDarkStoreRequest{
		StoreCode:       "RACE-STORE-" + uuid.New().String()[:8],
		Name:            "Race Dark Store",
		Address:         "100 Concurrency Blvd",
		Latitude:        12.9716,
		Longitude:       77.5946,
		MaxActiveOrders: 100,
	})
	require.NoError(t, err)

	cat, err := catRepo.CreateCategory(ctx, catalog.CreateCategoryRequest{
		Name: "Race Category " + uuid.New().String()[:8],
		Slug: "race-cat-" + uuid.New().String()[:8],
	})
	require.NoError(t, err)

	prod, err := catRepo.CreateProduct(ctx, catalog.CreateProductRequest{
		CategoryID: cat.ID,
		SKU:        "RACE-SKU-" + uuid.New().String()[:8],
		Name:       "Last Unit Item",
		Unit:       "1pc",
		PricePaise: 9900,
	})
	require.NoError(t, err)

	// Set EXACTLY 1 unit in stock
	_, err = catRepo.UpsertStoreInventory(ctx, store.ID, prod.ID, 1)
	require.NoError(t, err)

	user1 := createTestUser(t, pool)
	user2 := createTestUser(t, pool)

	// Seed carts for user 1 and user 2
	_, err = cartSvc.AddItem(ctx, user1, cart.AddItemRequest{
		StoreID:   store.ID,
		ProductID: prod.ID,
		Quantity:  1,
	})
	require.NoError(t, err)

	_, err = cartSvc.AddItem(ctx, user2, cart.AddItemRequest{
		StoreID:   store.ID,
		ProductID: prod.ID,
		Quantity:  1,
	})
	require.NoError(t, err)

	// 2. Launch concurrent checkouts
	startBarrier := make(chan struct{})
	var wg sync.WaitGroup
	wg.Add(2)

	var successCount int32
	var conflictCount int32

	users := []uuid.UUID{user1, user2}

	for i := 0; i < 2; i++ {
		go func(idx int) {
			defer wg.Done()
			u := users[idx]
			<-startBarrier // synchronize start

			_, cErr := orderSvc.Checkout(ctx, u, "idemp-race-"+u.String()[:8], CheckoutRequest{
				DeliveryAddress: "404 Race Way",
			})
			if cErr == nil {
				atomic.AddInt32(&successCount, 1)
			} else {
				var stockErr *ErrInsufficientStock
				if errors.As(cErr, &stockErr) {
					atomic.AddInt32(&conflictCount, 1)
				}
			}
		}(i)
	}

	close(startBarrier)
	wg.Wait()

	// 3. Verification: Exactly 1 winner, 1 conflict, stock never negative
	assert.Equal(t, int32(1), successCount, "exactly 1 user must succeed")
	assert.Equal(t, int32(1), conflictCount, "the loser must receive insufficient stock conflict")

	// Verify inventory state in PostgreSQL
	items, err := catRepo.ListProductsByStore(ctx, store.ID, 10, 0)
	require.NoError(t, err)
	for _, it := range items {
		if it.ID == prod.ID {
			assert.Equal(t, int32(0), it.AvailableQuantity, "available quantity must end at 0")
			assert.Equal(t, int32(1), it.ReservedQuantity, "reserved quantity must be 1")
		}
	}
}

func TestRollback_MultiProductInsufficientInventory(t *testing.T) {
	ctx := context.Background()
	pool, catRepo, orderSvc, cartSvc := setupRealDB(t)

	store, err := catRepo.CreateDarkStore(ctx, catalog.CreateDarkStoreRequest{
		StoreCode:       "MULTI-STORE-" + uuid.New().String()[:8],
		Name:            "Multi Rollback Store",
		Address:         "200 Rollback Ave",
		Latitude:        12.9716,
		Longitude:       77.5946,
		MaxActiveOrders: 100,
	})
	require.NoError(t, err)

	cat, err := catRepo.CreateCategory(ctx, catalog.CreateCategoryRequest{
		Name: "Multi Cat " + uuid.New().String()[:8],
		Slug: "multi-cat-" + uuid.New().String()[:8],
	})
	require.NoError(t, err)

	// Product A: 10 units
	prodA, err := catRepo.CreateProduct(ctx, catalog.CreateProductRequest{
		CategoryID: cat.ID,
		SKU:        "SKU-A-" + uuid.New().String()[:8],
		Name:       "Sufficient Item A",
		Unit:       "1pc",
		PricePaise: 3000,
	})
	require.NoError(t, err)
	_, err = catRepo.UpsertStoreInventory(ctx, store.ID, prodA.ID, 10)
	require.NoError(t, err)

	// Product B: 0 units
	prodB, err := catRepo.CreateProduct(ctx, catalog.CreateProductRequest{
		CategoryID: cat.ID,
		SKU:        "SKU-B-" + uuid.New().String()[:8],
		Name:       "Out Of Stock Item B",
		Unit:       "1pc",
		PricePaise: 4000,
	})
	require.NoError(t, err)
	_, err = catRepo.UpsertStoreInventory(ctx, store.ID, prodB.ID, 0)
	require.NoError(t, err)

	user := createTestUser(t, pool)
	_, err = cartSvc.AddItem(ctx, user, cart.AddItemRequest{
		StoreID:   store.ID,
		ProductID: prodA.ID,
		Quantity:  2,
	})
	require.NoError(t, err)

	_, err = cartSvc.AddItem(ctx, user, cart.AddItemRequest{
		StoreID:   store.ID,
		ProductID: prodB.ID,
		Quantity:  1,
	})
	require.NoError(t, err)

	// Attempt checkout: should fail due to Product B and completely rollback Product A
	_, err = orderSvc.Checkout(ctx, user, "idemp-rollback-001", CheckoutRequest{
		DeliveryAddress: "99 Rollback Lane",
	})
	require.Error(t, err)

	var stockErr *ErrInsufficientStock
	require.True(t, errors.As(err, &stockErr))
	assert.Equal(t, prodB.ID, stockErr.ProductID)

	// Verify Product A inventory was NOT deducted (ACID rollback check)
	items, err := catRepo.ListProductsByStore(ctx, store.ID, 10, 0)
	require.NoError(t, err)
	for _, it := range items {
		if it.ID == prodA.ID {
			assert.Equal(t, int32(10), it.AvailableQuantity, "Product A stock must remain untouched at 10")
			assert.Equal(t, int32(0), it.ReservedQuantity, "Product A reserved stock must be 0")
		}
	}
}

func TestIdempotency_ConcurrentDuplicateCheckouts(t *testing.T) {
	ctx := context.Background()
	pool, catRepo, orderSvc, cartSvc := setupRealDB(t)

	store, err := catRepo.CreateDarkStore(ctx, catalog.CreateDarkStoreRequest{
		StoreCode:       "IDEMP-STORE-" + uuid.New().String()[:8],
		Name:            "Idemp Store",
		Address:         "300 Deduplication St",
		Latitude:        12.9716,
		Longitude:       77.5946,
		MaxActiveOrders: 100,
	})
	require.NoError(t, err)

	cat, err := catRepo.CreateCategory(ctx, catalog.CreateCategoryRequest{
		Name: "Idemp Cat " + uuid.New().String()[:8],
		Slug: "idemp-cat-" + uuid.New().String()[:8],
	})
	require.NoError(t, err)

	prod, err := catRepo.CreateProduct(ctx, catalog.CreateProductRequest{
		CategoryID: cat.ID,
		SKU:        "IDEMP-SKU-" + uuid.New().String()[:8],
		Name:       "Idempotency Item",
		Unit:       "1pc",
		PricePaise: 5000,
	})
	require.NoError(t, err)

	_, err = catRepo.UpsertStoreInventory(ctx, store.ID, prod.ID, 10)
	require.NoError(t, err)

	user := createTestUser(t, pool)
	_, err = cartSvc.AddItem(ctx, user, cart.AddItemRequest{
		StoreID:   store.ID,
		ProductID: prod.ID,
		Quantity:  2,
	})
	require.NoError(t, err)

	sharedKey := "idemp-race-test-" + uuid.New().String()[:8]

	var wg sync.WaitGroup
	wg.Add(2)

	orders := make([]*Order, 2)
	errs := make([]error, 2)
	barrier := make(chan struct{})

	for i := 0; i < 2; i++ {
		go func(idx int) {
			defer wg.Done()
			<-barrier
			ord, cErr := orderSvc.Checkout(ctx, user, sharedKey, CheckoutRequest{
				DeliveryAddress: "Exact Same Address, Floor 3",
			})
			orders[idx] = ord
			errs[idx] = cErr
		}(i)
	}

	close(barrier)
	wg.Wait()

	require.NoError(t, errs[0])
	require.NoError(t, errs[1])
	require.NotNil(t, orders[0])
	require.NotNil(t, orders[1])

	// Both requests MUST return the exact same order ID and order number
	assert.Equal(t, orders[0].ID, orders[1].ID)
	assert.Equal(t, orders[0].OrderNumber, orders[1].OrderNumber)

	// Stock must have been deducted ONLY ONCE: 10 - 2 = 8 available
	items, err := catRepo.ListProductsByStore(ctx, store.ID, 10, 0)
	require.NoError(t, err)
	for _, it := range items {
		if it.ID == prod.ID {
			assert.Equal(t, int32(8), it.AvailableQuantity, "stock must only be deducted once for 2 concurrent duplicate requests")
			assert.Equal(t, int32(2), it.ReservedQuantity)
		}
	}
}

func TestCancellation_DoubleCancelAtomicity(t *testing.T) {
	ctx := context.Background()
	pool, catRepo, orderSvc, cartSvc := setupRealDB(t)

	store, err := catRepo.CreateDarkStore(ctx, catalog.CreateDarkStoreRequest{
		StoreCode:       "CANCEL-STORE-" + uuid.New().String()[:8],
		Name:            "Cancel Store",
		Address:         "400 Atomic Blvd",
		Latitude:        12.9716,
		Longitude:       77.5946,
		MaxActiveOrders: 100,
	})
	require.NoError(t, err)

	cat, err := catRepo.CreateCategory(ctx, catalog.CreateCategoryRequest{
		Name: "Cancel Cat " + uuid.New().String()[:8],
		Slug: "cancel-cat-" + uuid.New().String()[:8],
	})
	require.NoError(t, err)

	prod, err := catRepo.CreateProduct(ctx, catalog.CreateProductRequest{
		CategoryID: cat.ID,
		SKU:        "CANCEL-SKU-" + uuid.New().String()[:8],
		Name:       "Cancellable Item",
		Unit:       "1pc",
		PricePaise: 2500,
	})
	require.NoError(t, err)

	_, err = catRepo.UpsertStoreInventory(ctx, store.ID, prod.ID, 10)
	require.NoError(t, err)

	user := createTestUser(t, pool)
	_, err = cartSvc.AddItem(ctx, user, cart.AddItemRequest{
		StoreID:   store.ID,
		ProductID: prod.ID,
		Quantity:  3,
	})
	require.NoError(t, err)

	ord, err := orderSvc.Checkout(ctx, user, "idemp-cancel-"+uuid.New().String()[:8], CheckoutRequest{
		DeliveryAddress: "400 Atomic Blvd",
	})
	require.NoError(t, err)

	// Post checkout: 10 - 3 = 7 available, 3 reserved
	items, err := catRepo.ListProductsByStore(ctx, store.ID, 10, 0)
	require.NoError(t, err)
	for _, it := range items {
		if it.ID == prod.ID {
			assert.Equal(t, int32(7), it.AvailableQuantity)
			assert.Equal(t, int32(3), it.ReservedQuantity)
		}
	}

	// 2 concurrent cancel requests
	var wg sync.WaitGroup
	wg.Add(2)
	barrier := make(chan struct{})

	var cancelSuccess int32
	var cancelConflict int32

	for i := 0; i < 2; i++ {
		go func() {
			defer wg.Done()
			<-barrier
			_, cErr := orderSvc.CancelOrder(ctx, user, ord.ID, CancelOrderRequest{
				Reason: "Concurrent cancel test",
			})
			if cErr == nil {
				atomic.AddInt32(&cancelSuccess, 1)
			} else if errors.Is(cErr, ErrOrderAlreadyCancelled) {
				atomic.AddInt32(&cancelConflict, 1)
			}
		}()
	}

	close(barrier)
	wg.Wait()

	assert.Equal(t, int32(1), cancelSuccess, "exactly 1 cancellation must succeed")
	assert.Equal(t, int32(1), cancelConflict, "the second cancellation must be rejected as already cancelled")

	// Stock must be restored to 10 (7 + 3 = 10), NEVER double-restored to 13
	items, err = catRepo.ListProductsByStore(ctx, store.ID, 10, 0)
	require.NoError(t, err)
	for _, it := range items {
		if it.ID == prod.ID {
			assert.Equal(t, int32(10), it.AvailableQuantity, "stock must be restored exactly once back to 10")
			assert.Equal(t, int32(0), it.ReservedQuantity, "reserved stock must be 0")
		}
	}
}
