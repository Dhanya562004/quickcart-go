package catalog

import (
	"context"
	"time"

	"github.com/google/uuid"

	"github.com/Dhanya562004/quickcart-go/internal/platform/redis"
)

// CachedRepository decorates catalog.Repository with Redis cache-aside and invalidation.
// Architectural Invariants:
// 1. PostgreSQL is the authoritative source of truth.
// 2. Cache-aside for reads: Checks Redis first; on miss or error, falls back transparently to PostgreSQL.
// 3. Invalidation on write: Writes commit to PostgreSQL first, then invalidate associated Redis cache keys.
// 4. Redis failure resilience: Redis connection loss or timeouts never return errors to clients.
type CachedRepository struct {
	base        Repository
	redisClient *redis.Client
	productTTL  time.Duration
}

// NewCachedRepository creates a caching decorator around Repository.
func NewCachedRepository(base Repository, redisClient *redis.Client) Repository {
	return &CachedRepository{
		base:        base,
		redisClient: redisClient,
		productTTL:  1 * time.Hour,
	}
}

func (c *CachedRepository) GetProductByID(ctx context.Context, id uuid.UUID) (*Product, error) {
	key := redis.ProductKey(id)
	return redis.GetOrSet(ctx, c.redisClient, key, c.productTTL, func(ctx context.Context) (*Product, error) {
		return c.base.GetProductByID(ctx, id)
	})
}

func (c *CachedRepository) UpdateProduct(ctx context.Context, id uuid.UUID, req UpdateProductRequest) (*Product, error) {
	// 1. Write to PostgreSQL source of truth
	prod, err := c.base.UpdateProduct(ctx, id, req)
	if err != nil {
		return nil, err
	}

	// 2. Invalidate cache in Redis
	key := redis.ProductKey(id)
	_ = redis.Invalidate(ctx, c.redisClient, key)

	return prod, nil
}

func (c *CachedRepository) UpsertStoreInventory(ctx context.Context, storeID, productID uuid.UUID, quantity int32) (*StoreInventory, error) {
	// 1. Write to PostgreSQL source of truth
	inv, err := c.base.UpsertStoreInventory(ctx, storeID, productID, quantity)
	if err != nil {
		return nil, err
	}

	// 2. Invalidate inventory cache in Redis
	invKey := redis.InventoryKey(storeID, productID)
	_ = redis.Invalidate(ctx, c.redisClient, invKey)

	return inv, nil
}

// Passthrough methods directly delegating to base repository:

func (c *CachedRepository) ListCategories(ctx context.Context) ([]Category, error) {
	return c.base.ListCategories(ctx)
}

func (c *CachedRepository) GetCategoryByID(ctx context.Context, id uuid.UUID) (*Category, error) {
	return c.base.GetCategoryByID(ctx, id)
}

func (c *CachedRepository) CreateCategory(ctx context.Context, req CreateCategoryRequest) (*Category, error) {
	return c.base.CreateCategory(ctx, req)
}

func (c *CachedRepository) CreateProduct(ctx context.Context, req CreateProductRequest) (*Product, error) {
	return c.base.CreateProduct(ctx, req)
}

func (c *CachedRepository) CreateDarkStore(ctx context.Context, req CreateDarkStoreRequest) (*DarkStore, error) {
	return c.base.CreateDarkStore(ctx, req)
}

func (c *CachedRepository) GetDarkStoreByID(ctx context.Context, id uuid.UUID) (*DarkStore, error) {
	return c.base.GetDarkStoreByID(ctx, id)
}

func (c *CachedRepository) ListDarkStores(ctx context.Context) ([]DarkStore, error) {
	return c.base.ListDarkStores(ctx)
}

func (c *CachedRepository) ListProductsByStore(ctx context.Context, storeID uuid.UUID, limit, offset int32) ([]StoreProduct, error) {
	return c.base.ListProductsByStore(ctx, storeID, limit, offset)
}
