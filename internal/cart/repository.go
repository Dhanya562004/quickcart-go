package cart

import (
	"context"
	"errors"
	"fmt"

	"github.com/Dhanya562004/quickcart-go/internal/db"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
)

var (
	ErrNotFound   = errors.New("cart not found")
	ErrDatabaseOp = errors.New("cart database operation failed")
)

type Repository interface {
	GetCartByUserID(ctx context.Context, userID uuid.UUID) (*Cart, error)
	CreateCart(ctx context.Context, userID uuid.UUID, storeID *uuid.UUID) (*Cart, error)
	UpdateCartStore(ctx context.Context, cartID uuid.UUID, storeID *uuid.UUID) error
	GetCartItems(ctx context.Context, cartID uuid.UUID) ([]CartItem, error)
	UpsertCartItem(ctx context.Context, cartID, productID uuid.UUID, quantity int32) (*CartItem, error)
	UpdateCartItemQuantity(ctx context.Context, cartID, productID uuid.UUID, quantity int32) (*CartItem, error)
	DeleteCartItem(ctx context.Context, cartID, productID uuid.UUID) error
	ClearCart(ctx context.Context, cartID uuid.UUID) error
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

func toOptionalPGUUID(u *uuid.UUID) pgtype.UUID {
	if u == nil || *u == uuid.Nil {
		return pgtype.UUID{Valid: false}
	}
	return pgtype.UUID{Bytes: *u, Valid: true}
}

func fromPGUUID(p pgtype.UUID) uuid.UUID {
	if !p.Valid {
		return uuid.Nil
	}
	return uuid.UUID(p.Bytes)
}

func fromOptionalPGUUID(p pgtype.UUID) *uuid.UUID {
	if !p.Valid {
		return nil
	}
	u := uuid.UUID(p.Bytes)
	return &u
}

func (r *pgxRepository) GetCartByUserID(ctx context.Context, userID uuid.UUID) (*Cart, error) {
	row, err := r.q.GetCartByUserID(ctx, toPGUUID(userID))
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("%w: %w", ErrDatabaseOp, err)
	}

	return &Cart{
		ID:        fromPGUUID(row.ID),
		UserID:    fromPGUUID(row.UserID),
		StoreID:   fromOptionalPGUUID(row.StoreID),
		CreatedAt: row.CreatedAt.Time,
		UpdatedAt: row.UpdatedAt.Time,
	}, nil
}

func (r *pgxRepository) CreateCart(ctx context.Context, userID uuid.UUID, storeID *uuid.UUID) (*Cart, error) {
	row, err := r.q.CreateCart(ctx, db.CreateCartParams{
		UserID:  toPGUUID(userID),
		StoreID: toOptionalPGUUID(storeID),
	})
	if err != nil {
		return nil, fmt.Errorf("%w: %w", ErrDatabaseOp, err)
	}

	return &Cart{
		ID:        fromPGUUID(row.ID),
		UserID:    fromPGUUID(row.UserID),
		StoreID:   fromOptionalPGUUID(row.StoreID),
		CreatedAt: row.CreatedAt.Time,
		UpdatedAt: row.UpdatedAt.Time,
	}, nil
}

func (r *pgxRepository) UpdateCartStore(ctx context.Context, cartID uuid.UUID, storeID *uuid.UUID) error {
	_, err := r.q.UpdateCartStore(ctx, db.UpdateCartStoreParams{
		ID:      toPGUUID(cartID),
		StoreID: toOptionalPGUUID(storeID),
	})
	if err != nil {
		return fmt.Errorf("%w: %w", ErrDatabaseOp, err)
	}
	return nil
}

func (r *pgxRepository) GetCartItems(ctx context.Context, cartID uuid.UUID) ([]CartItem, error) {
	rows, err := r.q.GetCartItems(ctx, toPGUUID(cartID))
	if err != nil {
		return nil, fmt.Errorf("%w: %w", ErrDatabaseOp, err)
	}

	items := make([]CartItem, len(rows))
	for i, row := range rows {
		items[i] = CartItem{
			ID:              fromPGUUID(row.ID),
			CartID:          fromPGUUID(row.CartID),
			ProductID:       fromPGUUID(row.ProductID),
			ProductName:     row.ProductName,
			ProductSKU:      row.ProductSku,
			ProductUnit:     row.ProductUnit,
			UnitPricePaise:  row.UnitPricePaise,
			Quantity:        row.Quantity,
			TotalPricePaise: row.TotalPricePaise,
			CreatedAt:       row.CreatedAt.Time,
			UpdatedAt:       row.UpdatedAt.Time,
		}
	}
	return items, nil
}

func (r *pgxRepository) UpsertCartItem(ctx context.Context, cartID, productID uuid.UUID, quantity int32) (*CartItem, error) {
	row, err := r.q.UpsertCartItem(ctx, db.UpsertCartItemParams{
		CartID:    toPGUUID(cartID),
		ProductID: toPGUUID(productID),
		Quantity:  quantity,
	})
	if err != nil {
		return nil, fmt.Errorf("%w: %w", ErrDatabaseOp, err)
	}

	return &CartItem{
		ID:        fromPGUUID(row.ID),
		CartID:    fromPGUUID(row.CartID),
		ProductID: fromPGUUID(row.ProductID),
		Quantity:  row.Quantity,
		CreatedAt: row.CreatedAt.Time,
		UpdatedAt: row.UpdatedAt.Time,
	}, nil
}

func (r *pgxRepository) UpdateCartItemQuantity(ctx context.Context, cartID, productID uuid.UUID, quantity int32) (*CartItem, error) {
	row, err := r.q.UpdateCartItemQuantity(ctx, db.UpdateCartItemQuantityParams{
		CartID:    toPGUUID(cartID),
		ProductID: toPGUUID(productID),
		Quantity:  quantity,
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("%w: %w", ErrDatabaseOp, err)
	}

	return &CartItem{
		ID:        fromPGUUID(row.ID),
		CartID:    fromPGUUID(row.CartID),
		ProductID: fromPGUUID(row.ProductID),
		Quantity:  row.Quantity,
		CreatedAt: row.CreatedAt.Time,
		UpdatedAt: row.UpdatedAt.Time,
	}, nil
}

func (r *pgxRepository) DeleteCartItem(ctx context.Context, cartID, productID uuid.UUID) error {
	err := r.q.DeleteCartItem(ctx, db.DeleteCartItemParams{
		CartID:    toPGUUID(cartID),
		ProductID: toPGUUID(productID),
	})
	if err != nil {
		return fmt.Errorf("%w: %w", ErrDatabaseOp, err)
	}
	return nil
}

func (r *pgxRepository) ClearCart(ctx context.Context, cartID uuid.UUID) error {
	err := r.q.ClearCart(ctx, toPGUUID(cartID))
	if err != nil {
		return fmt.Errorf("%w: %w", ErrDatabaseOp, err)
	}
	return nil
}
