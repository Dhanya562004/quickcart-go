package order

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
	rg.POST("/checkout", h.Checkout)
	orders := rg.Group("/orders")
	{
		orders.GET("", h.ListOrders)
		orders.GET("/:id", h.GetOrder)
		orders.POST("/:id/cancel", h.CancelOrder)
	}
}

func getUserID(c *gin.Context) (uuid.UUID, error) {
	header := c.GetHeader("X-User-ID")
	if header == "" {
		return uuid.Nil, errors.New("missing X-User-ID header")
	}
	u, err := uuid.Parse(header)
	if err != nil {
		return uuid.Nil, errors.New("invalid X-User-ID header UUID format")
	}
	return u, nil
}

func (h *Handler) Checkout(c *gin.Context) {
	userID, err := getUserID(c)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	idempotencyKey := c.GetHeader("Idempotency-Key")
	if idempotencyKey == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": ErrEmptyIdempotencyKey.Error()})
		return
	}

	var req CheckoutRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	ord, err := h.svc.Checkout(c.Request.Context(), userID, idempotencyKey, req)
	if err != nil {
		var stockErr *ErrInsufficientStock
		if errors.As(err, &stockErr) {
			c.JSON(http.StatusConflict, gin.H{
				"error":        stockErr.Error(),
				"product_id":   stockErr.ProductID,
				"product_name": stockErr.ProductName,
				"requested":    stockErr.Requested,
				"available":    stockErr.Available,
			})
			return
		}
		if errors.Is(err, ErrIdempotencyPayloadMismatch) {
			c.JSON(http.StatusUnprocessableEntity, gin.H{"error": err.Error()})
			return
		}
		if errors.Is(err, ErrCartEmpty) || errors.Is(err, ErrCartMissingStore) {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusCreated, ord)
}

func (h *Handler) GetOrder(c *gin.Context) {
	userID, err := getUserID(c)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	orderID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid order_id UUID"})
		return
	}

	ord, err := h.svc.GetOrder(c.Request.Context(), userID, orderID)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": "order not found"})
			return
		}
		if errors.Is(err, ErrUnauthorizedOrder) {
			c.JSON(http.StatusForbidden, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, ord)
}

func (h *Handler) ListOrders(c *gin.Context) {
	userID, err := getUserID(c)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
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

	orders, err := h.svc.ListOrders(c.Request.Context(), userID, limit, offset)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, orders)
}

func (h *Handler) CancelOrder(c *gin.Context) {
	userID, err := getUserID(c)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	orderID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid order_id UUID"})
		return
	}

	var req CancelOrderRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	ord, err := h.svc.CancelOrder(c.Request.Context(), userID, orderID, req)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": "order not found"})
			return
		}
		if errors.Is(err, ErrUnauthorizedOrder) {
			c.JSON(http.StatusForbidden, gin.H{"error": err.Error()})
			return
		}
		if errors.Is(err, ErrOrderAlreadyCancelled) {
			c.JSON(http.StatusConflict, gin.H{"error": err.Error()})
			return
		}
		if errors.Is(err, ErrOrderCannotBeCancelled) {
			c.JSON(http.StatusUnprocessableEntity, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, ord)
}
