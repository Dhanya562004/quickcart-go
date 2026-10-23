package payment

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"net/http"

	"github.com/Dhanya562004/quickcart-go/internal/auth"
	"github.com/Dhanya562004/quickcart-go/internal/order"
	"github.com/Dhanya562004/quickcart-go/internal/platform/metrics"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

type PayOrderRequest struct {
	Method PaymentMethod `json:"method"`
}

type RefundOrderRequest struct {
	Reason string `json:"reason"`
}

type WebhookPayload struct {
	Event       string `json:"event"`
	OrderID     string `json:"order_id"`
	PaymentID   string `json:"payment_id"`
	Status      string `json:"status"`
	ReferenceID string `json:"reference_id"`
}

type Handler struct {
	svc           *Service
	orderRepo     order.Repository
	webhookSecret string
}

func NewHandler(svc *Service, orderRepo order.Repository, webhookSecret string) *Handler {
	return &Handler{
		svc:           svc,
		orderRepo:     orderRepo,
		webhookSecret: webhookSecret,
	}
}

func (h *Handler) RegisterRoutes(rg *gin.RouterGroup, authMiddleware gin.HandlerFunc, adminMiddleware gin.HandlerFunc) {
	// Customer payment on own order (or admin)
	rg.POST("/orders/:id/pay", authMiddleware, h.PayOrder)

	// Admin-only refund operation
	rg.POST("/orders/:id/refund", authMiddleware, adminMiddleware, h.RefundOrder)

	// Authenticated Payment Gateway Webhook Callback
	rg.POST("/payments/webhook", h.HandleWebhook)
}

// PayOrder processes an authorized payment for an existing placed order.
func (h *Handler) PayOrder(c *gin.Context) {
	userID, ok := auth.GetUserID(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized; authentication required"})
		return
	}
	role := auth.GetUserRole(c)

	orderID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid order_id UUID"})
		return
	}

	ord, err := h.orderRepo.GetOrderByID(c.Request.Context(), orderID)
	if err != nil {
		if errors.Is(err, order.ErrNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": "order not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	// Verify order ownership: only the owner or an admin may submit payment
	if role != auth.RoleAdmin && ord.UserID != userID {
		c.JSON(http.StatusForbidden, gin.H{"error": "forbidden; cannot pay for another user's order"})
		return
	}

	var req PayOrderRequest
	_ = c.ShouldBindJSON(&req)
	if req.Method == "" {
		req.Method = MethodUPI
	}

	paymentRes, err := h.svc.ProcessPayment(c.Request.Context(), ProcessPaymentRequest{
		OrderID:     ord.ID,
		AmountPaise: ord.TotalAmountPaise,
		Currency:    "INR",
		Method:      req.Method,
	})
	if err != nil {
		metrics.RecordPaymentAttempt("fail", string(req.Method))
		c.JSON(http.StatusBadGateway, gin.H{
			"error":  "payment processing failed",
			"detail": err.Error(),
		})
		return
	}

	metrics.RecordPaymentAttempt("success", string(req.Method))
	c.JSON(http.StatusOK, paymentRes)
}

// RefundOrder executes an administrative refund on an existing payment.
func (h *Handler) RefundOrder(c *gin.Context) {
	orderID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid order_id UUID"})
		return
	}

	var req RefundOrderRequest
	_ = c.ShouldBindJSON(&req)

	// Fetch latest payment for this order
	latestPayment, err := h.svc.repo.GetLatestPaymentForOrder(c.Request.Context(), orderID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "no payment found for order"})
		return
	}

	if latestPayment.Status != StatusCaptured {
		c.JSON(http.StatusUnprocessableEntity, gin.H{"error": "payment is not in captured status; ineligible for refund"})
		return
	}

	refund, err := h.svc.RefundPayment(c.Request.Context(), latestPayment.ID, req.Reason)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, refund)
}

// HandleWebhook verifies HMAC-SHA256 signature and processes gateway callbacks.
func (h *Handler) HandleWebhook(c *gin.Context) {
	if h.webhookSecret == "" {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "webhook handler not configured"})
		return
	}

	signature := c.GetHeader("X-Gateway-Signature")
	if signature == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "missing X-Gateway-Signature header"})
		return
	}

	body, err := io.ReadAll(c.Request.Body)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "failed to read request body"})
		return
	}

	// Compute expected HMAC-SHA256 signature
	mac := hmac.New(sha256.New, []byte(h.webhookSecret))
	mac.Write(body)
	expectedSig := hex.EncodeToString(mac.Sum(nil))

	if !hmac.Equal([]byte(signature), []byte(expectedSig)) {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid gateway signature"})
		return
	}

	var payload WebhookPayload
	if err := json.Unmarshal(body, &payload); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "malformed webhook JSON"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"status":  "processed",
		"event":   payload.Event,
		"order_id": payload.OrderID,
	})
}
