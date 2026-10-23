package cart

import (
	"time"

	"github.com/google/uuid"
)

// Cart represents a user's shopping basket bounded to a single dark store.
type Cart struct {
	ID               uuid.UUID  `json:"id"`
	UserID           uuid.UUID  `json:"user_id"`
	StoreID          *uuid.UUID `json:"store_id,omitempty"`
	Items            []CartItem `json:"items"`
	TotalAmountPaise int64      `json:"total_amount_paise"`
	CreatedAt        time.Time  `json:"created_at"`
	UpdatedAt        time.Time  `json:"updated_at"`
}

// CartItem represents an individual product line in a cart.
type CartItem struct {
	ID              uuid.UUID `json:"id"`
	CartID          uuid.UUID `json:"cart_id"`
	ProductID       uuid.UUID `json:"product_id"`
	ProductName     string    `json:"product_name"`
	ProductSKU      string    `json:"product_sku"`
	ProductUnit     string    `json:"product_unit"`
	UnitPricePaise  int64     `json:"unit_price_paise"`
	Quantity        int32     `json:"quantity"`
	TotalPricePaise int64     `json:"total_price_paise"`
	CreatedAt       time.Time `json:"created_at"`
	UpdatedAt       time.Time `json:"updated_at"`
}

// AddItemRequest defines payload to add a product to the cart.
type AddItemRequest struct {
	StoreID   uuid.UUID `json:"store_id" binding:"required"`
	ProductID uuid.UUID `json:"product_id" binding:"required"`
	Quantity  int32     `json:"quantity" binding:"required,gt=0"`
}

// UpdateQuantityRequest defines payload to alter quantity of a cart line item.
type UpdateQuantityRequest struct {
	Quantity int32 `json:"quantity" binding:"required,gte=0"`
}
