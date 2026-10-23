package order

import (
	"context"
	"fmt"
	"sort"
	"sync"
	"time"

	"github.com/Dhanya562004/quickcart-go/internal/cart"
	"github.com/Dhanya562004/quickcart-go/internal/db"
	"github.com/google/uuid"
)

type FakeRepository struct {
	mu          sync.Mutex
	Orders      map[uuid.UUID]Order
	OrderItems  map[uuid.UUID][]OrderItem
	Reservs     map[uuid.UUID][]Reservation
	UserOrders  map[uuid.UUID][]uuid.UUID
	IdempOrders map[string]uuid.UUID // userID_idempKey -> orderID
	IdempRecs   map[string]db.IdempotencyKey
	Inventory   map[string]int32 // storeID_productID -> available
	Reserved    map[string]int32 // storeID_productID -> reserved
}

func NewFakeRepository() *FakeRepository {
	return &FakeRepository{
		Orders:      make(map[uuid.UUID]Order),
		OrderItems:  make(map[uuid.UUID][]OrderItem),
		Reservs:     make(map[uuid.UUID][]Reservation),
		UserOrders:  make(map[uuid.UUID][]uuid.UUID),
		IdempOrders: make(map[string]uuid.UUID),
		IdempRecs:   make(map[string]db.IdempotencyKey),
		Inventory:   make(map[string]int32),
		Reserved:    make(map[string]int32),
	}
}

func (f *FakeRepository) SetStock(storeID, productID uuid.UUID, available int32) {
	f.mu.Lock()
	defer f.mu.Unlock()
	key := storeID.String() + "_" + productID.String()
	f.Inventory[key] = available
	f.Reserved[key] = 0
}

func (f *FakeRepository) GetStock(storeID, productID uuid.UUID) (available, reserved int32) {
	f.mu.Lock()
	defer f.mu.Unlock()
	key := storeID.String() + "_" + productID.String()
	return f.Inventory[key], f.Reserved[key]
}

func (f *FakeRepository) GetOrderByID(ctx context.Context, id uuid.UUID) (*Order, error) {
	f.mu.Lock()
	defer f.mu.Unlock()

	ord, ok := f.Orders[id]
	if !ok {
		return nil, ErrNotFound
	}
	ord.Items = f.OrderItems[id]
	ord.Reservations = f.Reservs[id]
	return &ord, nil
}

func (f *FakeRepository) GetOrderByUserAndIdempotencyKey(ctx context.Context, userID uuid.UUID, idempotencyKey string) (*Order, error) {
	f.mu.Lock()
	defer f.mu.Unlock()

	key := userID.String() + "_" + idempotencyKey
	orderID, ok := f.IdempOrders[key]
	if !ok {
		return nil, ErrNotFound
	}
	ord := f.Orders[orderID]
	ord.Items = f.OrderItems[orderID]
	ord.Reservations = f.Reservs[orderID]
	return &ord, nil
}

func (f *FakeRepository) ListOrdersByUserID(ctx context.Context, userID uuid.UUID, limit, offset int32) ([]Order, error) {
	f.mu.Lock()
	defer f.mu.Unlock()

	orderIDs := f.UserOrders[userID]
	res := make([]Order, 0, len(orderIDs))
	for _, id := range orderIDs {
		ord := f.Orders[id]
		ord.Items = f.OrderItems[id]
		res = append(res, ord)
	}
	return res, nil
}

func (f *FakeRepository) GetOrderItems(ctx context.Context, orderID uuid.UUID) ([]OrderItem, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.OrderItems[orderID], nil
}

func (f *FakeRepository) GetReservations(ctx context.Context, orderID uuid.UUID) ([]Reservation, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.Reservs[orderID], nil
}

func (f *FakeRepository) GetIdempotencyRecord(ctx context.Context, userID uuid.UUID, key string) (*db.IdempotencyKey, error) {
	f.mu.Lock()
	defer f.mu.Unlock()

	composite := userID.String() + "_" + key
	rec, ok := f.IdempRecs[composite]
	if !ok {
		return nil, ErrNotFound
	}
	return &rec, nil
}

