package worker

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/Dhanya562004/quickcart-go/internal/queue"
)

// OrderConfirmPayload contains contextual metadata for confirming an order asynchronously.
type OrderConfirmPayload struct {
	OrderID    string    `json:"order_id"`
	StoreID    string    `json:"store_id"`
	CustomerID string    `json:"customer_id"`
	PlacedAt   time.Time `json:"placed_at"`
}

// ReservationReclaimPayload contains parameters for reclaiming expired inventory reservations.
// Note: In Phase 3, this defines the job and dispatch contract.
// Full database processing using FOR UPDATE SKIP LOCKED is implemented in Phase 4.
type ReservationReclaimPayload struct {
	DarkStoreID string `json:"dark_store_id"`
	BatchLimit  int    `json:"batch_limit"`
}

// BatchInventoryCheckPayload defines a payload for batch verifying stock across items.
type BatchInventoryCheckPayload struct {
	StoreID    string   `json:"store_id"`
	ProductIDs []string `json:"product_ids"`
}

// DefaultOrderConfirmHandler logs and executes simulated order confirmation.
func DefaultOrderConfirmHandler(logger *slog.Logger) JobHandler {
	return func(ctx context.Context, job queue.Job) error {
		payload, ok := job.Payload.(OrderConfirmPayload)
		if !ok {
			return fmt.Errorf("invalid payload for %s: expected OrderConfirmPayload, got %T", job.Type, job.Payload)
		}

		logger.Info("processing order confirmation job",
			"job_id", job.ID,
			"order_id", payload.OrderID,
			"store_id", payload.StoreID,
		)
		return nil
	}
}

// DefaultReservationReclaimHandler logs and manages reservation reclamation cycles.
// In Phase 3, this executes the queue dispatch infrastructure.
func DefaultReservationReclaimHandler(logger *slog.Logger) JobHandler {
	return func(ctx context.Context, job queue.Job) error {
		payload, ok := job.Payload.(ReservationReclaimPayload)
		if !ok {
			return fmt.Errorf("invalid payload for %s: expected ReservationReclaimPayload, got %T", job.Type, job.Payload)
		}

		logger.Info("executing reservation reclaim job (Phase 3 infrastructure)",
			"job_id", job.ID,
			"store_id", payload.DarkStoreID,
			"limit", payload.BatchLimit,
		)
		return nil
	}
}
