package inventory

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"sync"
	"sync/atomic"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/Dhanya562004/quickcart-go/internal/db"
	"github.com/Dhanya562004/quickcart-go/internal/platform/database"
)

// ExpiryWorkerConfig sets polling and batching parameters for the reservation reclaimer.
type ExpiryWorkerConfig struct {
	BatchSize    int32
	PollInterval time.Duration
}

// DefaultExpiryConfig provides sensible defaults for quick-commerce TTL reclamation.
func DefaultExpiryConfig() ExpiryWorkerConfig {
	return ExpiryWorkerConfig{
		BatchSize:    50,
		PollInterval: 1 * time.Second,
	}
}

// ReclaimResult reports outcomes from a single reclamation sweep.
type ReclaimResult struct {
	ProcessedCount int
	ReclaimedCount int
}

// ExpiryWorker periodically reclaims expired reservations using FOR UPDATE SKIP LOCKED.
type ExpiryWorker struct {
	pool    *pgxpool.Pool
	queries *db.Queries
	cfg     ExpiryWorkerConfig
	logger  *slog.Logger

	stopCh  chan struct{}
	wg      sync.WaitGroup
	stopped atomic.Bool
}

// NewExpiryWorker constructs a reservation reclaimer.
func NewExpiryWorker(pool *pgxpool.Pool, cfg ExpiryWorkerConfig, logger *slog.Logger) *ExpiryWorker {
	if cfg.BatchSize <= 0 {
		cfg.BatchSize = 50
	}
	if cfg.PollInterval <= 0 {
		cfg.PollInterval = 1 * time.Second
	}
	if logger == nil {
		logger = slog.Default()
	}

	return &ExpiryWorker{
		pool:    pool,
		queries: db.New(pool),
		cfg:     cfg,
		logger:  logger,
		stopCh:  make(chan struct{}),
	}
}

// Start spawns the background worker polling loop.
func (w *ExpiryWorker) Start(ctx context.Context) {
	w.wg.Add(1)
	go func() {
		defer w.wg.Done()
		ticker := time.NewTicker(w.cfg.PollInterval)
		defer ticker.Stop()

		for {
			select {
			case <-w.stopCh:
				return
			case <-ctx.Done():
				return
			case <-ticker.C:
				if _, err := w.ReclaimBatch(ctx); err != nil && !errors.Is(err, context.Canceled) {
					w.logger.Error("error during reservation reclamation sweep", "error", err)
				}
			}
		}
	}()
}

// Stop gracefully halts the background worker.
func (w *ExpiryWorker) Stop() {
	if w.stopped.CompareAndSwap(false, true) {
		close(w.stopCh)
		w.wg.Wait()
	}
}

// ReclaimBatch executes one atomic sweep using FOR UPDATE SKIP LOCKED.
// Concurrency Guarantees:
// 1. FOR UPDATE SKIP LOCKED ensures that concurrent worker instances never process the same rows
//    or deadlock with each other.
// 2. Conditional status guards (WHERE status = 'RESERVED') ensure that reservations concurrently
//    confirmed by payment capture are NEVER expired, preventing double-restoration or invalid cancellations.
// 3. All updates (reservation expiration, order cancellation, inventory restoration, audit log)
//    execute inside a single ACID transaction via database.WithTx.
func (w *ExpiryWorker) ReclaimBatch(ctx context.Context) (ReclaimResult, error) {
	var result ReclaimResult

	err := database.WithTx(ctx, w.pool, func(tx pgx.Tx) error {
		qtx := db.New(tx)

		// 1. Atomically pick expired reservations while skipping rows locked by other workers
		rows, err := qtx.PickExpiredReservationsForUpdate(ctx, w.cfg.BatchSize)
		if err != nil {
			return fmt.Errorf("failed to pick expired reservations: %w", err)
		}

		if len(rows) == 0 {
			return nil
		}

		result.ProcessedCount = len(rows)

		for _, row := range rows {
			// 2. Guarded transition: status must still be 'RESERVED'
			// If payment completion concurrently confirmed this reservation, this returns ErrNoRows
			_, mErr := qtx.MarkReservationExpired(ctx, row.ID)
			if mErr != nil {
				// Reservation was modified by another transaction; skip safely
				continue
			}

			// 3. Atomically restore inventory to available pool
			_, rErr := qtx.RestoreInventoryAtomic(ctx, db.RestoreInventoryAtomicParams{
				StoreID:           row.StoreID,
				ProductID:         row.ProductID,
				AvailableQuantity: row.Quantity,
			})
			if rErr != nil {
				return fmt.Errorf("failed to restore inventory for product %v: %w", row.ProductID, rErr)
			}

			// 4. Cancel order if it is still in PLACED or CONFIRMED state
			orderID := row.OrderID
			var cancelledID pgtype.UUID
			cErr := tx.QueryRow(ctx, `
				UPDATE orders
				SET status = 'CANCELLED', updated_at = NOW()
				WHERE id = $1 AND status IN ('PLACED', 'CONFIRMED')
				RETURNING id
			`, orderID).Scan(&cancelledID)

			if cErr == nil {
				// 5. Append audit history record
				_, _ = tx.Exec(ctx, `
					INSERT INTO order_status_history (
						id, order_id, from_status, to_status, actor_type, notes, created_at
					) VALUES (
						gen_random_uuid(), $1, 'PLACED', 'CANCELLED', 'SYSTEM',
						'Order auto-cancelled due to expired reservation TTL', NOW()
					)
				`, orderID)
			}

			result.ReclaimedCount++
		}

		return nil
	})

	if err != nil {
		return ReclaimResult{}, err
	}

	if result.ReclaimedCount > 0 {
		w.logger.Info("reclaimed expired reservations successfully",
			"picked", result.ProcessedCount,
			"reclaimed", result.ReclaimedCount,
		)
	}

	return result, nil
}
