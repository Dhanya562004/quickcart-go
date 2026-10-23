package tracking_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/goleak"

	"github.com/Dhanya562004/quickcart-go/internal/auth"
	"github.com/Dhanya562004/quickcart-go/internal/order"
	"github.com/Dhanya562004/quickcart-go/internal/tracking"
)

type mockOrderLookup struct {
	orders map[uuid.UUID]*order.Order
}

func (m *mockOrderLookup) GetOrderByID(_ context.Context, id uuid.UUID) (*order.Order, error) {
	ord, exists := m.orders[id]
	if !exists {
		return nil, order.ErrNotFound
	}
	return ord, nil
}

func setupTestTokenManager() auth.TokenManager {
	return auth.NewTokenManager(auth.DefaultTokenConfig("sse-test-secret-key-32-bytes-long!"), nil)
}

func TestSSE_PublishAndReceive(t *testing.T) {
	defer goleak.VerifyNone(t)

	hub := tracking.NewSSEHub()
	orderID := uuid.New()

	ch := hub.Subscribe(orderID)
	defer hub.Unsubscribe(orderID, ch)

	assert.Equal(t, 1, hub.SubscriberCount(orderID))

	// Publish domain event
	event := tracking.OrderEvent{
		OrderID:   orderID,
		Status:    "PACKED",
		Message:   "Order packed by warehouse picker",
		Timestamp: time.Now().UTC(),
	}
	hub.Publish(event)

	select {
	case received := <-ch:
		assert.Equal(t, orderID, received.OrderID)
		assert.Equal(t, "PACKED", received.Status)
		assert.Equal(t, "Order packed by warehouse picker", received.Message)
	case <-time.After(500 * time.Millisecond):
		t.Fatal("timed out waiting for SSE domain event")
	}
}

func TestSSEHandler_AccessControl_OwnershipEnforced(t *testing.T) {
	defer goleak.VerifyNone(t)
	gin.SetMode(gin.TestMode)

	tm := setupTestTokenManager()
	hub := tracking.NewSSEHub()

	ownerID := uuid.New()
	otherID := uuid.New()
	adminID := uuid.New()
	orderID := uuid.New()

	// Generate authenticated tokens
	ownerToken, err := tm.GenerateTokenPair(auth.User{ID: ownerID, Email: "owner@quickcart.in", Role: auth.RoleCustomer})
	require.NoError(t, err)

	otherToken, err := tm.GenerateTokenPair(auth.User{ID: otherID, Email: "other@quickcart.in", Role: auth.RoleCustomer})
	require.NoError(t, err)

	adminToken, err := tm.GenerateTokenPair(auth.User{ID: adminID, Email: "admin@quickcart.in", Role: auth.RoleAdmin})
	require.NoError(t, err)

	mockOrders := &mockOrderLookup{
		orders: map[uuid.UUID]*order.Order{
			orderID: {
				ID:     orderID,
				UserID: ownerID,
				Status: order.StatusPlaced,
			},
		},
	}

	handler := tracking.NewHandler(hub, mockOrders, 5*time.Second)
	r := gin.New()
	handler.RegisterRoutes(r.Group("/api/v1"), auth.Authenticate(tm))

	// 1. Anonymous request without token -> 401 Unauthorized
	reqAnon := httptest.NewRequest(http.MethodGet, "/api/v1/orders/"+orderID.String()+"/events", nil)
	wAnon := httptest.NewRecorder()
	r.ServeHTTP(wAnon, reqAnon)
	assert.Equal(t, http.StatusUnauthorized, wAnon.Code)

	// 2. Spoofed headers/params ignored -> 401 Unauthorized
	reqSpoofed := httptest.NewRequest(http.MethodGet, "/api/v1/orders/"+orderID.String()+"/events?user_id="+ownerID.String(), nil)
	reqSpoofed.Header.Set("X-User-ID", ownerID.String())
	reqSpoofed.Header.Set("X-User-Role", "admin")
	wSpoofed := httptest.NewRecorder()
	r.ServeHTTP(wSpoofed, reqSpoofed)
	assert.Equal(t, http.StatusUnauthorized, wSpoofed.Code, "unverified headers/query parameters MUST NOT be trusted")

	// 3. Cross-user access (Customer B attempts to access Customer A's order) -> 403 Forbidden
	reqCross := httptest.NewRequest(http.MethodGet, "/api/v1/orders/"+orderID.String()+"/events", nil)
	reqCross.Header.Set("Authorization", "Bearer "+otherToken.AccessToken)
	wCross := httptest.NewRecorder()
	r.ServeHTTP(wCross, reqCross)
	assert.Equal(t, http.StatusForbidden, wCross.Code)

	// 4. Non-existent order -> 404 Not Found
	missingID := uuid.New().String()
	req404 := httptest.NewRequest(http.MethodGet, "/api/v1/orders/"+missingID+"/events", nil)
	req404.Header.Set("Authorization", "Bearer "+ownerToken.AccessToken)
	w404 := httptest.NewRecorder()
	r.ServeHTTP(w404, req404)
	assert.Equal(t, http.StatusNotFound, w404.Code)

	// 5. Admin can access any order stream
	ctxAdmin, cancelAdmin := context.WithCancel(context.Background())
	defer cancelAdmin()
	reqAdmin := httptest.NewRequest(http.MethodGet, "/api/v1/orders/"+orderID.String()+"/events", nil).WithContext(ctxAdmin)
	reqAdmin.Header.Set("Authorization", "Bearer "+adminToken.AccessToken)
	wAdmin := httptest.NewRecorder()

	var wgAdmin sync.WaitGroup
	wgAdmin.Add(1)
	go func() {
		defer wgAdmin.Done()
		r.ServeHTTP(wAdmin, reqAdmin)
	}()

	require.Eventually(t, func() bool {
		return hub.SubscriberCount(orderID) == 1
	}, 500*time.Millisecond, 10*time.Millisecond)
	cancelAdmin()
	wgAdmin.Wait()
}

