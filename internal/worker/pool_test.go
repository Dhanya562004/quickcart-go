package worker_test

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/goleak"

	"github.com/Dhanya562004/quickcart-go/internal/queue"
	"github.com/Dhanya562004/quickcart-go/internal/worker"
)

func TestWorkerPool_ProcessesJobsAcrossWorkers(t *testing.T) {
	defer goleak.VerifyNone(t)

	q := queue.NewChannelQueue(20)
	cfg := worker.PoolConfig{
		WorkerCount:  4,
		DrainTimeout: 2 * time.Second,
	}
	pool := worker.NewWorkerPool(cfg, q, slog.Default())

	var processedCount atomic.Int32
	pool.RegisterHandler(queue.JobTypeOrderConfirm, func(ctx context.Context, job queue.Job) error {
		processedCount.Add(1)
		return nil
	})

	require.NoError(t, pool.Start(context.Background()))

	// Enqueue 10 jobs
	for i := 1; i <= 10; i++ {
		require.NoError(t, q.Enqueue(context.Background(), queue.Job{
			ID:   fmt.Sprintf("job-%d", i),
			Type: queue.JobTypeOrderConfirm,
		}))
	}

	// Wait for processing
	for processedCount.Load() < 10 {
		time.Sleep(5 * time.Millisecond)
	}

	require.NoError(t, pool.Stop(context.Background()))
	assert.Equal(t, int32(10), processedCount.Load())
}

func TestWorkerPool_JobFailureDoesNotKillPool(t *testing.T) {
	defer goleak.VerifyNone(t)

	q := queue.NewChannelQueue(10)
	cfg := worker.PoolConfig{
		WorkerCount:  2,
		DrainTimeout: 2 * time.Second,
	}
	pool := worker.NewWorkerPool(cfg, q, slog.Default())

	var processedCount atomic.Int32
	errBoom := errors.New("boom")

	pool.RegisterHandler(queue.JobTypeOrderConfirm, func(ctx context.Context, job queue.Job) error {
		processedCount.Add(1)
		if job.ID == "failing-job" {
			return errBoom
		}
		return nil
	})

	require.NoError(t, pool.Start(context.Background()))

	// Enqueue a failing job followed by successful jobs
	require.NoError(t, q.Enqueue(context.Background(), queue.Job{ID: "failing-job", Type: queue.JobTypeOrderConfirm}))
	require.NoError(t, q.Enqueue(context.Background(), queue.Job{ID: "ok-job-1", Type: queue.JobTypeOrderConfirm}))
	require.NoError(t, q.Enqueue(context.Background(), queue.Job{ID: "ok-job-2", Type: queue.JobTypeOrderConfirm}))

	for processedCount.Load() < 3 {
		time.Sleep(5 * time.Millisecond)
	}

	// Stop must succeed cleanly without returning errBoom as a fatal pool error
	require.NoError(t, pool.Stop(context.Background()))
	assert.Equal(t, int32(3), processedCount.Load())
}

func TestWorkerPool_GracefulDrainOnStop(t *testing.T) {
	defer goleak.VerifyNone(t)

	q := queue.NewChannelQueue(20)
	cfg := worker.PoolConfig{
		WorkerCount:  2,
		DrainTimeout: 3 * time.Second,
	}
	pool := worker.NewWorkerPool(cfg, q, slog.Default())

	var processedCount atomic.Int32
	pool.RegisterHandler(queue.JobTypeOrderConfirm, func(ctx context.Context, job queue.Job) error {
		time.Sleep(10 * time.Millisecond)
		processedCount.Add(1)
		return nil
	})

	// Pre-fill 10 jobs before starting
	for i := 1; i <= 10; i++ {
		require.NoError(t, q.Enqueue(context.Background(), queue.Job{
			ID:   fmt.Sprintf("job-%d", i),
			Type: queue.JobTypeOrderConfirm,
		}))
	}

	require.NoError(t, pool.Start(context.Background()))

	// Immediately call Stop: all 10 buffered items must be drained gracefully
	require.NoError(t, pool.Stop(context.Background()))
	assert.Equal(t, int32(10), processedCount.Load())
}

func TestWorkerPool_ForcedCancellationOnDrainTimeout(t *testing.T) {
	defer goleak.VerifyNone(t)

	q := queue.NewChannelQueue(5)
	cfg := worker.PoolConfig{
		WorkerCount:  1,
		DrainTimeout: 100 * time.Millisecond, // Short drain timeout
	}
	pool := worker.NewWorkerPool(cfg, q, slog.Default())

	var cancelledObserved atomic.Bool
	pool.RegisterHandler(queue.JobTypeOrderConfirm, func(ctx context.Context, job queue.Job) error {
		select {
		case <-time.After(2 * time.Second): // Hangs longer than DrainTimeout
			return nil
		case <-ctx.Done():
			cancelledObserved.Store(true)
			return ctx.Err()
		}
	})

	require.NoError(t, pool.Start(context.Background()))

	require.NoError(t, q.Enqueue(context.Background(), queue.Job{
		ID:   "hanging-job",
		Type: queue.JobTypeOrderConfirm,
	}))

	// Allow worker to pick up hanging job
	time.Sleep(20 * time.Millisecond)

	// Stop must trigger forced cancellation after DrainTimeout
	require.NoError(t, pool.Stop(context.Background()))
	assert.True(t, cancelledObserved.Load(), "worker context must receive cancellation")
}
