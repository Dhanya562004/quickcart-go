package tracking

import (
	"context"
	"errors"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"github.com/Dhanya562004/quickcart-go/internal/auth"
	"github.com/Dhanya562004/quickcart-go/internal/order"
)

// OrderLookup abstracts fetching an order to verify ownership before opening the event stream.
type OrderLookup interface {
	GetOrderByID(ctx context.Context, id uuid.UUID) (*order.Order, error)
}

// Handler handles live SSE order tracking streams.
type Handler struct {
	hub               *SSEHub
	orderLookup       OrderLookup
	heartbeatInterval time.Duration
}

// NewHandler constructs an SSE handler.
func NewHandler(hub *SSEHub, orderLookup OrderLookup, heartbeatInterval time.Duration) *Handler {
	if heartbeatInterval <= 0 {
		heartbeatInterval = 15 * time.Second
	}
	return &Handler{
		hub:               hub,
		orderLookup:       orderLookup,
		heartbeatInterval: heartbeatInterval,
	}
}

// RegisterRoutes registers the SSE streaming endpoint on the provided router group.
func (h *Handler) RegisterRoutes(rg *gin.RouterGroup, authMiddleware ...gin.HandlerFunc) {
	handlers := append(authMiddleware, h.StreamOrderEvents)
	rg.GET("/orders/:id/events", handlers...)
}

// StreamOrderEvents handles GET /api/v1/orders/:id/events.
// Guarantees:
// 1. Order ownership verification: returns HTTP 403 if requester does not own the order and is not admin.
//    Derives caller identity strictly from authenticated JWT context, rejecting spoofed headers/params.
// 2. Regular heartbeats every 15s to keep proxy connections (Nginx, AWS ALB) from terminating idle streams.
// 3. Clean exit on disconnect: when client closes connection, c.Request.Context().Done() unregisters
//    the subscriber channel and terminates the handler goroutine immediately without leaking.
func (h *Handler) StreamOrderEvents(c *gin.Context) {
	orderIDStr := c.Param("id")
	orderID, err := uuid.Parse(orderIDStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid order ID format"})
		return
	}

	// 1. Verify order exists
	ord, err := h.orderLookup.GetOrderByID(c.Request.Context(), orderID)
	if err != nil {
		if errors.Is(err, order.ErrNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": "order not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to retrieve order"})
		return
	}

	// 2. Verify server-side authenticated identity (RBAC & ownership)
	callerID, ok := auth.GetUserID(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized; authentication required"})
		return
	}
	callerRole := auth.GetUserRole(c)

	if callerRole != auth.RoleAdmin && callerID != ord.UserID {
		c.JSON(http.StatusForbidden, gin.H{"error": "forbidden; you do not have permission to view this order stream"})
		return
	}

	// 3. Set SSE response headers
	c.Writer.Header().Set("Content-Type", "text/event-stream")
	c.Writer.Header().Set("Cache-Control", "no-cache, no-transform")
	c.Writer.Header().Set("Connection", "keep-alive")
	c.Writer.Header().Set("X-Accel-Buffering", "no")

	// 4. Subscribe to domain hub
	eventCh := h.hub.Subscribe(orderID)
	defer h.hub.Unsubscribe(orderID, eventCh)

	// Send initial connection event with current status
	c.SSEvent("order_status", OrderEvent{
		OrderID:   ord.ID,
		Status:    string(ord.Status),
		Message:   "Connected to live order tracking",
		Timestamp: time.Now().UTC(),
	})
	c.Writer.Flush()

	ticker := time.NewTicker(h.heartbeatInterval)
	defer ticker.Stop()

	for {
		select {
		case <-c.Request.Context().Done():
			// Client disconnected; unregister and terminate stream cleanly
			return

		case event, ok := <-eventCh:
			if !ok {
				return
			}
			c.SSEvent("order_status", event)
			c.Writer.Flush()

		case t := <-ticker.C:
			// Heartbeat keepalive for reverse proxies
			c.SSEvent("heartbeat", gin.H{"timestamp": t.UTC()})
			c.Writer.Flush()
		}
	}
}