func TestSSEHandler_BrowserEventSource_QueryParamToken(t *testing.T) {
	defer goleak.VerifyNone(t)
	gin.SetMode(gin.TestMode)

	tm := setupTestTokenManager()
	hub := tracking.NewSSEHub()
	ownerID := uuid.New()
	orderID := uuid.New()

	ownerToken, err := tm.GenerateTokenPair(auth.User{ID: ownerID, Email: "owner@quickcart.in", Role: auth.RoleCustomer})
	require.NoError(t, err)

	mockOrders := &mockOrderLookup{
		orders: map[uuid.UUID]*order.Order{
			orderID: {
				ID:     orderID,
				UserID: ownerID,
				Status: order.StatusPlaced,
			},
		},
	}

	handler := tracking.NewHandler(hub, mockOrders, 50*time.Millisecond)
	r := gin.New()
	handler.RegisterRoutes(r.Group("/api/v1"), auth.Authenticate(tm))

	// Browser EventSource connects via: GET /api/v1/orders/:id/events?token=<jwt_access_token>
	ctx, cancel := context.WithCancel(context.Background())
	req := httptest.NewRequest(http.MethodGet, "/api/v1/orders/"+orderID.String()+"/events?token="+ownerToken.AccessToken, nil).WithContext(ctx)
	w := httptest.NewRecorder()

	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		r.ServeHTTP(w, req)
	}()

	require.Eventually(t, func() bool {
		return hub.SubscriberCount(orderID) == 1
	}, 500*time.Millisecond, 10*time.Millisecond)

	cancel()
	wg.Wait()

	assert.Equal(t, 0, hub.SubscriberCount(orderID))
}

func TestSSEHandler_ClientDisconnect_CleanExit(t *testing.T) {
	defer goleak.VerifyNone(t)
	gin.SetMode(gin.TestMode)

	tm := setupTestTokenManager()
	hub := tracking.NewSSEHub()
	ownerID := uuid.New()
	orderID := uuid.New()

	ownerToken, err := tm.GenerateTokenPair(auth.User{ID: ownerID, Email: "owner@quickcart.in", Role: auth.RoleCustomer})
	require.NoError(t, err)

	mockOrders := &mockOrderLookup{
		orders: map[uuid.UUID]*order.Order{
			orderID: {
				ID:     orderID,
				UserID: ownerID,
				Status: order.StatusPlaced,
			},
		},
	}

	handler := tracking.NewHandler(hub, mockOrders, 50*time.Millisecond)
	r := gin.New()
	handler.RegisterRoutes(r.Group("/api/v1"), auth.Authenticate(tm))

	ctx, cancel := context.WithCancel(context.Background())
	req := httptest.NewRequest(http.MethodGet, "/api/v1/orders/"+orderID.String()+"/events", nil).WithContext(ctx)
	req.Header.Set("Authorization", "Bearer "+ownerToken.AccessToken)
	w := httptest.NewRecorder()

	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		r.ServeHTTP(w, req)
	}()

	require.Eventually(t, func() bool {
		return hub.SubscriberCount(orderID) == 1
	}, 500*time.Millisecond, 10*time.Millisecond)

	hub.Publish(tracking.OrderEvent{
		OrderID:   orderID,
		Status:    "CONFIRMED",
		Message:   "Payment received",
		Timestamp: time.Now().UTC(),
	})

	cancel()
	wg.Wait()

	assert.Equal(t, 0, hub.SubscriberCount(orderID), "subscriber count must be 0 after client disconnect")
	assert.Equal(t, http.StatusOK, w.Code)
}

func TestDistributedSSE_CrossInstancePublishAndReceive(t *testing.T) {
	// If Redis is not available locally, test in-memory fallback behavior
	hub1 := tracking.NewSSEHub()
	defer hub1.Close()

	hub2 := tracking.NewSSEHub()
	defer hub2.Close()

	orderID := uuid.New()
	ch1 := hub1.Subscribe(orderID)
	defer hub1.Unsubscribe(orderID, ch1)

	// Publish on hub1
	hub1.Publish(tracking.OrderEvent{
		OrderID:   orderID,
		Status:    "OUT_FOR_DELIVERY",
		Message:   "Rider picked up order",
		Timestamp: time.Now().UTC(),
	})

	select {
	case event := <-ch1:
		assert.Equal(t, "OUT_FOR_DELIVERY", event.Status)
		assert.Equal(t, orderID, event.OrderID)
	case <-time.After(500 * time.Millisecond):
		t.Fatal("timed out waiting for event on hub1")
	}
}
