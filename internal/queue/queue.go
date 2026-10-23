package queue

import (
	"context"
	"errors"
	"time"
)

// Sentinel errors for queue lifecycle and operation states.
var (
	// ErrQueueClosed is returned when attempting to Enqueue or Dequeue on a closed and drained queue.
	ErrQueueClosed = errors.New("queue is closed")

	// ErrQueueFull is returned when non-blocking enqueue fails due to full buffer.
	ErrQueueFull = errors.New("queue is full; buffer capacity exceeded")

	// ErrJobCancelled indicates that a job's context was cancelled prior to execution.
	ErrJobCancelled = errors.New("job context cancelled prior to execution")
)

// JobType identifies the category of asynchronous background work.
type JobType string

const (
	// JobTypeOrderConfirm represents an asynchronous order confirmation event.
	JobTypeOrderConfirm JobType = "ORDER_CONFIRM"

	// JobTypeReservationReclaim represents reclaiming expired inventory reservations.
	JobTypeReservationReclaim JobType = "RESERVATION_RECLAIM"

	// JobTypeBatchInventoryCheck represents multi-product stock verification across dark stores.
	JobTypeBatchInventoryCheck JobType = "BATCH_INVENTORY_CHECK"
)

// Job defines a self-contained unit of asynchronous work with contextual tracing.
type Job struct {
	ID        string    `json:"id"`
	Type      JobType   `json:"type"`
	Payload   any       `json:"payload"`
	CreatedAt time.Time `json:"created_at"`
}

// JobResult represents the execution outcome of a Job.
type JobResult struct {
	JobID       string        `json:"job_id"`
	Error       error         `json:"error,omitempty"`
	ProcessedAt time.Time     `json:"processed_at"`
	Duration    time.Duration `json:"duration"`
}

// Queue defines the minimal abstraction for job ingestion and retrieval.
// This interface allows seamless substitution with distributed message brokers
// (e.g., Redis Streams or NATS JetStream) in subsequent phases.
type Queue interface {
	// Enqueue adds a job to the queue. Blocks if full until ctx cancels or slot frees.
	// Returns ErrQueueClosed if the queue is shutting down.
	Enqueue(ctx context.Context, job Job) error

	// TryEnqueue attempts to add a job without blocking. Returns ErrQueueFull if buffer is saturated.
	TryEnqueue(job Job) error

	// Dequeue retrieves the next job. Blocks until a job arrives, ctx cancels, or queue closes.
	// If the queue is closed and drained, returns ErrQueueClosed.
	Dequeue(ctx context.Context) (Job, error)

	// Close marks the queue as closed to new enqueues and signals workers to drain pending jobs.
	Close() error

	// Len returns the current count of buffered jobs awaiting processing.
	Len() int

	// Cap returns the total buffer capacity of the queue.
	Cap() int
}
