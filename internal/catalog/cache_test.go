package catalog_test

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/Dhanya562004/quickcart-go/internal/catalog"
)

func TestCachedRepository_RedisOutageFallback(t *testing.T) {
	// Tests that when Redis is nil (outage), all reads and writes fall back to DB seamlessly
	fakeRepo := catalog.NewFakeRepository()
	cachedRepo := catalog.NewCachedRepository(fakeRepo, nil)

	ctx := context.Background()

	cat, err := cachedRepo.CreateCategory(ctx, catalog.CreateCategoryRequest{
		Name: "Dairy",
		Slug: "dairy",
	})
	require.NoError(t, err)

	prod, err := cachedRepo.CreateProduct(ctx, catalog.CreateProductRequest{
		CategoryID: cat.ID,
		SKU:        "DAIRY-MILK-1",
		Name:       "Full Cream Milk",
		Unit:       "500ml",
		PricePaise: 3400,
	})
	require.NoError(t, err)

	// Read with Redis down must succeed from base repository
	readProd, err := cachedRepo.GetProductByID(ctx, prod.ID)
	require.NoError(t, err)
	assert.Equal(t, "Full Cream Milk", readProd.Name)

	// Write with Redis down must succeed and update base repository
	newName := "Toned Milk 500ml"
	updated, err := cachedRepo.UpdateProduct(ctx, prod.ID, catalog.UpdateProductRequest{
		Name: &newName,
	})
	require.NoError(t, err)
	assert.Equal(t, newName, updated.Name)

	readAgain, err := cachedRepo.GetProductByID(ctx, prod.ID)
	require.NoError(t, err)
	assert.Equal(t, newName, readAgain.Name)
}

func TestCachedRepository_PassthroughMethods(t *testing.T) {
	fakeRepo := catalog.NewFakeRepository()
	cachedRepo := catalog.NewCachedRepository(fakeRepo, nil)

	ctx := context.Background()

	// ListCategories
	cats, err := cachedRepo.ListCategories(ctx)
	require.NoError(t, err)
	assert.Empty(t, cats)

	// CreateDarkStore & List
	store, err := cachedRepo.CreateDarkStore(ctx, catalog.CreateDarkStoreRequest{
		StoreCode: "STORE-1",
		Name:      "Indiranagar Hub",
	})
	require.NoError(t, err)
	assert.Equal(t, "STORE-1", store.StoreCode)

	stores, err := cachedRepo.ListDarkStores(ctx)
	require.NoError(t, err)
	assert.Len(t, stores, 1)

	// UpsertStoreInventory
	prodID := uuid.New()
	inv, err := cachedRepo.UpsertStoreInventory(ctx, store.ID, prodID, 25)
	require.NoError(t, err)
	assert.Equal(t, int32(25), inv.AvailableQuantity)
}
