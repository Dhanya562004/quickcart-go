package cart

import (
	"context"
	"sync"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCartService_SingleStoreBoundary(t *testing.T) {
	ctx := context.Background()
	repo := NewFakeRepository()
	svc := NewService(repo)

	userID := uuid.New()
	storeA := uuid.New()
	storeB := uuid.New()
	prod1 := uuid.New()
	prod2 := uuid.New()

	// 1. Add item from store A
	cartA, err := svc.AddItem(ctx, userID, AddItemRequest{
		StoreID:   storeA,
		ProductID: prod1,
		Quantity:  2,
	})
	require.NoError(t, err)
	require.NotNil(t, cartA.StoreID)
	assert.Equal(t, storeA, *cartA.StoreID)
	assert.Len(t, cartA.Items, 1)

	// 2. Attempt to add item from store B -> MUST return ErrCrossStoreConflict (409)
	_, err = svc.AddItem(ctx, userID, AddItemRequest{
		StoreID:   storeB,
		ProductID: prod2,
		Quantity:  1,
	})
	require.Error(t, err)
	assert.ErrorIs(t, err, ErrCrossStoreConflict)

	// 3. Clear cart
	err = svc.ClearCart(ctx, userID)
	require.NoError(t, err)

	// 4. Now adding item from store B MUST succeed because cart was cleared
	cartB, err := svc.AddItem(ctx, userID, AddItemRequest{
		StoreID:   storeB,
		ProductID: prod2,
		Quantity:  3,
	})
	require.NoError(t, err)
	require.NotNil(t, cartB.StoreID)
	assert.Equal(t, storeB, *cartB.StoreID)
	assert.Len(t, cartB.Items, 1)
	assert.Equal(t, int32(3), cartB.Items[0].Quantity)
}

func TestCartService_QuantityAndRemoval(t *testing.T) {
	ctx := context.Background()
	repo := NewFakeRepository()
	svc := NewService(repo)

	userID := uuid.New()
	storeID := uuid.New()
	prodID := uuid.New()

	_, err := svc.AddItem(ctx, userID, AddItemRequest{
		StoreID:   storeID,
		ProductID: prodID,
		Quantity:  5,
	})
	require.NoError(t, err)

	// Update quantity to 10
	cart, err := svc.UpdateItemQuantity(ctx, userID, prodID, 10)
	require.NoError(t, err)
	assert.Equal(t, int32(10), cart.Items[0].Quantity)

	// Update quantity to 0 removes line item and resets store
	cart, err = svc.UpdateItemQuantity(ctx, userID, prodID, 0)
	require.NoError(t, err)
	assert.Empty(t, cart.Items)
	assert.Nil(t, cart.StoreID)
}

func TestCartService_ConcurrentCartModifications(t *testing.T) {
	ctx := context.Background()
	repo := NewFakeRepository()
	svc := NewService(repo)

	userID := uuid.New()
	storeID := uuid.New()

	const numGoroutines = 20
	var wg sync.WaitGroup
	wg.Add(numGoroutines)

	// Concurrently add items to the same user cart
	for i := 0; i < numGoroutines; i++ {
		go func(idx int) {
			defer wg.Done()
			prodID := uuid.New()
			_, err := svc.AddItem(ctx, userID, AddItemRequest{
				StoreID:   storeID,
				ProductID: prodID,
				Quantity:  1,
			})
			assert.NoError(t, err)
		}(i)
	}
	wg.Wait()

	cart, err := svc.GetCart(ctx, userID)
	require.NoError(t, err)
	assert.Equal(t, numGoroutines, len(cart.Items))
	assert.Equal(t, int64(numGoroutines*5000), cart.TotalAmountPaise)
}
