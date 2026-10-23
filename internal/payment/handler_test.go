package payment

import (
	"bytes"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Dhanya562004/quickcart-go/internal/auth"
	"github.com/Dhanya562004/quickcart-go/internal/order"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func init() {
	gin.SetMode(gin.TestMode)
}

func setupTestRouter(h *Handler, tokenMgr auth.TokenManager) *gin.Engine {
	r := gin.New()
	v1 := r.Group("/api/v1")
	authMw := auth.Authenticate(tokenMgr)
	adminMw := auth.RequireRole(auth.RoleAdmin)
	h.RegisterRoutes(v1, authMw, adminMw)
	return r
}

func TestPaymentHandler_SecurityAndRBAC(t *testing.T) {
	tokenMgr := auth.NewTokenManager(auth.DefaultTokenConfig("secret-key-at-least-32-chars-long!"), nil)
	webhookSecret := "webhook-secret-key-32-bytes-minimum"

	customerID := uuid.New()
	otherCustomerID := uuid.New()
	adminID := uuid.New()

	custPair, err := tokenMgr.GenerateTokenPair(auth.User{ID: customerID, Email: "customer@example.com", Role: auth.RoleCustomer})
	require.NoError(t, err)
	custToken := custPair.AccessToken

	otherPair, err := tokenMgr.GenerateTokenPair(auth.User{ID: otherCustomerID, Email: "other@example.com", Role: auth.RoleCustomer})
	require.NoError(t, err)
	otherToken := otherPair.AccessToken

	adminPair, err := tokenMgr.GenerateTokenPair(auth.User{ID: adminID, Email: "admin@example.com", Role: auth.RoleAdmin})
	require.NoError(t, err)
	adminToken := adminPair.AccessToken

	fakeOrderRepo := order.NewFakeRepository()
	orderID := uuid.New()
	fakeOrderRepo.Orders[orderID] = order.Order{
		ID:               orderID,
		UserID:           customerID,
		TotalAmountPaise: 50000,
		Status:           order.StatusPlaced,
	}

	fakePayRepo := NewFakeRepository()
	gateway := NewFakeGateway()
	svc := NewService(fakePayRepo, gateway, nil, DefaultRetryPolicy(), nil)
	handler := NewHandler(svc, fakeOrderRepo, webhookSecret)

	router := setupTestRouter(handler, tokenMgr)

	t.Run("Reject Unauthenticated Payment", func(t *testing.T) {
		w := httptest.NewRecorder()
		req, _ := http.NewRequest(http.MethodPost, "/api/v1/orders/"+orderID.String()+"/pay", nil)
		router.ServeHTTP(w, req)
		assert.Equal(t, http.StatusUnauthorized, w.Code)
	})

	t.Run("Reject Cross-User Payment (Customer Cannot Pay for Another's Order)", func(t *testing.T) {
		w := httptest.NewRecorder()
		req, _ := http.NewRequest(http.MethodPost, "/api/v1/orders/"+orderID.String()+"/pay", bytes.NewBufferString(`{"method":"UPI"}`))
		req.Header.Set("Authorization", "Bearer "+otherToken)
		req.Header.Set("Content-Type", "application/json")
		router.ServeHTTP(w, req)
		assert.Equal(t, http.StatusForbidden, w.Code)
	})

	t.Run("Allow Order Owner to Pay", func(t *testing.T) {
		w := httptest.NewRecorder()
		req, _ := http.NewRequest(http.MethodPost, "/api/v1/orders/"+orderID.String()+"/pay", bytes.NewBufferString(`{"method":"UPI"}`))
		req.Header.Set("Authorization", "Bearer "+custToken)
		req.Header.Set("Content-Type", "application/json")
		router.ServeHTTP(w, req)
		assert.Equal(t, http.StatusOK, w.Code)

		var res Payment
		err := json.Unmarshal(w.Body.Bytes(), &res)
		require.NoError(t, err)
		assert.Equal(t, StatusCaptured, res.Status)
		assert.Equal(t, orderID, res.OrderID)
	})

	t.Run("Customer Cannot Execute Administrative Refund", func(t *testing.T) {
		w := httptest.NewRecorder()
		req, _ := http.NewRequest(http.MethodPost, "/api/v1/orders/"+orderID.String()+"/refund", bytes.NewBufferString(`{"reason":"Fraud"}`))
		req.Header.Set("Authorization", "Bearer "+custToken)
		req.Header.Set("Content-Type", "application/json")
		router.ServeHTTP(w, req)
		assert.Equal(t, http.StatusForbidden, w.Code)
	})

	t.Run("Admin Can Execute Refund", func(t *testing.T) {
		w := httptest.NewRecorder()
		req, _ := http.NewRequest(http.MethodPost, "/api/v1/orders/"+orderID.String()+"/refund", bytes.NewBufferString(`{"reason":"Item unavailable"}`))
		req.Header.Set("Authorization", "Bearer "+adminToken)
		req.Header.Set("Content-Type", "application/json")
		router.ServeHTTP(w, req)
		assert.Equal(t, http.StatusOK, w.Code)

		var res Refund
		err := json.Unmarshal(w.Body.Bytes(), &res)
		require.NoError(t, err)
		assert.Equal(t, "COMPLETED", res.Status)
	})

	t.Run("Webhook Rejects Missing Signature", func(t *testing.T) {
		body := `{"event":"payment.succeeded","order_id":"` + orderID.String() + `"}`
		w := httptest.NewRecorder()
		req, _ := http.NewRequest(http.MethodPost, "/api/v1/payments/webhook", bytes.NewBufferString(body))
		req.Header.Set("Content-Type", "application/json")
		router.ServeHTTP(w, req)
		assert.Equal(t, http.StatusUnauthorized, w.Code)
	})

	t.Run("Webhook Rejects Invalid Signature", func(t *testing.T) {
		body := `{"event":"payment.succeeded","order_id":"` + orderID.String() + `"}`
		w := httptest.NewRecorder()
		req, _ := http.NewRequest(http.MethodPost, "/api/v1/payments/webhook", bytes.NewBufferString(body))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("X-Gateway-Signature", "invalid-hex-signature")
		router.ServeHTTP(w, req)
		assert.Equal(t, http.StatusUnauthorized, w.Code)
	})

	t.Run("Webhook Accepts Valid HMAC Signature", func(t *testing.T) {
		body := []byte(`{"event":"payment.succeeded","order_id":"` + orderID.String() + `"}`)
		mac := hmac.New(sha256.New, []byte(webhookSecret))
		mac.Write(body)
		validSig := hex.EncodeToString(mac.Sum(nil))

		w := httptest.NewRecorder()
		req, _ := http.NewRequest(http.MethodPost, "/api/v1/payments/webhook", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("X-Gateway-Signature", validSig)
		router.ServeHTTP(w, req)
		assert.Equal(t, http.StatusOK, w.Code)

		var res map[string]string
		err := json.Unmarshal(w.Body.Bytes(), &res)
		require.NoError(t, err)
		assert.Equal(t, "processed", res["status"])
		assert.Equal(t, "payment.succeeded", res["event"])
	})
}