func (f *FakeRepository) CheckoutTx(ctx context.Context, params CheckoutParams) (*CheckoutResult, error) {
	f.mu.Lock()
	defer f.mu.Unlock()

	// Check idempotency unique constraint
	if params.IdempotencyKey != "" {
		idempKey := params.UserID.String() + "_" + params.IdempotencyKey
		if _, exists := f.IdempOrders[idempKey]; exists {
			return nil, ErrIdempotencyRaceConflict
		}
	}

	// 1. Sort items by product ID
	sortedItems := make([]cart.CartItem, len(params.Items))
	copy(sortedItems, params.Items)
	sort.Slice(sortedItems, func(i, j int) bool {
		return sortedItems[i].ProductID.String() < sortedItems[j].ProductID.String()
	})

	// 2. Validate and reserve stock atomically
	for _, item := range sortedItems {
		invKey := params.StoreID.String() + "_" + item.ProductID.String()
		avail := f.Inventory[invKey]
		if avail < item.Quantity {
			return nil, &ErrInsufficientStock{
				ProductID:   item.ProductID,
				ProductName: item.ProductName,
				Requested:   item.Quantity,
				Available:   avail,
			}
		}
	}

	// Deduct available, increment reserved
	for _, item := range sortedItems {
		invKey := params.StoreID.String() + "_" + item.ProductID.String()
		f.Inventory[invKey] -= item.Quantity
		f.Reserved[invKey] += item.Quantity
	}

	var totalPaise int64
	for _, item := range sortedItems {
		totalPaise += item.TotalPricePaise
	}

	orderID := uuid.New()
	orderNumber := fmt.Sprintf("QC-TEST-%d", time.Now().UnixNano()%1000000)

	orderItems := make([]OrderItem, len(sortedItems))
	reservations := make([]Reservation, len(sortedItems))
	expiresAt := time.Now().Add(10 * time.Minute)

	for i, item := range sortedItems {
		orderItems[i] = OrderItem{
			ID:              uuid.New(),
			OrderID:         orderID,
			ProductID:       item.ProductID,
			ProductName:     item.ProductName,
			ProductSKU:      item.ProductSKU,
			ProductUnit:     item.ProductUnit,
			Quantity:        item.Quantity,
			UnitPricePaise:  item.UnitPricePaise,
			TotalPricePaise: item.TotalPricePaise,
			CreatedAt:       time.Now(),
		}
		reservations[i] = Reservation{
			ID:        uuid.New(),
			OrderID:   orderID,
			StoreID:   params.StoreID,
			ProductID: item.ProductID,
			Quantity:  item.Quantity,
			Status:    ReservationReserved,
			ExpiresAt: expiresAt,
			CreatedAt: time.Now(),
			UpdatedAt: time.Now(),
		}
	}

	ord := Order{
		ID:               orderID,
		OrderNumber:      orderNumber,
		UserID:           params.UserID,
		StoreID:          params.StoreID,
		Status:           StatusPlaced,
		TotalAmountPaise: totalPaise,
		DeliveryAddress:  params.DeliveryAddress,
		IdempotencyKey:   params.IdempotencyKey,
		Items:            orderItems,
		Reservations:     reservations,
		CreatedAt:        time.Now(),
		UpdatedAt:        time.Now(),
	}

	f.Orders[orderID] = ord
	f.OrderItems[orderID] = orderItems
	f.Reservs[orderID] = reservations
	f.UserOrders[params.UserID] = append(f.UserOrders[params.UserID], orderID)

	if params.IdempotencyKey != "" {
		idempKey := params.UserID.String() + "_" + params.IdempotencyKey
		f.IdempOrders[idempKey] = orderID
		f.IdempRecs[idempKey] = db.IdempotencyKey{
			ID:          toPGUUID(uuid.New()),
			Key:         params.IdempotencyKey,
			UserID:      toPGUUID(params.UserID),
			RequestPath: "/api/v1/checkout",
			RequestHash: params.RequestHash,
			Status:      "COMPLETED",
		}
	}

	return &CheckoutResult{
		Order:       &ord,
		IsDuplicate: false,
	}, nil
}

func (f *FakeRepository) CancelOrderTx(ctx context.Context, orderID uuid.UUID, reason string, actorID *uuid.UUID) (*Order, error) {
	f.mu.Lock()
	defer f.mu.Unlock()

	ord, ok := f.Orders[orderID]
	if !ok {
		return nil, ErrNotFound
	}

	if ord.Status == StatusCancelled {
		return nil, ErrOrderAlreadyCancelled
	}
	if !CanCancel(ord.Status) {
		return nil, fmt.Errorf("%w: status %s cannot be cancelled", ErrOrderCannotBeCancelled, ord.Status)
	}

	ord.Status = StatusCancelled
	ord.CancellationReason = reason
	ord.UpdatedAt = time.Now()
	f.Orders[orderID] = ord

	// Release reservations and restore inventory
	reservations := f.Reservs[orderID]
	for i, res := range reservations {
		if res.Status == ReservationReserved {
			reservations[i].Status = ReservationReleased
			reservations[i].UpdatedAt = time.Now()

			invKey := res.StoreID.String() + "_" + res.ProductID.String()
			f.Inventory[invKey] += res.Quantity
			f.Reserved[invKey] -= res.Quantity
		}
	}
	f.Reservs[orderID] = reservations

	ord.Reservations = reservations
	ord.Items = f.OrderItems[orderID]
	return &ord, nil
}
