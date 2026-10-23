package catalog

import (
	"context"
	"errors"
	"fmt"
	"math/big"

	"github.com/Dhanya562004/quickcart-go/internal/db"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
)

var (
	ErrNotFound       = errors.New("resource not found")
	ErrDuplicateKey   = errors.New("resource with unique identifier already exists")
	ErrDatabaseOp     = errors.New("database operation failed")
)

type Repository interface {
	ListCategories(ctx context.Context) ([]Category, error)
	GetCategoryByID(ctx context.Context, id uuid.UUID) (*Category, error)
	CreateCategory(ctx context.Context, req CreateCategoryRequest) (*Category, error)

	CreateProduct(ctx context.Context, req CreateProductRequest) (*Product, error)
	GetProductByID(ctx context.Context, id uuid.UUID) (*Product, error)
	UpdateProduct(ctx context.Context, id uuid.UUID, req UpdateProductRequest) (*Product, error)

	CreateDarkStore(ctx context.Context, req CreateDarkStoreRequest) (*DarkStore, error)
	GetDarkStoreByID(ctx context.Context, id uuid.UUID) (*DarkStore, error)
	ListDarkStores(ctx context.Context) ([]DarkStore, error)

	ListProductsByStore(ctx context.Context, storeID uuid.UUID, limit, offset int32) ([]StoreProduct, error)
	UpsertStoreInventory(ctx context.Context, storeID, productID uuid.UUID, quantity int32) (*StoreInventory, error)
}

type pgxRepository struct {
	q db.Querier
}

func NewRepository(q db.Querier) Repository {
	return &pgxRepository{q: q}
}

func toPGUUID(u uuid.UUID) pgtype.UUID {
	return pgtype.UUID{Bytes: u, Valid: true}
}

func fromPGUUID(p pgtype.UUID) uuid.UUID {
	if !p.Valid {
		return uuid.Nil
	}
	return uuid.UUID(p.Bytes)
}

func floatToNumeric(f float64) pgtype.Numeric {
	var num pgtype.Numeric
	_ = num.Scan(fmt.Sprintf("%.7f", f))
	return num
}

func numericToFloat(n pgtype.Numeric) float64 {
	if !n.Valid {
		return 0
	}
	f, _ := n.Float64Value()
	if f.Valid {
		return f.Float64
	}
	if n.Int != nil {
		bigFloat := new(big.Float).SetInt(n.Int)
		if n.Exp < 0 {
			div := new(big.Int).Exp(big.NewInt(10), big.NewInt(int64(-n.Exp)), nil)
			bigFloat.Quo(bigFloat, new(big.Float).SetInt(div))
		}
		res, _ := bigFloat.Float64()
		return res
	}
	return 0
}

func (r *pgxRepository) ListCategories(ctx context.Context) ([]Category, error) {
	rows, err := r.q.ListCategories(ctx)
	if err != nil {
		return nil, fmt.Errorf("%w: %w", ErrDatabaseOp, err)
	}

	categories := make([]Category, len(rows))
	for i, row := range rows {
		categories[i] = Category{
			ID:          fromPGUUID(row.ID),
			Name:        row.Name,
			Slug:        row.Slug,
			Description: row.Description.String,
			CreatedAt:   row.CreatedAt.Time,
		}
	}
	return categories, nil
}

func (r *pgxRepository) GetCategoryByID(ctx context.Context, id uuid.UUID) (*Category, error) {
	row, err := r.q.GetCategoryByID(ctx, toPGUUID(id))
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("%w: %w", ErrDatabaseOp, err)
	}

	return &Category{
		ID:          fromPGUUID(row.ID),
		Name:        row.Name,
		Slug:        row.Slug,
		Description: row.Description.String,
		CreatedAt:   row.CreatedAt.Time,
	}, nil
}

func (r *pgxRepository) CreateCategory(ctx context.Context, req CreateCategoryRequest) (*Category, error) {
	desc := pgtype.Text{String: req.Description, Valid: req.Description != ""}
	row, err := r.q.CreateCategory(ctx, db.CreateCategoryParams{
		Name:        req.Name,
		Slug:        req.Slug,
		Description: desc,
	})
	if err != nil {
		return nil, fmt.Errorf("%w: %w", ErrDatabaseOp, err)
	}

	return &Category{
		ID:          fromPGUUID(row.ID),
		Name:        row.Name,
		Slug:        row.Slug,
		Description: row.Description.String,
		CreatedAt:   row.CreatedAt.Time,
	}, nil
}

