package catalog

import (
	"context"
	"sync"
	"time"

	"github.com/google/uuid"
)

type FakeRepository struct {
	mu          sync.RWMutex
	Categories  map[uuid.UUID]Category
	Products    map[uuid.UUID]Product
	DarkStores  map[uuid.UUID]DarkStore
	Inventories map[string]StoreInventory // key: storeID_productID
}

func NewFakeRepository() *FakeRepository {
	return &FakeRepository{
		Categories:  make(map[uuid.UUID]Category),
		Products:    make(map[uuid.UUID]Product),
		DarkStores:  make(map[uuid.UUID]DarkStore),
		Inventories: make(map[string]StoreInventory),
	}
}

func (f *FakeRepository) ListCategories(ctx context.Context) ([]Category, error) {
	f.mu.RLock()
	defer f.mu.RUnlock()

	res := make([]Category, 0, len(f.Categories))
	for _, c := range f.Categories {
		res = append(res, c)
	}
	return res, nil
}

func (f *FakeRepository) GetCategoryByID(ctx context.Context, id uuid.UUID) (*Category, error) {
	f.mu.RLock()
	defer f.mu.RUnlock()

	c, ok := f.Categories[id]
	if !ok {
		return nil, ErrNotFound
	}
	return &c, nil
}

func (f *FakeRepository) CreateCategory(ctx context.Context, req CreateCategoryRequest) (*Category, error) {
	f.mu.Lock()
	defer f.mu.Unlock()

	c := Category{
		ID:          uuid.New(),
		Name:        req.Name,
		Slug:        req.Slug,
		Description: req.Description,
		CreatedAt:   time.Now(),
	}
	f.Categories[c.ID] = c
	return &c, nil
}

func (f *FakeRepository) CreateProduct(ctx context.Context, req CreateProductRequest) (*Product, error) {
	f.mu.Lock()
	defer f.mu.Unlock()

	isActive := true
	if req.IsActive != nil {
		isActive = *req.IsActive
	}

	p := Product{
		ID:          uuid.New(),
		CategoryID:  req.CategoryID,
		SKU:         req.SKU,
		Name:        req.Name,
		Description: req.Description,
		Unit:        req.Unit,
		PricePaise:  req.PricePaise,
		IsActive:    isActive,
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
	}
	f.Products[p.ID] = p
	return &p, nil
}

func (f *FakeRepository) GetProductByID(ctx context.Context, id uuid.UUID) (*Product, error) {
	f.mu.RLock()
	defer f.mu.RUnlock()

	p, ok := f.Products[id]
	if !ok {
		return nil, ErrNotFound
	}
	return &p, nil
}

func (f *FakeRepository) UpdateProduct(ctx context.Context, id uuid.UUID, req UpdateProductRequest) (*Product, error) {
	f.mu.Lock()
	defer f.mu.Unlock()

	p, ok := f.Products[id]
	if !ok {
		return nil, ErrNotFound
	}

	if req.CategoryID != nil {
		p.CategoryID = *req.CategoryID
	}
	if req.Name != nil {
		p.Name = *req.Name
	}
	if req.Description != nil {
		p.Description = *req.Description
	}
	if req.Unit != nil {
		p.Unit = *req.Unit
	}
	if req.PricePaise != nil {
		p.PricePaise = *req.PricePaise
	}
	if req.IsActive != nil {
		p.IsActive = *req.IsActive
	}
	p.UpdatedAt = time.Now()
	f.Products[id] = p
	return &p, nil
}

func (f *FakeRepository) CreateDarkStore(ctx context.Context, req CreateDarkStoreRequest) (*DarkStore, error) {
	f.mu.Lock()
	defer f.mu.Unlock()

	isActive := true
	if req.IsActive != nil {
		isActive = *req.IsActive
	}

	d := DarkStore{
		ID:              uuid.New(),
		StoreCode:       req.StoreCode,
		Name:            req.Name,
		Address:         req.Address,
		Latitude:        req.Latitude,
		Longitude:       req.Longitude,
		IsActive:        isActive,
		MaxActiveOrders: req.MaxActiveOrders,
		CreatedAt:       time.Now(),
		UpdatedAt:       time.Now(),
	}
	f.DarkStores[d.ID] = d
	return &d, nil
}

func (f *FakeRepository) GetDarkStoreByID(ctx context.Context, id uuid.UUID) (*DarkStore, error) {
	f.mu.RLock()
	defer f.mu.RUnlock()

	d, ok := f.DarkStores[id]
	if !ok {
		return nil, ErrNotFound
	}
	return &d, nil
}

func (f *FakeRepository) ListDarkStores(ctx context.Context) ([]DarkStore, error) {
	f.mu.RLock()
	defer f.mu.RUnlock()

	res := make([]DarkStore, 0, len(f.DarkStores))
	for _, d := range f.DarkStores {
		if d.IsActive {
			res = append(res, d)
		}
	}
	return res, nil
}

func (f *FakeRepository) ListProductsByStore(ctx context.Context, storeID uuid.UUID, limit, offset int32) ([]StoreProduct, error) {
	f.mu.RLock()
	defer f.mu.RUnlock()

	var res []StoreProduct
	for _, p := range f.Products {
		if !p.IsActive {
			continue
		}
		key := storeID.String() + "_" + p.ID.String()
		inv, ok := f.Inventories[key]
		avail := int32(0)
		resv := int32(0)
		if ok {
			avail = inv.AvailableQuantity
			resv = inv.ReservedQuantity
		}
		res = append(res, StoreProduct{
			Product:           p,
			StoreID:           storeID,
			AvailableQuantity: avail,
			ReservedQuantity:  resv,
		})
	}
	return res, nil
}

func (f *FakeRepository) UpsertStoreInventory(ctx context.Context, storeID, productID uuid.UUID, quantity int32) (*StoreInventory, error) {
	f.mu.Lock()
	defer f.mu.Unlock()

	key := storeID.String() + "_" + productID.String()
	inv, ok := f.Inventories[key]
	if !ok {
		inv = StoreInventory{
			StoreID:           storeID,
			ProductID:         productID,
			AvailableQuantity: quantity,
			ReservedQuantity:  0,
			Version:           1,
			CreatedAt:         time.Now(),
			UpdatedAt:         time.Now(),
		}
	} else {
		inv.AvailableQuantity = quantity
		inv.Version++
		inv.UpdatedAt = time.Now()
	}
	f.Inventories[key] = inv
	return &inv, nil
}
