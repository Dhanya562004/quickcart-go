package cart

import (
	"errors"
	"net/http"

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
	cart := rg.Group("/cart")
	{
		cart.GET("", h.GetCart)
		cart.POST("/items", h.AddItem)
		cart.PATCH("/items/:product_id", h.UpdateQuantity)
		cart.DELETE("/items/:product_id", h.RemoveItem)
		cart.DELETE("", h.ClearCart)
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

func (h *Handler) GetCart(c *gin.Context) {
	userID, err := getUserID(c)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	cart, err := h.svc.GetCart(c.Request.Context(), userID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, cart)
}

func (h *Handler) AddItem(c *gin.Context) {
	userID, err := getUserID(c)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	var req AddItemRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	cart, err := h.svc.AddItem(c.Request.Context(), userID, req)
	if err != nil {
		if errors.Is(err, ErrCrossStoreConflict) {
			c.JSON(http.StatusConflict, gin.H{"error": err.Error()})
			return
		}
		if errors.Is(err, ErrInvalidQuantity) {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, cart)
}

func (h *Handler) UpdateQuantity(c *gin.Context) {
	userID, err := getUserID(c)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	productID, err := uuid.Parse(c.Param("product_id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid product_id UUID"})
		return
	}

	var req UpdateQuantityRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	cart, err := h.svc.UpdateItemQuantity(c.Request.Context(), userID, productID, req.Quantity)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, cart)
}

func (h *Handler) RemoveItem(c *gin.Context) {
	userID, err := getUserID(c)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	productID, err := uuid.Parse(c.Param("product_id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid product_id UUID"})
		return
	}

	cart, err := h.svc.RemoveItem(c.Request.Context(), userID, productID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, cart)
}

func (h *Handler) ClearCart(c *gin.Context) {
	userID, err := getUserID(c)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if err := h.svc.ClearCart(c.Request.Context(), userID); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "cart cleared successfully"})
}
