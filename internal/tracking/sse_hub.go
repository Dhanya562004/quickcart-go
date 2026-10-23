package tracking

import (
	"sync"
	"time"

	"github.com/google/uuid"
)

// OrderEvent represents a live status transition event streamed to the customer.
type OrderEvent struct {
	OrderID   uuid.UUID `json:"order_id"`
	Status    string    `json:"status"`
	Message   string    `json:"message"`
	Timestamp time.Time `json:"timestamp"`
}

// SSEHub manages active client subscriptions partitioned by order ID.
// Concurrency Guarantees:
// 1. Thread-safe subscription and unsubscription via sync.RWMutex.
// 2. Decoupled domain broadcasting: Domain services publish status changes to the hub
//    without knowledge of HTTP connections or network protocols.
// 3. Buffered subscriber channels prevent slow or stalled network clients from blocking publishers.
type SSEHub struct {
	mu          sync.RWMutex
	subscribers map[uuid.UUID]map[chan OrderEvent]struct{}
}

// NewSSEHub initializes a fresh SSE event hub.
func NewSSEHub() *SSEHub {
	return &SSEHub{
		subscribers: make(map[uuid.UUID]map[chan OrderEvent]struct{}),
	}
}

// Subscribe registers a new subscriber channel for the specified order ID.
func (h *SSEHub) Subscribe(orderID uuid.UUID) chan OrderEvent {
	h.mu.Lock()
	defer h.mu.Unlock()

	ch := make(chan OrderEvent, 16)
	if _, exists := h.subscribers[orderID]; !exists {
		h.subscribers[orderID] = make(map[chan OrderEvent]struct{})
	}
	h.subscribers[orderID][ch] = struct{}{}
	return ch
}

// Unsubscribe cleanly unregisters a subscriber channel and cleans up empty order maps.
func (h *SSEHub) Unsubscribe(orderID uuid.UUID, ch chan OrderEvent) {
	h.mu.Lock()
	defer h.mu.Unlock()

	if set, exists := h.subscribers[orderID]; exists {
		delete(set, ch)
		close(ch)
		if len(set) == 0 {
			delete(h.subscribers, orderID)
		}
	}
}

// Publish broadcasts an OrderEvent to all active subscribers for that order ID.
// Non-blocking: if a subscriber buffer is full, it drops the event or continues rather than blocking.
func (h *SSEHub) Publish(event OrderEvent) {
	h.mu.RLock()
	defer h.mu.RUnlock()

	if set, exists := h.subscribers[event.OrderID]; exists {
		for ch := range set {
			select {
			case ch <- event:
			default:
				// Buffer full; avoid stalling domain publisher
			}
		}
	}
}

// SubscriberCount returns the current number of active listeners for an order.
func (h *SSEHub) SubscriberCount(orderID uuid.UUID) int {
	h.mu.RLock()
	defer h.mu.RUnlock()

	if set, exists := h.subscribers[orderID]; exists {
		return len(set)
	}
	return 0
}
