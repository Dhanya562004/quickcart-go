package catalog

import (
	"time"

	"github.com/google/uuid"
)

// Category represents a product category taxonomy.
type Category struct {
	ID          uuid.UUID `json:"id"`
	Name        string    `json:"name"`
	Slug        string    `json:"slug"`
	Description string    `json:"description,omitempty"`
	CreatedAt   time.Time `json:"created_at"`
}

// Product represents a catalog SKU with prices in paise.
type Product struct {
	ID          uuid.UUID `json:"id"`
	CategoryID  uuid.UUID `json:"category_id"`
	SKU         string    `json:"sku"`
	Name        string    `json:"name"`
	Description string    `json:"description,omitempty"`
	Unit        string    `json:"unit"`
	PricePaise  int64     `json:"price_paise"`
	IsActive    bool      `json:"is_active"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

// DarkStore represents a localized micro-fulfillment center.
type DarkStore struct {
	ID              uuid.UUID `json:"id"`
	StoreCode       string    `json:"store_code"`
	Name            string    `json:"name"`
	Address         string    `json:"address"`
	Latitude        float64   `json:"latitude"`
	Longitude       float64   `json:"longitude"`
	IsActive        bool      `json:"is_active"`
	MaxActiveOrders int32     `json:"max_active_orders"`
	CreatedAt       time.Time `json:"created_at"`
	UpdatedAt       time.Time `json:"updated_at"`
}

// StoreProduct represents a catalog product with its current localized store inventory.
type StoreProduct struct {
	Product
	StoreID           uuid.UUID `json:"store_id"`
	AvailableQuantity int32     `json:"available_quantity"`
	ReservedQuantity  int32     `json:"reserved_quantity"`
}

// StoreInventory represents stock counts for a specific store and product tuple.
type StoreInventory struct {
	StoreID           uuid.UUID `json:"store_id"`
	ProductID         uuid.UUID `json:"product_id"`
	AvailableQuantity int32     `json:"available_quantity"`
	ReservedQuantity  int32     `json:"reserved_quantity"`
	Version           int32     `json:"version"`
	CreatedAt         time.Time `json:"created_at"`
	UpdatedAt         time.Time `json:"updated_at"`
}

// CreateCategoryRequest defines the payload for creating a category.
type CreateCategoryRequest struct {
	Name        string `json:"name" binding:"required,min=2,max=100"`
	Slug        string `json:"slug" binding:"required,min=2,max=100"`
	Description string `json:"description"`
}

// CreateProductRequest defines the payload for admin product creation.
type CreateProductRequest struct {
	CategoryID  uuid.UUID `json:"category_id" binding:"required"`
	SKU         string    `json:"sku" binding:"required,min=2,max=64"`
	Name        string    `json:"name" binding:"required,min=2,max=255"`
	Description string    `json:"description"`
	Unit        string    `json:"unit" binding:"required,min=1,max=32"`
	PricePaise  int64     `json:"price_paise" binding:"required,gte=0"`
	IsActive    *bool     `json:"is_active"`
}

// UpdateProductRequest defines the payload for admin product modification.
type UpdateProductRequest struct {
	CategoryID  *uuid.UUID `json:"category_id"`
	Name        *string    `json:"name"`
	Description *string    `json:"description"`
	Unit        *string    `json:"unit"`
	PricePaise  *int64     `json:"price_paise"`
	IsActive    *bool      `json:"is_active"`
}

// CreateDarkStoreRequest defines the payload for dark store provisioning.
type CreateDarkStoreRequest struct {
	StoreCode       string  `json:"store_code" binding:"required,min=2,max=32"`
	Name            string  `json:"name" binding:"required,min=2,max=100"`
	Address         string  `json:"address" binding:"required"`
	Latitude        float64 `json:"latitude" binding:"required"`
	Longitude       float64 `json:"longitude" binding:"required"`
	IsActive        *bool   `json:"is_active"`
	MaxActiveOrders int32   `json:"max_active_orders" binding:"required,gt=0"`
}

// UpsertInventoryRequest defines the payload to set stock for a store SKU.
type UpsertInventoryRequest struct {
	AvailableQuantity int32 `json:"available_quantity" binding:"required,gte=0"`
}
