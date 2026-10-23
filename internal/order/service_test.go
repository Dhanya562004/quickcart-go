package order

import (
	"context"
	"testing"

	"github.com/Dhanya562004/quickcart-go/internal/cart"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func setupTestServices(t *testing.T) (Service, *FakeRepository, cart.Service) {
	t.Helper()
	orderRepo := NewFakeRepository()
	cartRepo := cart.NewFakeRepository()
	cartSvc := cart.NewService(cartRepo)
	orderSvc := NewService(orderRepo, cartSvc)
	return orderSvc, orderRepo, cartSvc
}

func TestOrderService_Checkout_Success(t *testing.T) {
	ctx := context.Background()
	orderSvc, orderRepo, cartSvc := setupTestServices(t)

	userID := uuid.New()
	storeID := uuid.New()
	prodID := uuid.New()

	// 10 units in stock
	orderRepo.SetStock(storeID, prodID, 10)

	// Add 2 units to cart (unit price 5000 paise = 50 INR, total 10000 paise)
	_, err := cartSvc.AddItem(ctx, userID, cart.AddItemRequest{
		StoreID:   storeID,
		ProductID: prodID,
		Quantity:  2,
	})
	require.NoError(t, err)

	idempKey := "idemp-checkout-001"
	ord, err := orderSvc.Checkout(ctx, userID, idempKey, CheckoutRequest{
		DeliveryAddress: "Flat 101, Palm Grove, Bengaluru",
	})
	require.NoError(t, err)
	require.NotNil(t, ord)
	assert.Equal(t, StatusPlaced, ord.Status)
	assert.Equal(t, int64(10000), ord.TotalAmountPaise)
	assert.Equal(t, "Flat 101, Palm Grove, Bengaluru", ord.DeliveryAddress)

	// Verify stock is atomically reserved: 10 - 2 = 8 available, 2 reserved
	avail, reserved := orderRepo.GetStock(storeID, prodID)
	assert.Equal(t, int32(8), avail)
	assert.Equal(t, int32(2), reserved)
}

func TestOrderService_Checkout_InsufficientStock(t *testing.T) {
	ctx := context.Background()
	orderSvc, orderRepo, cartSvc := setupTestServices(t)

	userID := uuid.New()
	storeID := uuid.New()
	prodID := uuid.New()

	// Only 1 unit in stock
	orderRepo.SetStock(storeID, prodID, 1)

	// Add 3 units to cart
	_, err := cartSvc.AddItem(ctx, userID, cart.AddItemRequest{
		StoreID:   storeID,
		ProductID: prodID,
		Quantity:  3,
	})
	require.NoError(t, err)

	_, err = orderSvc.Checkout(ctx, userID, "idemp-oversell-001", CheckoutRequest{
		DeliveryAddress: "123 Main St",
	})
	require.Error(t, err)

	var stockErr *ErrInsufficientStock
	require.True(t, assert.ErrorAs(t, err, &stockErr))
	assert.Equal(t, prodID, stockErr.ProductID)
	assert.Equal(t, int32(3), stockErr.Requested)
	assert.Equal(t, int32(1), stockErr.Available)

	// Verify inventory was untouched
	avail, reserved := orderRepo.GetStock(storeID, prodID)
	assert.Equal(t, int32(1), avail)
	assert.Equal(t, int32(0), reserved)
}

func TestOrderService_Idempotency_SameAndConflictingPayloads(t *testing.T) {
	ctx := context.Background()
	orderSvc, orderRepo, cartSvc := setupTestServices(t)

	userID := uuid.New()
	storeID := uuid.New()
	prodID := uuid.New()

	orderRepo.SetStock(storeID, prodID, 10)

	_, err := cartSvc.AddItem(ctx, userID, cart.AddItemRequest{
		StoreID:   storeID,
		ProductID: prodID,
		Quantity:  2,
	})
	require.NoError(t, err)

	idempKey := "idemp-unique-123"

	// 1. Initial checkout succeeds
	ord1, err := orderSvc.Checkout(ctx, userID, idempKey, CheckoutRequest{
		DeliveryAddress: "Flat 101, Indiranagar",
	})
	require.NoError(t, err)
	require.NotNil(t, ord1)

	// 2. Retry with exact same key and address -> MUST return original order without double reservation
	ord2, err := orderSvc.Checkout(ctx, userID, idempKey, CheckoutRequest{
		DeliveryAddress: "Flat 101, Indiranagar",
	})
	require.NoError(t, err)
	require.NotNil(t, ord2)
	assert.Equal(t, ord1.ID, ord2.ID)
	assert.Equal(t, ord1.OrderNumber, ord2.OrderNumber)

	// Verify stock deducted only once (10 - 2 = 8, reserved = 2)
	avail, reserved := orderRepo.GetStock(storeID, prodID)
	assert.Equal(t, int32(8), avail)
	assert.Equal(t, int32(2), reserved)

	// 3. Same key with DIFFERENT payload -> MUST return ErrIdempotencyPayloadMismatch (422)
	_, err = orderSvc.Checkout(ctx, userID, idempKey, CheckoutRequest{
		DeliveryAddress: "Different Address, Koramangala",
	})
	require.Error(t, err)
	assert.ErrorIs(t, err, ErrIdempotencyPayloadMismatch)
}

func TestOrderService_DoubleCancellationWithoutDoubleRestoration(t *testing.T) {
	ctx := context.Background()
	orderSvc, orderRepo, cartSvc := setupTestServices(t)

	userID := uuid.New()
	storeID := uuid.New()
	prodID := uuid.New()

	orderRepo.SetStock(storeID, prodID, 10)

	_, err := cartSvc.AddItem(ctx, userID, cart.AddItemRequest{
		StoreID:   storeID,
		ProductID: prodID,
		Quantity:  4,
	})
	require.NoError(t, err)

	ord, err := orderSvc.Checkout(ctx, userID, "idemp-cancel-001", CheckoutRequest{
		DeliveryAddress: "Sector 2, HSR Layout",
	})
	require.NoError(t, err)

	// Post checkout: 10 - 4 = 6 available, 4 reserved
	avail, reserved := orderRepo.GetStock(storeID, prodID)
	assert.Equal(t, int32(6), avail)
	assert.Equal(t, int32(4), reserved)

	// First cancellation: succeeds and restores inventory
	cancelledOrd, err := orderSvc.CancelOrder(ctx, userID, ord.ID, CancelOrderRequest{
		Reason: "Customer changed mind",
	})
	require.NoError(t, err)
	assert.Equal(t, StatusCancelled, cancelledOrd.Status)

	// Inventory restored: 6 + 4 = 10 available, 0 reserved
	avail, reserved = orderRepo.GetStock(storeID, prodID)
	assert.Equal(t, int32(10), avail)
	assert.Equal(t, int32(0), reserved)

	// Second cancellation: MUST fail with ErrOrderAlreadyCancelled and stock MUST NOT increase to 14!
	_, err = orderSvc.CancelOrder(ctx, userID, ord.ID, CancelOrderRequest{
		Reason: "Accidental double cancel click",
	})
	require.Error(t, err)
	assert.ErrorIs(t, err, ErrOrderAlreadyCancelled)

	// Inventory remains strictly 10, not 14
	avail, reserved = orderRepo.GetStock(storeID, prodID)
	assert.Equal(t, int32(10), avail)
	assert.Equal(t, int32(0), reserved)
}
