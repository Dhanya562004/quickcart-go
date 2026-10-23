package cart

import (
	"context"
	"errors"
	"fmt"
	"sync"

	"github.com/google/uuid"
)

var (
	ErrInvalidQuantity    = errors.New("item quantity must be greater than zero")
	ErrCrossStoreConflict = errors.New("cart already belongs to another dark store; clear cart to switch stores")
	ErrCartEmpty          = errors.New("cart is empty")
)

type Service interface {
	GetCart(ctx context.Context, userID uuid.UUID) (*Cart, error)
	AddItem(ctx context.Context, userID uuid.UUID, req AddItemRequest) (*Cart, error)
	UpdateItemQuantity(ctx context.Context, userID, productID uuid.UUID, quantity int32) (*Cart, error)
	RemoveItem(ctx context.Context, userID, productID uuid.UUID) (*Cart, error)
	ClearCart(ctx context.Context, userID uuid.UUID) error
}

type service struct {
	repo Repository
	// Mutex per user to prevent concurrent race conditions on the same cart
	userLocks sync.Map
}

func NewService(repo Repository) Service {
	return &service{
		repo: repo,
	}
}

func (s *service) getLock(userID uuid.UUID) *sync.Mutex {
	val, _ := s.userLocks.LoadOrStore(userID, &sync.Mutex{})
	return val.(*sync.Mutex)
}

func (s *service) GetCart(ctx context.Context, userID uuid.UUID) (*Cart, error) {
	cart, err := s.repo.GetCartByUserID(ctx, userID)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			// Initialize cart for first-time customer
			cart, err = s.repo.CreateCart(ctx, userID, nil)
			if err != nil {
				return nil, err
			}
		} else {
			return nil, err
		}
	}

	items, err := s.repo.GetCartItems(ctx, cart.ID)
	if err != nil {
		return nil, err
	}

	cart.Items = items
	var total int64
	for _, it := range items {
		total += it.TotalPricePaise
	}
	cart.TotalAmountPaise = total

	return cart, nil
}

func (s *service) AddItem(ctx context.Context, userID uuid.UUID, req AddItemRequest) (*Cart, error) {
	if req.Quantity <= 0 {
		return nil, ErrInvalidQuantity
	}

	lock := s.getLock(userID)
	lock.Lock()
	defer lock.Unlock()

	cart, err := s.GetCart(ctx, userID)
	if err != nil {
		return nil, err
	}

	// Rule: A cart belongs to exactly one dark store.
	// Adding items from another dark store returns 409 Conflict.
	if cart.StoreID != nil && len(cart.Items) > 0 && *cart.StoreID != req.StoreID {
		return nil, fmt.Errorf("%w: existing store %s, requested store %s", ErrCrossStoreConflict, *cart.StoreID, req.StoreID)
	}

	// Update cart store if empty or unassigned
	if cart.StoreID == nil || len(cart.Items) == 0 {
		if err := s.repo.UpdateCartStore(ctx, cart.ID, &req.StoreID); err != nil {
			return nil, err
		}
		cart.StoreID = &req.StoreID
	}

	if _, err := s.repo.UpsertCartItem(ctx, cart.ID, req.ProductID, req.Quantity); err != nil {
		return nil, err
	}

	return s.GetCart(ctx, userID)
}

func (s *service) UpdateItemQuantity(ctx context.Context, userID, productID uuid.UUID, quantity int32) (*Cart, error) {
	lock := s.getLock(userID)
	lock.Lock()
	defer lock.Unlock()

	cart, err := s.GetCart(ctx, userID)
	if err != nil {
		return nil, err
	}

	if quantity <= 0 {
		if err := s.repo.DeleteCartItem(ctx, cart.ID, productID); err != nil {
			return nil, err
		}
	} else {
		if _, err := s.repo.UpdateCartItemQuantity(ctx, cart.ID, productID, quantity); err != nil {
			return nil, err
		}
	}

	// Refresh cart items
	updatedCart, err := s.GetCart(ctx, userID)
	if err != nil {
		return nil, err
	}

	// If cart is now completely empty, reset store affinity
	if len(updatedCart.Items) == 0 && updatedCart.StoreID != nil {
		_ = s.repo.UpdateCartStore(ctx, cart.ID, nil)
		updatedCart.StoreID = nil
	}

	return updatedCart, nil
}

func (s *service) RemoveItem(ctx context.Context, userID, productID uuid.UUID) (*Cart, error) {
	return s.UpdateItemQuantity(ctx, userID, productID, 0)
}

func (s *service) ClearCart(ctx context.Context, userID uuid.UUID) error {
	lock := s.getLock(userID)
	lock.Lock()
	defer lock.Unlock()

	cart, err := s.GetCart(ctx, userID)
	if err != nil {
		return err
	}

	if err := s.repo.ClearCart(ctx, cart.ID); err != nil {
		return err
	}

	return s.repo.UpdateCartStore(ctx, cart.ID, nil)
}
