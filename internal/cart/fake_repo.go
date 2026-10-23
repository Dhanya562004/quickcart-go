package cart

import (
	"context"
	"sync"
	"time"

	"github.com/google/uuid"
)

type FakeRepository struct {
	mu        sync.RWMutex
	Carts     map[uuid.UUID]Cart
	UserCarts map[uuid.UUID]uuid.UUID // userID -> cartID
	Items     map[uuid.UUID][]CartItem
}

func NewFakeRepository() *FakeRepository {
	return &FakeRepository{
		Carts:     make(map[uuid.UUID]Cart),
		UserCarts: make(map[uuid.UUID]uuid.UUID),
		Items:     make(map[uuid.UUID][]CartItem),
	}
}

func (f *FakeRepository) GetCartByUserID(ctx context.Context, userID uuid.UUID) (*Cart, error) {
	f.mu.RLock()
	defer f.mu.RUnlock()

	cartID, ok := f.UserCarts[userID]
	if !ok {
		return nil, ErrNotFound
	}
	cart := f.Carts[cartID]
	return &cart, nil
}

func (f *FakeRepository) CreateCart(ctx context.Context, userID uuid.UUID, storeID *uuid.UUID) (*Cart, error) {
	f.mu.Lock()
	defer f.mu.Unlock()

	cartID := uuid.New()
	cart := Cart{
		ID:        cartID,
		UserID:    userID,
		StoreID:   storeID,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
	f.Carts[cartID] = cart
	f.UserCarts[userID] = cartID
	f.Items[cartID] = make([]CartItem, 0)
	return &cart, nil
}

func (f *FakeRepository) UpdateCartStore(ctx context.Context, cartID uuid.UUID, storeID *uuid.UUID) error {
	f.mu.Lock()
	defer f.mu.Unlock()

	cart, ok := f.Carts[cartID]
	if !ok {
		return ErrNotFound
	}
	cart.StoreID = storeID
	cart.UpdatedAt = time.Now()
	f.Carts[cartID] = cart
	return nil
}

func (f *FakeRepository) GetCartItems(ctx context.Context, cartID uuid.UUID) ([]CartItem, error) {
	f.mu.RLock()
	defer f.mu.RUnlock()

	items := f.Items[cartID]
	res := make([]CartItem, len(items))
	copy(res, items)
	return res, nil
}

func (f *FakeRepository) UpsertCartItem(ctx context.Context, cartID, productID uuid.UUID, quantity int32) (*CartItem, error) {
	f.mu.Lock()
	defer f.mu.Unlock()

	items := f.Items[cartID]
	for i, it := range items {
		if it.ProductID == productID {
			items[i].Quantity += quantity
			items[i].UpdatedAt = time.Now()
			items[i].TotalPricePaise = items[i].UnitPricePaise * int64(items[i].Quantity)
			f.Items[cartID] = items
			return &items[i], nil
		}
	}

	newItem := CartItem{
		ID:              uuid.New(),
		CartID:          cartID,
		ProductID:       productID,
		ProductName:     "Test Product",
		ProductSKU:      "SKU-TEST",
		ProductUnit:     "1pc",
		UnitPricePaise:  5000,
		Quantity:        quantity,
		TotalPricePaise: 5000 * int64(quantity),
		CreatedAt:       time.Now(),
		UpdatedAt:       time.Now(),
	}
	f.Items[cartID] = append(f.Items[cartID], newItem)
	return &newItem, nil
}

func (f *FakeRepository) UpdateCartItemQuantity(ctx context.Context, cartID, productID uuid.UUID, quantity int32) (*CartItem, error) {
	f.mu.Lock()
	defer f.mu.Unlock()

	items := f.Items[cartID]
	for i, it := range items {
		if it.ProductID == productID {
			items[i].Quantity = quantity
			items[i].UpdatedAt = time.Now()
			items[i].TotalPricePaise = items[i].UnitPricePaise * int64(quantity)
			f.Items[cartID] = items
			return &items[i], nil
		}
	}
	return nil, ErrNotFound
}

func (f *FakeRepository) DeleteCartItem(ctx context.Context, cartID, productID uuid.UUID) error {
	f.mu.Lock()
	defer f.mu.Unlock()

	items := f.Items[cartID]
	newItems := make([]CartItem, 0, len(items))
	for _, it := range items {
		if it.ProductID != productID {
			newItems = append(newItems, it)
		}
	}
	f.Items[cartID] = newItems
	return nil
}

func (f *FakeRepository) ClearCart(ctx context.Context, cartID uuid.UUID) error {
	f.mu.Lock()
	defer f.mu.Unlock()

	f.Items[cartID] = make([]CartItem, 0)
	return nil
}
