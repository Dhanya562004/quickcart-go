package order

import (
	"time"

	"github.com/google/uuid"
)

// Order status constants
const (
	StatusPlaced         = "PLACED"
	StatusConfirmed      = "CONFIRMED"
	StatusPacked         = "PACKED"
	StatusOutForDelivery = "OUT_FOR_DELIVERY"
	StatusDelivered      = "DELIVERED"
	StatusCancelled      = "CANCELLED"
	StatusFailed         = "FAILED"
)

// Reservation status constants
const (
	ReservationReserved  = "RESERVED"
	ReservationConfirmed = "CONFIRMED"
	ReservationReleased  = "RELEASED"
	ReservationExpired   = "EXPIRED"
)

// Order represents an e-commerce order with its fulfillment status.
type Order struct {
	ID                 uuid.UUID     `json:"id"`
	OrderNumber        string        `json:"order_number"`
	UserID             uuid.UUID     `json:"user_id"`
	StoreID            uuid.UUID     `json:"store_id"`
	Status             string        `json:"status"`
	TotalAmountPaise   int64         `json:"total_amount_paise"`
	DeliveryAddress    string        `json:"delivery_address"`
	IdempotencyKey     string        `json:"idempotency_key,omitempty"`
	CancellationReason string        `json:"cancellation_reason,omitempty"`
	Items              []OrderItem   `json:"items,omitempty"`
	Reservations       []Reservation `json:"reservations,omitempty"`
	CreatedAt          time.Time     `json:"created_at"`
	UpdatedAt          time.Time     `json:"updated_at"`
}

// OrderItem represents a line item in an order.
type OrderItem struct {
	ID              uuid.UUID `json:"id"`
	OrderID         uuid.UUID `json:"order_id"`
	ProductID       uuid.UUID `json:"product_id"`
	ProductName     string    `json:"product_name"`
	ProductSKU      string    `json:"product_sku"`
	ProductUnit     string    `json:"product_unit"`
	Quantity        int32     `json:"quantity"`
	UnitPricePaise  int64     `json:"unit_price_paise"`
	TotalPricePaise int64     `json:"total_price_paise"`
	CreatedAt       time.Time `json:"created_at"`
}

// Reservation represents inventory temporarily locked for an in-flight order.
type Reservation struct {
	ID        uuid.UUID `json:"id"`
	OrderID   uuid.UUID `json:"order_id"`
	StoreID   uuid.UUID `json:"store_id"`
	ProductID uuid.UUID `json:"product_id"`
	Quantity  int32     `json:"quantity"`
	Status    string    `json:"status"`
	ExpiresAt time.Time `json:"expires_at"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// CheckoutRequest defines the payload for creating an order via checkout.
type CheckoutRequest struct {
	DeliveryAddress string `json:"delivery_address" binding:"required,min=5,max=500"`
}

// CancelOrderRequest defines the payload for cancelling an order.
type CancelOrderRequest struct {
	Reason string `json:"reason" binding:"required,min=3,max=255"`
}
