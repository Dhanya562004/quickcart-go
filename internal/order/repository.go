package order

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"time"

	"github.com/Dhanya562004/quickcart-go/internal/cart"
	"github.com/Dhanya562004/quickcart-go/internal/db"
	"github.com/Dhanya562004/quickcart-go/internal/platform/database"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
)

var (
	ErrNotFound                   = errors.New("order not found")
	ErrDatabaseOp                 = errors.New("order database operation failed")
	ErrOrderAlreadyCancelled      = errors.New("order is already cancelled")
	ErrIdempotencyPayloadMismatch = errors.New("idempotency key was previously used with a different request payload")
	ErrIdempotencyRaceConflict    = errors.New("idempotency race detected; retry to fetch committed order")
)

// ErrInsufficientStock indicates that conditional reservation failed due to depleted inventory.
type ErrInsufficientStock struct {
	ProductID   uuid.UUID
	ProductName string
	Requested   int32
	Available   int32
}

func (e *ErrInsufficientStock) Error() string {
	return fmt.Sprintf("insufficient stock for product %s (%s): requested %d, available %d",
		e.ProductName, e.ProductID, e.Requested, e.Available)
}

type CheckoutParams struct {
	UserID          uuid.UUID
	CartID          uuid.UUID
	StoreID         uuid.UUID
	DeliveryAddress string
	IdempotencyKey  string
	RequestHash     string
	Items           []cart.CartItem
}

type CheckoutResult struct {
	Order       *Order
	IsDuplicate bool
}

type Repository interface {
	GetOrderByID(ctx context.Context, id uuid.UUID) (*Order, error)
	GetOrderByUserAndIdempotencyKey(ctx context.Context, userID uuid.UUID, idempotencyKey string) (*Order, error)
	ListOrdersByUserID(ctx context.Context, userID uuid.UUID, limit, offset int32) ([]Order, error)
	GetOrderItems(ctx context.Context, orderID uuid.UUID) ([]OrderItem, error)
	GetReservations(ctx context.Context, orderID uuid.UUID) ([]Reservation, error)

	CheckoutTx(ctx context.Context, params CheckoutParams) (*CheckoutResult, error)
	CancelOrderTx(ctx context.Context, orderID uuid.UUID, reason string, actorID *uuid.UUID) (*Order, error)

	GetIdempotencyRecord(ctx context.Context, userID uuid.UUID, key string) (*db.IdempotencyKey, error)
}

type pgxRepository struct {
	pool    *pgxpool.Pool
	queries *db.Queries
}

func NewRepository(pool *pgxpool.Pool) Repository {
	return &pgxRepository{
		pool:    pool,
		queries: db.New(pool),
	}
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

func (r *pgxRepository) GetOrderByID(ctx context.Context, id uuid.UUID) (*Order, error) {
	row, err := r.queries.GetOrderByID(ctx, toPGUUID(id))
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("%w: %w", ErrDatabaseOp, err)
	}

	ord := &Order{
		ID:                 fromPGUUID(row.ID),
		OrderNumber:        row.OrderNumber,
		UserID:             fromPGUUID(row.UserID),
		StoreID:            fromPGUUID(row.StoreID),
		Status:             row.Status,
		TotalAmountPaise:   row.TotalAmountPaise,
		DeliveryAddress:    row.DeliveryAddress,
		IdempotencyKey:     row.IdempotencyKey.String,
		CancellationReason: row.CancellationReason.String,
		CreatedAt:          row.CreatedAt.Time,
		UpdatedAt:          row.UpdatedAt.Time,
	}

	items, err := r.GetOrderItems(ctx, ord.ID)
	if err == nil {
		ord.Items = items
	}
	return ord, nil
}

