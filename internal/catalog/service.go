package catalog

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"
)

var (
	ErrInvalidPrice     = errors.New("price_paise cannot be negative")
	ErrStoreInactive    = errors.New("dark store is currently inactive")
	ErrInvalidQuantity = errors.New("available quantity cannot be negative")
)

type Service interface {
	ListCategories(ctx context.Context) ([]Category, error)
	GetCategory(ctx context.Context, id uuid.UUID) (*Category, error)
	CreateCategory(ctx context.Context, req CreateCategoryRequest) (*Category, error)

	CreateProduct(ctx context.Context, req CreateProductRequest) (*Product, error)
	GetProduct(ctx context.Context, id uuid.UUID) (*Product, error)
	UpdateProduct(ctx context.Context, id uuid.UUID, req UpdateProductRequest) (*Product, error)

	CreateDarkStore(ctx context.Context, req CreateDarkStoreRequest) (*DarkStore, error)
	GetDarkStore(ctx context.Context, id uuid.UUID) (*DarkStore, error)
	ListDarkStores(ctx context.Context) ([]DarkStore, error)

	ListProductsByStore(ctx context.Context, storeID uuid.UUID, limit, offset int32) ([]StoreProduct, error)
	UpsertStoreInventory(ctx context.Context, storeID, productID uuid.UUID, quantity int32) (*StoreInventory, error)
}

type service struct {
	repo Repository
}

func NewService(repo Repository) Service {
	return &service{repo: repo}
}

func (s *service) ListCategories(ctx context.Context) ([]Category, error) {
	return s.repo.ListCategories(ctx)
}

func (s *service) GetCategory(ctx context.Context, id uuid.UUID) (*Category, error) {
	return s.repo.GetCategoryByID(ctx, id)
}

func (s *service) CreateCategory(ctx context.Context, req CreateCategoryRequest) (*Category, error) {
	return s.repo.CreateCategory(ctx, req)
}

func (s *service) CreateProduct(ctx context.Context, req CreateProductRequest) (*Product, error) {
	if req.PricePaise < 0 {
		return nil, ErrInvalidPrice
	}
	// Verify category exists
	if _, err := s.repo.GetCategoryByID(ctx, req.CategoryID); err != nil {
		return nil, fmt.Errorf("category %s: %w", req.CategoryID, err)
	}
	return s.repo.CreateProduct(ctx, req)
}

func (s *service) GetProduct(ctx context.Context, id uuid.UUID) (*Product, error) {
	return s.repo.GetProductByID(ctx, id)
}

func (s *service) UpdateProduct(ctx context.Context, id uuid.UUID, req UpdateProductRequest) (*Product, error) {
	if req.PricePaise != nil && *req.PricePaise < 0 {
		return nil, ErrInvalidPrice
	}
	if req.CategoryID != nil {
		if _, err := s.repo.GetCategoryByID(ctx, *req.CategoryID); err != nil {
			return nil, fmt.Errorf("category %s: %w", *req.CategoryID, err)
		}
	}
	return s.repo.UpdateProduct(ctx, id, req)
}

func (s *service) CreateDarkStore(ctx context.Context, req CreateDarkStoreRequest) (*DarkStore, error) {
	return s.repo.CreateDarkStore(ctx, req)
}

func (s *service) GetDarkStore(ctx context.Context, id uuid.UUID) (*DarkStore, error) {
	return s.repo.GetDarkStoreByID(ctx, id)
}

func (s *service) ListDarkStores(ctx context.Context) ([]DarkStore, error) {
	return s.repo.ListDarkStores(ctx)
}

func (s *service) ListProductsByStore(ctx context.Context, storeID uuid.UUID, limit, offset int32) ([]StoreProduct, error) {
	store, err := s.repo.GetDarkStoreByID(ctx, storeID)
	if err != nil {
		return nil, err
	}
	if !store.IsActive {
		return nil, ErrStoreInactive
	}

	if limit <= 0 || limit > 100 {
		limit = 20
	}
	if offset < 0 {
		offset = 0
	}

	return s.repo.ListProductsByStore(ctx, storeID, limit, offset)
}

func (s *service) UpsertStoreInventory(ctx context.Context, storeID, productID uuid.UUID, quantity int32) (*StoreInventory, error) {
	if quantity < 0 {
		return nil, ErrInvalidQuantity
	}
	if _, err := s.repo.GetDarkStoreByID(ctx, storeID); err != nil {
		return nil, fmt.Errorf("store %s: %w", storeID, err)
	}
	if _, err := s.repo.GetProductByID(ctx, productID); err != nil {
		return nil, fmt.Errorf("product %s: %w", productID, err)
	}

	return s.repo.UpsertStoreInventory(ctx, storeID, productID, quantity)
}
