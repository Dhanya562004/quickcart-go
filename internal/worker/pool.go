package worker

import (
	"context"
	"errors"
	"log/slog"
	"sync"
	"time"

	"golang.org/x/sync/errgroup"

	"github.com/Dhanya562004/quickcart-go/internal/queue"
)

var (
	// ErrPoolStarted is returned when attempting to start an already running worker pool.
	ErrPoolStarted = errors.New("worker pool already started")

	// ErrPoolStopped is returned when operating on a stopped worker pool.
	ErrPoolStopped = errors.New("worker pool stopped")

	// ErrNoHandlerRegistered is returned when a dequeued job has no registered handler.
	ErrNoHandlerRegistered = errors.New("no handler registered for job type")
)

// JobHandler defines the function signature for executing a background job.
// A non-nil error indicates a job-level failure (which is logged), but does NOT
// terminate or kill the worker pool.
type JobHandler func(ctx context.Context, job queue.Job) error

// PoolConfig specifies parameters for the bounded worker pool.
type PoolConfig struct {
	// WorkerCount is the bounded number of concurrent worker goroutines.
	WorkerCount int

	// DrainTimeout is the maximum duration permitted for workers to process
	// pending jobs from the queue after Stop() is invoked, before forced cancellation.
	DrainTimeout time.Duration
}

// WorkerPool manages a bounded number of worker goroutines consuming jobs from a Queue.
// Concurrency Architecture:
// - Uses golang.org/x/sync/errgroup to supervise worker goroutines.
// - Uses sync.RWMutex to protect the handler registry and state flags.
// - Uses sync.Once to guarantee idempotent, race-free shutdown.
// - Separates graceful draining from forced cancellation:
//   1. On Stop(), the queue is closed, stopping new admissions while workers drain buffered work.
//   2. If DrainTimeout or the caller context expires, cancelFunc() forces in-flight jobs to abort.
//   3. Normal job errors are recorded in logs and never kill the pool.
type WorkerPool struct {
	cfg      PoolConfig
	queue    queue.Queue
	handlers map[queue.JobType]JobHandler
	mu       sync.RWMutex // Protects handlers, started, and stopped flags

	started  bool
	stopped  bool
	stopOnce sync.Once // Guarantees Stop() is executed exactly once

	workerCtx  context.Context
	cancelFunc context.CancelFunc
	eg         *errgroup.Group

	logger *slog.Logger
}

// NewWorkerPool creates an unstarted worker pool with the specified configuration and queue.
func NewWorkerPool(cfg PoolConfig, q queue.Queue, logger *slog.Logger) *WorkerPool {
	if cfg.WorkerCount <= 0 {
		cfg.WorkerCount = 4
	}
	if cfg.DrainTimeout <= 0 {
		cfg.DrainTimeout = 5 * time.Second
	}
	if logger == nil {
		logger = slog.Default()
	}

	return &WorkerPool{
		cfg:      cfg,
		queue:    q,
		handlers: make(map[queue.JobType]JobHandler),
		logger:   logger,
	}
}

// RegisterHandler binds a JobType to a specific JobHandler function.
// Thread-safe via sync.RWMutex write-lock.
func (p *WorkerPool) RegisterHandler(jobType queue.JobType, handler JobHandler) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.handlers[jobType] = handler
}

// Start spawns exactly cfg.WorkerCount worker goroutines supervised by errgroup.
func (p *WorkerPool) Start(parentCtx context.Context) error {
	p.mu.Lock()
	defer p.mu.Unlock()

	if p.started {
		return ErrPoolStarted
	}
	if p.stopped {
		return ErrPoolStopped
	}

	p.started = true
	p.workerCtx, p.cancelFunc = context.WithCancel(parentCtx)
	p.eg, _ = errgroup.WithContext(p.workerCtx)

	for i := 0; i < p.cfg.WorkerCount; i++ {
		workerID := i
		p.eg.Go(func() error {
			p.runWorker(workerID)
			return nil
		})
	}

	p.logger.Info("worker pool started",
		"worker_count", p.cfg.WorkerCount,
		"drain_timeout", p.cfg.DrainTimeout,
	)
	return nil
}

// runWorker is the continuous execution loop for each bounded worker goroutine.
func (p *WorkerPool) runWorker(id int) {
	for {
		// Dequeue blocks until a job is ready, context is cancelled, or queue is closed and empty.
		job, err := p.queue.Dequeue(p.workerCtx)
		if err != nil {
			if errors.Is(err, queue.ErrQueueClosed) {
				// Clean shutdown: Queue has been closed and all pending items drained.
				return
			}
			if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
				// Forced shutdown: Context cancelled.
				return
			}
			p.logger.Error("unexpected dequeue error", "worker_id", id, "error", err)
			return
		}

		p.executeJob(id, job)
	}
}

// executeJob invokes the registered handler for a job, isolating errors to prevent pool termination.
func (p *WorkerPool) executeJob(workerID int, job queue.Job) {
	p.mu.RLock()
	handler, exists := p.handlers[job.Type]
	p.mu.RUnlock()

	if !exists {
		p.logger.Warn("no handler registered for job",
			"worker_id", workerID,
			"job_id", job.ID,
			"job_type", job.Type,
		)
		return
	}

	start := time.Now()
	// Job failure must NOT kill the worker pool. Log the error and continue.
	if err := handler(p.workerCtx, job); err != nil {
		p.logger.Error("job execution failed",
			"worker_id", workerID,
			"job_id", job.ID,
			"job_type", job.Type,
			"duration", time.Since(start),
			"error", err,
		)
		return
	}

	p.logger.Debug("job completed successfully",
		"worker_id", workerID,
		"job_id", job.ID,
		"duration", time.Since(start),
	)
}

// Stop executes a two-phase graceful shutdown:
// Phase 1 (Graceful Drain): Closes the queue to stop new admissions, allowing workers to drain buffered jobs.
// Phase 2 (Timeout / Forced Cancellation): If jobs do not complete within cfg.DrainTimeout or caller ctx,
// cancels workerCtx to abort remaining work.
func (p *WorkerPool) Stop(callerCtx context.Context) error {
	p.mu.Lock()
	if !p.started {
		p.mu.Unlock()
		return nil
	}
	if p.stopped {
		p.mu.Unlock()
		return nil
	}
	p.stopped = true
	p.mu.Unlock()

	var stopErr error
	p.stopOnce.Do(func() {
		p.logger.Info("stopping worker pool; beginning graceful queue drain...")

		// Step 1: Close queue to reject new enqueues while permitting Dequeue to drain existing items.
		if err := p.queue.Close(); err != nil {
			p.logger.Error("error closing queue", "error", err)
		}

		// Step 2: Supervise drain completion.
		drainDone := make(chan error, 1)
		go func() {
			drainDone <- p.eg.Wait()
		}()

		select {
		case err := <-drainDone:
			p.logger.Info("worker pool gracefully drained and stopped")
			p.cancelFunc()
			stopErr = err
		case <-time.After(p.cfg.DrainTimeout):
			p.logger.Warn("drain timeout exceeded; forcing worker cancellation",
				"timeout", p.cfg.DrainTimeout,
			)
			p.cancelFunc()
			stopErr = <-drainDone
		case <-callerCtx.Done():
			p.logger.Warn("caller shutdown context expired; forcing worker cancellation")
			p.cancelFunc()
			stopErr = <-drainDone
		}
	})

	return stopErr
}
