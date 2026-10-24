package tracking

import (
	"context"
	"encoding/json"
	"log/slog"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
)

// OrderEventsChannel is the global Redis Pub/Sub channel for live order state transitions.
const OrderEventsChannel = "quickcart:order_events"

// OrderEvent represents a live status transition event streamed to the customer.
type OrderEvent struct {
	OrderID    uuid.UUID `json:"order_id"`
	Status     string    `json:"status"`
	Message    string    `json:"message"`
	Timestamp  time.Time `json:"timestamp"`
	InstanceID string    `json:"instance_id,omitempty"`
}

// SSEHub manages active client subscriptions partitioned by order ID.
// In a distributed cluster behind Nginx, it bridges local in-memory subscriber channels
// with a shared Redis Pub/Sub topic so events emitted on any API instance reach subscribers
// connected to any other instance.
type SSEHub struct {
	mu          sync.RWMutex
	subscribers map[uuid.UUID]map[chan OrderEvent]struct{}
	rdb         *redis.Client
	instanceID  string
	cancel      context.CancelFunc
	wg          sync.WaitGroup
	logger      *slog.Logger
}

// NewSSEHub initializes a fresh process-local SSE event hub.
func NewSSEHub() *SSEHub {
	return &SSEHub{
		subscribers: make(map[uuid.UUID]map[chan OrderEvent]struct{}),
		instanceID:  uuid.New().String(),
		logger:      slog.Default(),
	}
}

// NewDistributedSSEHub initializes an SSE event hub backed by Redis Pub/Sub for cross-instance messaging.
func NewDistributedSSEHub(ctx context.Context, rdb *redis.Client, instanceID string, logger *slog.Logger) *SSEHub {
	if logger == nil {
		logger = slog.Default()
	}
	if instanceID == "" {
		instanceID = uuid.New().String()
	}

	hub := &SSEHub{
		subscribers: make(map[uuid.UUID]map[chan OrderEvent]struct{}),
		rdb:         rdb,
		instanceID:  instanceID,
		logger:      logger,
	}

	if rdb != nil {
		subCtx, cancel := context.WithCancel(ctx)
		hub.cancel = cancel
		hub.wg.Add(1)
		go hub.listenRedisPubSub(subCtx)
	}

	return hub
}

func (h *SSEHub) listenRedisPubSub(ctx context.Context) {
	defer h.wg.Done()

	pubsub := h.rdb.Subscribe(ctx, OrderEventsChannel)
	defer func() {
		_ = pubsub.Close()
	}()

	ch := pubsub.Channel()
	for {
		select {
		case <-ctx.Done():
			return
		case msg, ok := <-ch:
			if !ok {
				return
			}
			var event OrderEvent
			if err := json.Unmarshal([]byte(msg.Payload), &event); err != nil {
				h.logger.Warn("failed to unmarshal redis order event", "error", err)
				continue
			}

			// Avoid duplicate delivery: ignore messages originated by this same instance
			if event.InstanceID != h.instanceID {
				h.localBroadcast(event)
			}
		}
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
// It dispatches immediately to local clients, and if Redis is configured, broadcasts across the cluster.
func (h *SSEHub) Publish(event OrderEvent) {
	event.InstanceID = h.instanceID
	h.localBroadcast(event)

	if h.rdb != nil {
		data, err := json.Marshal(event)
		if err == nil {
			pubCtx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
			defer cancel()
			_ = h.rdb.Publish(pubCtx, OrderEventsChannel, data).Err()
		}
	}
}

func (h *SSEHub) localBroadcast(event OrderEvent) {
	h.mu.RLock()
	defer h.mu.RUnlock()

	if set, exists := h.subscribers[event.OrderID]; exists {
		for ch := range set {
			select {
			case ch <- event:
			default:
				// Buffer full; drop to avoid stalling publishers
			}
		}
	}
}

// SubscriberCount returns the current number of active listeners for an order on this instance.
func (h *SSEHub) SubscriberCount(orderID uuid.UUID) int {
	h.mu.RLock()
	defer h.mu.RUnlock()

	if set, exists := h.subscribers[orderID]; exists {
		return len(set)
	}
	return 0
}

// Close gracefully terminates background Redis pub/sub subscriptions.
func (h *SSEHub) Close() {
	if h.cancel != nil {
		h.cancel()
		h.wg.Wait()
	}
}