func (r *pgxRepository) CreateProduct(ctx context.Context, req CreateProductRequest) (*Product, error) {
	isActive := true
	if req.IsActive != nil {
		isActive = *req.IsActive
	}
	desc := pgtype.Text{String: req.Description, Valid: req.Description != ""}

	row, err := r.q.CreateProduct(ctx, db.CreateProductParams{
		CategoryID:  toPGUUID(req.CategoryID),
		Sku:         req.SKU,
		Name:        req.Name,
		Description: desc,
		Unit:        req.Unit,
		PricePaise:  req.PricePaise,
		IsActive:    isActive,
	})
	if err != nil {
		return nil, fmt.Errorf("%w: %w", ErrDatabaseOp, err)
	}

	return &Product{
		ID:          fromPGUUID(row.ID),
		CategoryID:  fromPGUUID(row.CategoryID),
		SKU:         row.Sku,
		Name:        row.Name,
		Description: row.Description.String,
		Unit:        row.Unit,
		PricePaise:  row.PricePaise,
		IsActive:    row.IsActive,
		CreatedAt:   row.CreatedAt.Time,
		UpdatedAt:   row.UpdatedAt.Time,
	}, nil
}

func (r *pgxRepository) GetProductByID(ctx context.Context, id uuid.UUID) (*Product, error) {
	row, err := r.q.GetProductByID(ctx, toPGUUID(id))
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("%w: %w", ErrDatabaseOp, err)
	}

	return &Product{
		ID:          fromPGUUID(row.ID),
		CategoryID:  fromPGUUID(row.CategoryID),
		SKU:         row.Sku,
		Name:        row.Name,
		Description: row.Description.String,
		Unit:        row.Unit,
		PricePaise:  row.PricePaise,
		IsActive:    row.IsActive,
		CreatedAt:   row.CreatedAt.Time,
		UpdatedAt:   row.UpdatedAt.Time,
	}, nil
}

func (r *pgxRepository) UpdateProduct(ctx context.Context, id uuid.UUID, req UpdateProductRequest) (*Product, error) {
	current, err := r.GetProductByID(ctx, id)
	if err != nil {
		return nil, err
	}

	categoryID := current.CategoryID
	if req.CategoryID != nil {
		categoryID = *req.CategoryID
	}
	name := current.Name
	if req.Name != nil {
		name = *req.Name
	}
	descStr := current.Description
	if req.Description != nil {
		descStr = *req.Description
	}
	unit := current.Unit
	if req.Unit != nil {
		unit = *req.Unit
	}
	pricePaise := current.PricePaise
	if req.PricePaise != nil {
		pricePaise = *req.PricePaise
	}
	isActive := current.IsActive
	if req.IsActive != nil {
		isActive = *req.IsActive
	}

	desc := pgtype.Text{String: descStr, Valid: descStr != ""}

	row, err := r.q.UpdateProduct(ctx, db.UpdateProductParams{
		ID:          toPGUUID(id),
		CategoryID:  toPGUUID(categoryID),
		Name:        name,
		Description: desc,
		Unit:        unit,
		PricePaise:  pricePaise,
		IsActive:    isActive,
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("%w: %w", ErrDatabaseOp, err)
	}

	return &Product{
		ID:          fromPGUUID(row.ID),
		CategoryID:  fromPGUUID(row.CategoryID),
		SKU:         row.Sku,
		Name:        row.Name,
		Description: row.Description.String,
		Unit:        row.Unit,
		PricePaise:  row.PricePaise,
		IsActive:    row.IsActive,
		CreatedAt:   row.CreatedAt.Time,
		UpdatedAt:   row.UpdatedAt.Time,
	}, nil
}

func (r *pgxRepository) CreateDarkStore(ctx context.Context, req CreateDarkStoreRequest) (*DarkStore, error) {
	isActive := true
	if req.IsActive != nil {
		isActive = *req.IsActive
	}

	row, err := r.q.CreateDarkStore(ctx, db.CreateDarkStoreParams{
		StoreCode:       req.StoreCode,
		Name:            req.Name,
		Address:         req.Address,
		Latitude:        floatToNumeric(req.Latitude),
		Longitude:       floatToNumeric(req.Longitude),
		IsActive:        isActive,
		MaxActiveOrders: req.MaxActiveOrders,
	})
	if err != nil {
		return nil, fmt.Errorf("%w: %w", ErrDatabaseOp, err)
	}

	return &DarkStore{
		ID:              fromPGUUID(row.ID),
		StoreCode:       row.StoreCode,
		Name:            row.Name,
		Address:         row.Address,
		Latitude:        numericToFloat(row.Latitude),
		Longitude:       numericToFloat(row.Longitude),
		IsActive:        row.IsActive,
		MaxActiveOrders: row.MaxActiveOrders,
		CreatedAt:       row.CreatedAt.Time,
		UpdatedAt:       row.UpdatedAt.Time,
	}, nil
}

func (r *pgxRepository) GetDarkStoreByID(ctx context.Context, id uuid.UUID) (*DarkStore, error) {
	row, err := r.q.GetDarkStoreByID(ctx, toPGUUID(id))
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("%w: %w", ErrDatabaseOp, err)
	}

	return &DarkStore{
		ID:              fromPGUUID(row.ID),
		StoreCode:       row.StoreCode,
		Name:            row.Name,
		Address:         row.Address,
		Latitude:        numericToFloat(row.Latitude),
		Longitude:       numericToFloat(row.Longitude),
		IsActive:        row.IsActive,
		MaxActiveOrders: row.MaxActiveOrders,
		CreatedAt:       row.CreatedAt.Time,
		UpdatedAt:       row.UpdatedAt.Time,
	}, nil
}

