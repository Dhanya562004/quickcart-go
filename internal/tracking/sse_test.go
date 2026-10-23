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

	"github.com/Dhanya562004/quickcart-go/internal/order"
	"github.com/Dhanya562004/quickcart-go/internal/tracking"
)

type mockOrderLookup struct {
	orders map[uuid.UUID]*order.Order
}

func (m *mockOrderLookup) GetOrderByID(ctx context.Context, id uuid.UUID) (*order.Order, error) {
	ord, exists := m.orders[id]
	if !exists {
		return nil, order.ErrNotFound
	}
	return ord, nil
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

	hub := tracking.NewSSEHub()
	ownerID := uuid.New()
	orderID := uuid.New()

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
	r.GET("/api/v1/orders/:id/events", handler.StreamOrderEvents)

	// Case 1: Unauthorized user receives 403 Forbidden
	otherUser := uuid.New().String()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/orders/"+orderID.String()+"/events", nil)
	req.Header.Set("X-User-ID", otherUser)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusForbidden, w.Code)
	assert.Contains(t, w.Body.String(), "forbidden")

	// Case 2: Non-existent order receives 404
	missingID := uuid.New().String()
	req404 := httptest.NewRequest(http.MethodGet, "/api/v1/orders/"+missingID+"/events", nil)
	req404.Header.Set("X-User-ID", ownerID.String())
	w404 := httptest.NewRecorder()
	r.ServeHTTP(w404, req404)

	assert.Equal(t, http.StatusNotFound, w404.Code)
}

func TestSSEHandler_ClientDisconnect_CleanExit(t *testing.T) {
	defer goleak.VerifyNone(t)
	gin.SetMode(gin.TestMode)

	hub := tracking.NewSSEHub()
	ownerID := uuid.New()
	orderID := uuid.New()

	mockOrders := &mockOrderLookup{
		orders: map[uuid.UUID]*order.Order{
			orderID: {
				ID:     orderID,
				UserID: ownerID,
				Status: order.StatusPlaced,
			},
		},
	}

	handler := tracking.NewHandler(hub, mockOrders, 50*time.Millisecond) // short heartbeat for testing
	r := gin.New()
	r.GET("/api/v1/orders/:id/events", handler.StreamOrderEvents)

	// Create request with cancellable context
	ctx, cancel := context.WithCancel(context.Background())
	req := httptest.NewRequest(http.MethodGet, "/api/v1/orders/"+orderID.String()+"/events", nil).WithContext(ctx)
	req.Header.Set("X-User-ID", ownerID.String())
	w := httptest.NewRecorder()

	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		r.ServeHTTP(w, req)
	}()

	// Wait until client has subscribed
	require.Eventually(t, func() bool {
		return hub.SubscriberCount(orderID) == 1
	}, 500*time.Millisecond, 10*time.Millisecond)

	// Publish an event while client is connected
	hub.Publish(tracking.OrderEvent{
		OrderID:   orderID,
		Status:    "CONFIRMED",
		Message:   "Payment received",
		Timestamp: time.Now().UTC(),
	})

	// Client disconnects mid-stream
	cancel()
	wg.Wait()

	// Verify subscriber channel was immediately unregistered
	assert.Equal(t, 0, hub.SubscriberCount(orderID), "subscriber count must be 0 after client disconnect")
	assert.Equal(t, http.StatusOK, w.Code)
}
