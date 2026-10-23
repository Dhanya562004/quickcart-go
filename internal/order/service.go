package order

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"sort"
	"strings"

	"github.com/Dhanya562004/quickcart-go/internal/cart"
	"github.com/google/uuid"
)

var (
	ErrEmptyIdempotencyKey = errors.New("idempotency-key header is required for checkout")
	ErrCartEmpty           = errors.New("cannot checkout an empty cart")
	ErrCartMissingStore    = errors.New("cart has no assigned dark store")
	ErrUnauthorizedOrder   = errors.New("user is not authorized to access this order")
)

type Service interface {
	Checkout(ctx context.Context, userID uuid.UUID, idempotencyKey string, req CheckoutRequest) (*Order, error)
	CancelOrder(ctx context.Context, userID, orderID uuid.UUID, req CancelOrderRequest) (*Order, error)
	GetOrder(ctx context.Context, userID, orderID uuid.UUID) (*Order, error)
	ListOrders(ctx context.Context, userID uuid.UUID, limit, offset int32) ([]Order, error)
}

type service struct {
	repo    Repository
	cartSvc cart.Service
}

func NewService(repo Repository, cartSvc cart.Service) Service {
	return &service{
		repo:    repo,
		cartSvc: cartSvc,
	}
}

// computeRequestHash generates a deterministic SHA-256 fingerprint of the checkout payload.
func computeRequestHash(address string, items []cart.CartItem) string {
	sorted := make([]string, len(items))
	for i, it := range items {
		sorted[i] = fmt.Sprintf("%s:%d:%d", it.ProductID.String(), it.Quantity, it.UnitPricePaise)
	}
	sort.Strings(sorted)

	payload := fmt.Sprintf("addr=%s|items=%s", strings.TrimSpace(address), strings.Join(sorted, ","))
	h := sha256.Sum256([]byte(payload))
	return hex.EncodeToString(h[:])
}

func (s *service) Checkout(ctx context.Context, userID uuid.UUID, idempotencyKey string, req CheckoutRequest) (*Order, error) {
	if strings.TrimSpace(idempotencyKey) == "" {
		return nil, ErrEmptyIdempotencyKey
	}

	// 1. Fetch current cart
	userCart, err := s.cartSvc.GetCart(ctx, userID)
	if err != nil {
		return nil, err
	}
	if len(userCart.Items) == 0 {
		// Before rejecting empty cart, check if an existing order was already created
		// with this idempotency key (e.g., retried after cart clearance).
		existingOrder, qErr := s.repo.GetOrderByUserAndIdempotencyKey(ctx, userID, idempotencyKey)
		if qErr == nil && existingOrder != nil {
			if existingOrder.DeliveryAddress != req.DeliveryAddress {
				return nil, ErrIdempotencyPayloadMismatch
			}
			return existingOrder, nil
		}
		return nil, ErrCartEmpty
	}
	if userCart.StoreID == nil {
		return nil, ErrCartMissingStore
	}

	reqHash := computeRequestHash(req.DeliveryAddress, userCart.Items)

	// 2. Check existing idempotency record
	record, err := s.repo.GetIdempotencyRecord(ctx, userID, idempotencyKey)
	if err == nil && record != nil {
		if record.RequestHash != reqHash {
			return nil, ErrIdempotencyPayloadMismatch
		}
		// Return original order
		existingOrder, qErr := s.repo.GetOrderByUserAndIdempotencyKey(ctx, userID, idempotencyKey)
		if qErr == nil && existingOrder != nil {
			return existingOrder, nil
		}
	}

	// 3. Execute atomic checkout transaction
	res, err := s.repo.CheckoutTx(ctx, CheckoutParams{
		UserID:          userID,
		CartID:          userCart.ID,
		StoreID:         *userCart.StoreID,
		DeliveryAddress: req.DeliveryAddress,
		IdempotencyKey:  idempotencyKey,
		RequestHash:     reqHash,
		Items:           userCart.Items,
	})
	if err != nil {
		// 4. Handle concurrent race condition where duplicate request committed first
		if errors.Is(err, ErrIdempotencyRaceConflict) {
			winnerOrder, fetchErr := s.repo.GetOrderByUserAndIdempotencyKey(ctx, userID, idempotencyKey)
			if fetchErr == nil && winnerOrder != nil {
				if winnerOrder.DeliveryAddress != req.DeliveryAddress {
					return nil, ErrIdempotencyPayloadMismatch
				}
				return winnerOrder, nil
			}
			return nil, ErrIdempotencyRaceConflict
		}
		return nil, err
	}

	return res.Order, nil
}

func (s *service) CancelOrder(ctx context.Context, userID, orderID uuid.UUID, req CancelOrderRequest) (*Order, error) {
	ord, err := s.repo.GetOrderByID(ctx, orderID)
	if err != nil {
		return nil, err
	}

	// Ensure user can only cancel their own order
	if ord.UserID != userID {
		return nil, ErrUnauthorizedOrder
	}

	if !CanCancel(ord.Status) {
		if ord.Status == StatusCancelled {
			return nil, ErrOrderAlreadyCancelled
		}
		return nil, ErrOrderCannotBeCancelled
	}

	return s.repo.CancelOrderTx(ctx, orderID, req.Reason, &userID)
}

func (s *service) GetOrder(ctx context.Context, userID, orderID uuid.UUID) (*Order, error) {
	ord, err := s.repo.GetOrderByID(ctx, orderID)
	if err != nil {
		return nil, err
	}

	if ord.UserID != userID {
		return nil, ErrUnauthorizedOrder
	}

	return ord, nil
}

func (s *service) ListOrders(ctx context.Context, userID uuid.UUID, limit, offset int32) ([]Order, error) {
	if limit <= 0 || limit > 100 {
		limit = 20
	}
	if offset < 0 {
		offset = 0
	}
	return s.repo.ListOrdersByUserID(ctx, userID, limit, offset)
}