func (r *pgxRepository) ListDarkStores(ctx context.Context) ([]DarkStore, error) {
	rows, err := r.q.ListDarkStores(ctx)
	if err != nil {
		return nil, fmt.Errorf("%w: %w", ErrDatabaseOp, err)
	}

	stores := make([]DarkStore, len(rows))
	for i, row := range rows {
		stores[i] = DarkStore{
			ID:              fromPGUUID(row.ID),
			StoreCode:       row.StoreCode,
			Name:            row.Name,
			Address:         row.Address,
			Latitude:        numericToFloat(row.Latitude),
			Longitude:       numericToFloat(row.Longitude),
			IsActive:        row.IsActive,
			MaxActiveOrders: row.MaxActiveOrders,
			CreatedAt:       row.CreatedAt.Time,
			UpdatedAt:       row.UpdatedAt.Time,
		}
	}
	return stores, nil
}

func (r *pgxRepository) ListProductsByStore(ctx context.Context, storeID uuid.UUID, limit, offset int32) ([]StoreProduct, error) {
	rows, err := r.q.ListProductsByStore(ctx, db.ListProductsByStoreParams{
		StoreID: toPGUUID(storeID),
		Limit:   limit,
		Offset:  offset,
	})
	if err != nil {
		return nil, fmt.Errorf("%w: %w", ErrDatabaseOp, err)
	}

	items := make([]StoreProduct, len(rows))
	for i, row := range rows {
		items[i] = StoreProduct{
			Product: Product{
				ID:          fromPGUUID(row.ProductID),
				CategoryID:  fromPGUUID(row.CategoryID),
				SKU:         row.Sku,
				Name:        row.Name,
				Description: row.Description.String,
				Unit:        row.Unit,
				PricePaise:  row.PricePaise,
				IsActive:    row.IsActive,
			},
			StoreID:           fromPGUUID(row.StoreID),
			AvailableQuantity: row.AvailableQuantity,
			ReservedQuantity:  row.ReservedQuantity,
		}
	}
	return items, nil
}

func (r *pgxRepository) UpsertStoreInventory(ctx context.Context, storeID, productID uuid.UUID, quantity int32) (*StoreInventory, error) {
	row, err := r.q.UpsertStoreInventory(ctx, db.UpsertStoreInventoryParams{
		StoreID:           toPGUUID(storeID),
		ProductID:         toPGUUID(productID),
		AvailableQuantity: quantity,
	})
	if err != nil {
		return nil, fmt.Errorf("%w: %w", ErrDatabaseOp, err)
	}

	return &StoreInventory{
		StoreID:           fromPGUUID(row.StoreID),
		ProductID:         fromPGUUID(row.ProductID),
		AvailableQuantity: row.AvailableQuantity,
		ReservedQuantity:  row.ReservedQuantity,
		Version:           row.Version,
		CreatedAt:         row.CreatedAt.Time,
		UpdatedAt:         row.UpdatedAt.Time,
	}, nil
}
