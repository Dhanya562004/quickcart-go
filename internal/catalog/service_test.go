package catalog

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCatalogService_CreateProduct(t *testing.T) {
	ctx := context.Background()
	repo := NewFakeRepository()
	svc := NewService(repo)

	// Seed category
	cat, err := svc.CreateCategory(ctx, CreateCategoryRequest{
		Name: "Dairy & Breakfast",
		Slug: "dairy-breakfast",
	})
	require.NoError(t, err)

	tests := []struct {
		name        string
		req         CreateProductRequest
		expectError bool
		expectedErr error
	}{
		{
			name: "valid product",
			req: CreateProductRequest{
				CategoryID: cat.ID,
				SKU:        "MILK-TONED-1L",
				Name:       "Toned Fresh Milk 1L",
				Unit:       "1L",
				PricePaise: 7400,
			},
			expectError: false,
		},
		{
			name: "negative price rejected",
			req: CreateProductRequest{
				CategoryID: cat.ID,
				SKU:        "MILK-INVALID-PRICE",
				Name:       "Invalid Milk",
				Unit:       "1L",
				PricePaise: -500,
			},
			expectError: true,
			expectedErr: ErrInvalidPrice,
		},
		{
			name: "unknown category rejected",
			req: CreateProductRequest{
				CategoryID: uuid.New(),
				SKU:        "ORPHAN-ITEM",
				Name:       "Orphan Item",
				Unit:       "1pc",
				PricePaise: 1000,
			},
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			prod, err := svc.CreateProduct(ctx, tt.req)
			if tt.expectError {
				require.Error(t, err)
				if tt.expectedErr != nil {
					assert.ErrorIs(t, err, tt.expectedErr)
				}
				assert.Nil(t, prod)
			} else {
				require.NoError(t, err)
				assert.NotNil(t, prod)
				assert.Equal(t, tt.req.Name, prod.Name)
				assert.Equal(t, tt.req.PricePaise, prod.PricePaise)
			}
		})
	}
}

func TestCatalogService_DarkStoreAndInventory(t *testing.T) {
	ctx := context.Background()
	repo := NewFakeRepository()
	svc := NewService(repo)

	cat, err := svc.CreateCategory(ctx, CreateCategoryRequest{Name: "Beverages", Slug: "beverages"})
	require.NoError(t, err)

	prod, err := svc.CreateProduct(ctx, CreateProductRequest{
		CategoryID: cat.ID,
		SKU:        "COFFEE-COLD-200ML",
		Name:       "Cold Coffee Can 200ml",
		Unit:       "200ml",
		PricePaise: 4500,
	})
	require.NoError(t, err)

	active := true
	store, err := svc.CreateDarkStore(ctx, CreateDarkStoreRequest{
		StoreCode:       "BLR-KOR-01",
		Name:            "Koramangala Pod",
		Address:         "80 Feet Rd, Koramangala 4th Block",
		Latitude:        12.9352,
		Longitude:       77.6245,
		IsActive:        &active,
		MaxActiveOrders: 150,
	})
	require.NoError(t, err)

	t.Run("upsert inventory valid and list", func(t *testing.T) {
		inv, err := svc.UpsertStoreInventory(ctx, store.ID, prod.ID, 50)
		require.NoError(t, err)
		assert.Equal(t, int32(50), inv.AvailableQuantity)

		items, err := svc.ListProductsByStore(ctx, store.ID, 20, 0)
		require.NoError(t, err)
		require.Len(t, items, 1)
		assert.Equal(t, prod.ID, items[0].ID)
		assert.Equal(t, int32(50), items[0].AvailableQuantity)
	})

	t.Run("negative inventory rejected", func(t *testing.T) {
		_, err := svc.UpsertStoreInventory(ctx, store.ID, prod.ID, -10)
		require.ErrorIs(t, err, ErrInvalidQuantity)
	})

	t.Run("inactive store returns error on listing", func(t *testing.T) {
		inactive := false
		storeInactive, err := svc.CreateDarkStore(ctx, CreateDarkStoreRequest{
			StoreCode:       "BLR-INACTIVE",
			Name:            "Closed Store",
			Address:         "Whitefield",
			Latitude:        12.9698,
			Longitude:       77.7500,
			IsActive:        &inactive,
			MaxActiveOrders: 50,
		})
		require.NoError(t, err)

		_, err = svc.ListProductsByStore(ctx, storeInactive.ID, 20, 0)
		require.ErrorIs(t, err, ErrStoreInactive)
	})
}