func (r *pgxRepository) GetOrderByUserAndIdempotencyKey(ctx context.Context, userID uuid.UUID, idempotencyKey string) (*Order, error) {
	row, err := r.queries.GetOrderByUserAndIdempotencyKey(ctx, db.GetOrderByUserAndIdempotencyKeyParams{
		UserID:         toPGUUID(userID),
		IdempotencyKey: pgtype.Text{String: idempotencyKey, Valid: true},
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("%w: %w", ErrDatabaseOp, err)
	}

	ord := &Order{
		ID:                 fromPGUUID(row.ID),
		OrderNumber:        row.OrderNumber,
		UserID:             fromPGUUID(row.UserID),
		StoreID:            fromPGUUID(row.StoreID),
		Status:             row.Status,
		TotalAmountPaise:   row.TotalAmountPaise,
		DeliveryAddress:    row.DeliveryAddress,
		IdempotencyKey:     row.IdempotencyKey.String,
		CancellationReason: row.CancellationReason.String,
		CreatedAt:          row.CreatedAt.Time,
		UpdatedAt:          row.UpdatedAt.Time,
	}

	items, err := r.GetOrderItems(ctx, ord.ID)
	if err == nil {
		ord.Items = items
	}
	return ord, nil
}

func (r *pgxRepository) ListOrdersByUserID(ctx context.Context, userID uuid.UUID, limit, offset int32) ([]Order, error) {
	rows, err := r.queries.ListOrdersByUserID(ctx, db.ListOrdersByUserIDParams{
		UserID: toPGUUID(userID),
		Limit:  limit,
		Offset: offset,
	})
	if err != nil {
		return nil, fmt.Errorf("%w: %w", ErrDatabaseOp, err)
	}

	orders := make([]Order, len(rows))
	for i, row := range rows {
		orders[i] = Order{
			ID:                 fromPGUUID(row.ID),
			OrderNumber:        row.OrderNumber,
			UserID:             fromPGUUID(row.UserID),
			StoreID:            fromPGUUID(row.StoreID),
			Status:             row.Status,
			TotalAmountPaise:   row.TotalAmountPaise,
			DeliveryAddress:    row.DeliveryAddress,
			IdempotencyKey:     row.IdempotencyKey.String,
			CancellationReason: row.CancellationReason.String,
			CreatedAt:          row.CreatedAt.Time,
			UpdatedAt:          row.UpdatedAt.Time,
		}
	}
	return orders, nil
}

func (r *pgxRepository) GetOrderItems(ctx context.Context, orderID uuid.UUID) ([]OrderItem, error) {
	rows, err := r.queries.GetOrderItemsByOrderID(ctx, toPGUUID(orderID))
	if err != nil {
		return nil, fmt.Errorf("%w: %w", ErrDatabaseOp, err)
	}

	items := make([]OrderItem, len(rows))
	for i, row := range rows {
		items[i] = OrderItem{
			ID:              fromPGUUID(row.ID),
			OrderID:         fromPGUUID(row.OrderID),
			ProductID:       fromPGUUID(row.ProductID),
			ProductName:     row.ProductName,
			ProductSKU:      row.ProductSku,
			ProductUnit:     row.ProductUnit,
			Quantity:        row.Quantity,
			UnitPricePaise:  row.UnitPricePaise,
			TotalPricePaise: row.TotalPricePaise,
			CreatedAt:       row.CreatedAt.Time,
		}
	}
	return items, nil
}

func (r *pgxRepository) GetReservations(ctx context.Context, orderID uuid.UUID) ([]Reservation, error) {
	rows, err := r.queries.GetReservationsByOrderID(ctx, toPGUUID(orderID))
	if err != nil {
		return nil, fmt.Errorf("%w: %w", ErrDatabaseOp, err)
	}

	res := make([]Reservation, len(rows))
	for i, row := range rows {
		res[i] = Reservation{
			ID:        fromPGUUID(row.ID),
			OrderID:   fromPGUUID(row.OrderID),
			StoreID:   fromPGUUID(row.StoreID),
			ProductID: fromPGUUID(row.ProductID),
			Quantity:  row.Quantity,
			Status:    row.Status,
			ExpiresAt: row.ExpiresAt.Time,
			CreatedAt: row.CreatedAt.Time,
			UpdatedAt: row.UpdatedAt.Time,
		}
	}
	return res, nil
}

func (r *pgxRepository) GetIdempotencyRecord(ctx context.Context, userID uuid.UUID, key string) (*db.IdempotencyKey, error) {
	record, err := r.queries.GetIdempotencyRecord(ctx, db.GetIdempotencyRecordParams{
		UserID: toPGUUID(userID),
		Key:    key,
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("%w: %w", ErrDatabaseOp, err)
	}
	return &record, nil
}

// CheckoutTx coordinates the zero-overselling inventory reservation, order placement,
// reservation audit log, and cart clearing in a single ACID transaction.
func (r *pgxRepository) CheckoutTx(ctx context.Context, params CheckoutParams) (*CheckoutResult, error) {
	var finalOrder *Order

	err := database.WithTx(ctx, r.pool, func(tx pgx.Tx) error {
		qtx := r.queries.WithTx(tx)

		// 1. Sort items by ProductID to eliminate deadlock risk on concurrent updates
		sortedItems := make([]cart.CartItem, len(params.Items))
		copy(sortedItems, params.Items)
		sort.Slice(sortedItems, func(i, j int) bool {
			return sortedItems[i].ProductID.String() < sortedItems[j].ProductID.String()
		})

		// 2. Atomically reserve inventory for each item using conditional updates
		for _, item := range sortedItems {
			_, err := qtx.ReserveInventoryAtomic(ctx, db.ReserveInventoryAtomicParams{
				StoreID:           toPGUUID(params.StoreID),
				ProductID:         toPGUUID(item.ProductID),
				AvailableQuantity: item.Quantity,
			})
			if err != nil {
				if errors.Is(err, pgx.ErrNoRows) {
					// Fetch available quantity for clear error reporting
					inv, qErr := qtx.GetStoreInventory(ctx, db.GetStoreInventoryParams{
						StoreID:   toPGUUID(params.StoreID),
						ProductID: toPGUUID(item.ProductID),
					})
					avail := int32(0)
					if qErr == nil {
						avail = inv.AvailableQuantity
					}
					return &ErrInsufficientStock{
						ProductID:   item.ProductID,
						ProductName: item.ProductName,
						Requested:   item.Quantity,
						Available:   avail,
					}
				}
				return fmt.Errorf("failed to reserve inventory for product %s: %w", item.ProductID, err)
			}
		}

		// 3. Compute total order amount in paise
		var totalPaise int64
		for _, item := range sortedItems {
			totalPaise += item.TotalPricePaise
		}

		// 4. Insert order record (enforces uq_orders_user_idempotency_key)
		orderRow, err := qtx.CreateOrder(ctx, db.CreateOrderParams{
			UserID:           toPGUUID(params.UserID),
			StoreID:          toPGUUID(params.StoreID),
			Status:           StatusPlaced,
			TotalAmountPaise: totalPaise,
			DeliveryAddress:  params.DeliveryAddress,
			IdempotencyKey:   pgtype.Text{String: params.IdempotencyKey, Valid: params.IdempotencyKey != ""},
		})
		if err != nil {
			var pgErr *pgconn.PgError
			if errors.As(err, &pgErr) && pgErr.Code == "23505" {
				return ErrIdempotencyRaceConflict
			}
			return fmt.Errorf("failed to create order: %w", err)
		}

		orderID := fromPGUUID(orderRow.ID)

		// 5. Insert order items & inventory reservations (10-minute TTL)
		orderItems := make([]OrderItem, len(sortedItems))
		reservations := make([]Reservation, len(sortedItems))
		expiresAt := time.Now().Add(10 * time.Minute)

		for i, item := range sortedItems {
			oiRow, err := qtx.CreateOrderItem(ctx, db.CreateOrderItemParams{
				OrderID:         toPGUUID(orderID),
				ProductID:       toPGUUID(item.ProductID),
				Quantity:        item.Quantity,
				UnitPricePaise:  item.UnitPricePaise,
				TotalPricePaise: item.TotalPricePaise,
			})
			if err != nil {
				return fmt.Errorf("failed to create order item: %w", err)
			}
			orderItems[i] = OrderItem{
				ID:              fromPGUUID(oiRow.ID),
				OrderID:         orderID,
				ProductID:       item.ProductID,
				ProductName:     item.ProductName,
				ProductSKU:      item.ProductSKU,
				ProductUnit:     item.ProductUnit,
				Quantity:        item.Quantity,
				UnitPricePaise:  item.UnitPricePaise,
				TotalPricePaise: item.TotalPricePaise,
				CreatedAt:       oiRow.CreatedAt.Time,
			}

			resRow, err := qtx.CreateInventoryReservation(ctx, db.CreateInventoryReservationParams{
				OrderID:   toPGUUID(orderID),
				StoreID:   toPGUUID(params.StoreID),
				ProductID: toPGUUID(item.ProductID),
				Quantity:  item.Quantity,
				ExpiresAt: pgtype.Timestamptz{Time: expiresAt, Valid: true},
			})
			if err != nil {
				return fmt.Errorf("failed to create inventory reservation: %w", err)
			}
			reservations[i] = Reservation{
				ID:        fromPGUUID(resRow.ID),
				OrderID:   orderID,
				StoreID:   params.StoreID,
				ProductID: item.ProductID,
				Quantity:  item.Quantity,
				Status:    resRow.Status,
				ExpiresAt: resRow.ExpiresAt.Time,
				CreatedAt: resRow.CreatedAt.Time,
				UpdatedAt: resRow.UpdatedAt.Time,
			}
		}

		// 6. Audit Trail: Insert order status history
		_, err = qtx.InsertOrderStatusHistory(ctx, db.InsertOrderStatusHistoryParams{
			OrderID:    toPGUUID(orderID),
			FromStatus: pgtype.Text{Valid: false},
			ToStatus:   StatusPlaced,
			ActorType:  "USER",
			ActorID:    toOptionalPGUUID(&params.UserID),
			Notes:      pgtype.Text{String: "Order created via checkout", Valid: true},
		})
		if err != nil {
			return fmt.Errorf("failed to record status history: %w", err)
		}

		// 7. Clear user cart and reset store affinity
		if params.CartID != uuid.Nil {
			_ = qtx.ClearCart(ctx, toPGUUID(params.CartID))
			_, _ = qtx.UpdateCartStore(ctx, db.UpdateCartStoreParams{
				ID:      toPGUUID(params.CartID),
				StoreID: pgtype.UUID{Valid: false},
			})
		}

		// 8. Record idempotency log
		if params.IdempotencyKey != "" {
			_, err = qtx.CreateIdempotencyRecord(ctx, db.CreateIdempotencyRecordParams{
				Key:         params.IdempotencyKey,
				UserID:      toPGUUID(params.UserID),
				RequestPath: "/api/v1/checkout",
				RequestHash: params.RequestHash,
				Status:      "COMPLETED",
				LockedUntil: pgtype.Timestamptz{Time: time.Now().Add(24 * time.Hour), Valid: true},
			})
			if err != nil {
				var pgErr *pgconn.PgError
				if errors.As(err, &pgErr) && pgErr.Code == "23505" {
					return ErrIdempotencyRaceConflict
				}
				// Non-fatal if idempotency table log fails as long as order succeeded
			}
		}

		finalOrder = &Order{
			ID:                 orderID,
			OrderNumber:        orderRow.OrderNumber,
			UserID:             params.UserID,
			StoreID:            params.StoreID,
			Status:             StatusPlaced,
			TotalAmountPaise:   totalPaise,
			DeliveryAddress:    params.DeliveryAddress,
			IdempotencyKey:     params.IdempotencyKey,
			Items:              orderItems,
			Reservations:       reservations,
			CreatedAt:          orderRow.CreatedAt.Time,
			UpdatedAt:          orderRow.UpdatedAt.Time,
		}

		return nil
	})

	if err != nil {
		return nil, err
	}

	return &CheckoutResult{
		Order:       finalOrder,
		IsDuplicate: false,
	}, nil
}

// CancelOrderTx atomically cancels an order, marks its reservations as RELEASED,
// and restores inventory back into available_quantity inside one ACID transaction.
func (r *pgxRepository) CancelOrderTx(ctx context.Context, orderID uuid.UUID, reason string, actorID *uuid.UUID) (*Order, error) {
	var cancelledOrder *Order

	err := database.WithTx(ctx, r.pool, func(tx pgx.Tx) error {
		qtx := r.queries.WithTx(tx)

		// 1. Atomically update status to CANCELLED guarded by status IN ('PLACED', 'CONFIRMED')
		row, err := qtx.CancelOrderAtomic(ctx, db.CancelOrderAtomicParams{
			ID:                 toPGUUID(orderID),
			CancellationReason: pgtype.Text{String: reason, Valid: true},
		})
		if err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				// Check current status to return accurate lifecycle error
				curr, qErr := qtx.GetOrderByID(ctx, toPGUUID(orderID))
				if qErr != nil {
					return ErrNotFound
				}
				if curr.Status == StatusCancelled {
					return ErrOrderAlreadyCancelled
				}
				return fmt.Errorf("%w: status %s cannot be cancelled", ErrOrderCannotBeCancelled, curr.Status)
			}
			return fmt.Errorf("failed to cancel order: %w", err)
		}

		// 2. Atomically release reservations guarded by status = 'RESERVED'
		releasedReservations, err := qtx.ReleaseReservationsByOrderID(ctx, toPGUUID(orderID))
		if err != nil {
			return fmt.Errorf("failed to release reservations: %w", err)
		}

		// 3. Atomically restore inventory for each released reservation
		for _, res := range releasedReservations {
			_, err := qtx.RestoreInventoryAtomic(ctx, db.RestoreInventoryAtomicParams{
				StoreID:           res.StoreID,
				ProductID:         res.ProductID,
				AvailableQuantity: res.Quantity,
			})
			if err != nil {
				return fmt.Errorf("failed to restore inventory for product %s: %w", fromPGUUID(res.ProductID), err)
			}
		}

		// 4. Audit Trail
		_, err = qtx.InsertOrderStatusHistory(ctx, db.InsertOrderStatusHistoryParams{
			OrderID:    toPGUUID(orderID),
			FromStatus: pgtype.Text{String: "PLACED/CONFIRMED", Valid: true},
			ToStatus:   StatusCancelled,
			ActorType:  "USER",
			ActorID:    toOptionalPGUUID(actorID),
			Notes:      pgtype.Text{String: reason, Valid: true},
		})
		if err != nil {
			return fmt.Errorf("failed to record status history: %w", err)
		}

		cancelledOrder = &Order{
			ID:                 fromPGUUID(row.ID),
			OrderNumber:        row.OrderNumber,
			UserID:             fromPGUUID(row.UserID),
			StoreID:            fromPGUUID(row.StoreID),
			Status:             StatusCancelled,
			TotalAmountPaise:   row.TotalAmountPaise,
			DeliveryAddress:    row.DeliveryAddress,
			IdempotencyKey:     row.IdempotencyKey.String,
			CancellationReason: row.CancellationReason.String,
			CreatedAt:          row.CreatedAt.Time,
			UpdatedAt:          row.UpdatedAt.Time,
		}

		return nil
	})

	if err != nil {
		return nil, err
	}

	return cancelledOrder, nil
}
