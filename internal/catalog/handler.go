package catalog

import (
	"errors"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

type Handler struct {
	svc Service
}

func NewHandler(svc Service) *Handler {
	return &Handler{svc: svc}
}

func (h *Handler) RegisterRoutes(rg *gin.RouterGroup) {
	// Public Catalog Routes
	rg.GET("/categories", h.ListCategories)
	rg.GET("/categories/:id", h.GetCategory)
	rg.GET("/stores", h.ListDarkStores)
	rg.GET("/stores/:store_id/products", h.ListProductsByStore)
	rg.GET("/products/:id", h.GetProduct)

	// Admin-Only Routes
	admin := rg.Group("/admin", requireAdminMiddleware())
	{
		admin.POST("/categories", h.CreateCategory)
		admin.POST("/products", h.CreateProduct)
		admin.PUT("/products/:id", h.UpdateProduct)
		admin.POST("/stores", h.CreateDarkStore)
		admin.POST("/stores/:store_id/inventory", h.UpsertStoreInventory)
	}
}

func requireAdminMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		role := c.GetHeader("X-User-Role")
		if role != "admin" {
			c.AbortWithStatusJSON(http.StatusForbidden, gin.H{
				"error": "admin authorization required",
			})
			return
		}
		c.Next()
	}
}

func (h *Handler) ListCategories(c *gin.Context) {
	cats, err := h.svc.ListCategories(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, cats)
}

func (h *Handler) GetCategory(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid category uuid"})
		return
	}
	cat, err := h.svc.GetCategory(c.Request.Context(), id)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": "category not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, cat)
}

func (h *Handler) CreateCategory(c *gin.Context) {
	var req CreateCategoryRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	cat, err := h.svc.CreateCategory(c.Request.Context(), req)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusCreated, cat)
}

func (h *Handler) CreateProduct(c *gin.Context) {
	var req CreateProductRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	prod, err := h.svc.CreateProduct(c.Request.Context(), req)
	if err != nil {
		if errors.Is(err, ErrInvalidPrice) {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		if errors.Is(err, ErrNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusCreated, prod)
}

func (h *Handler) GetProduct(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid product uuid"})
		return
	}
	prod, err := h.svc.GetProduct(c.Request.Context(), id)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": "product not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, prod)
}

func (h *Handler) UpdateProduct(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid product uuid"})
		return
	}
	var req UpdateProductRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	prod, err := h.svc.UpdateProduct(c.Request.Context(), id, req)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": "product not found"})
			return
		}
		if errors.Is(err, ErrInvalidPrice) {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, prod)
}

func (h *Handler) CreateDarkStore(c *gin.Context) {
	var req CreateDarkStoreRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	store, err := h.svc.CreateDarkStore(c.Request.Context(), req)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusCreated, store)
}

func (h *Handler) ListDarkStores(c *gin.Context) {
	stores, err := h.svc.ListDarkStores(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, stores)
}

func (h *Handler) ListProductsByStore(c *gin.Context) {
	storeID, err := uuid.Parse(c.Param("store_id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid store_id uuid"})
		return
	}

	limit := int32(20)
	offset := int32(0)
	if l := c.Query("limit"); l != "" {
		if parsed, err := strconv.Atoi(l); err == nil && parsed > 0 {
			limit = int32(parsed)
		}
	}
	if o := c.Query("offset"); o != "" {
		if parsed, err := strconv.Atoi(o); err == nil && parsed >= 0 {
			offset = int32(parsed)
		}
	}

	items, err := h.svc.ListProductsByStore(c.Request.Context(), storeID, limit, offset)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": "store not found"})
			return
		}
		if errors.Is(err, ErrStoreInactive) {
			c.JSON(http.StatusUnprocessableEntity, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, items)
}

func (h *Handler) UpsertStoreInventory(c *gin.Context) {
	storeID, err := uuid.Parse(c.Param("store_id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid store_id uuid"})
		return
	}
	productID, err := uuid.Parse(c.Query("product_id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "missing or invalid product_id query param"})
		return
	}

	var req UpsertInventoryRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	inv, err := h.svc.UpsertStoreInventory(c.Request.Context(), storeID, productID, req.AvailableQuantity)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
			return
		}
		if errors.Is(err, ErrInvalidQuantity) {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, inv)
}
