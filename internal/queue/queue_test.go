package queue_test

import (
	"context"
	"fmt"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/goleak"

	"github.com/Dhanya562004/quickcart-go/internal/queue"
)

func TestChannelQueue_BasicFIFO(t *testing.T) {
	defer goleak.VerifyNone(t)

	q := queue.NewChannelQueue(5)
	ctx := context.Background()

	jobs := []queue.Job{
		{ID: "job-1", Type: queue.JobTypeOrderConfirm, CreatedAt: time.Now()},
		{ID: "job-2", Type: queue.JobTypeReservationReclaim, CreatedAt: time.Now()},
		{ID: "job-3", Type: queue.JobTypeBatchInventoryCheck, CreatedAt: time.Now()},
	}

	for _, j := range jobs {
		err := q.Enqueue(ctx, j)
		require.NoError(t, err)
	}

	assert.Equal(t, 3, q.Len())
	assert.Equal(t, 5, q.Cap())

	for _, expected := range jobs {
		got, err := q.Dequeue(ctx)
		require.NoError(t, err)
		assert.Equal(t, expected.ID, got.ID)
		assert.Equal(t, expected.Type, got.Type)
	}

	assert.Equal(t, 0, q.Len())
	require.NoError(t, q.Close())
}

func TestChannelQueue_TryEnqueue_FullQueue(t *testing.T) {
	defer goleak.VerifyNone(t)

	q := queue.NewChannelQueue(2)

	require.NoError(t, q.TryEnqueue(queue.Job{ID: "1"}))
	require.NoError(t, q.TryEnqueue(queue.Job{ID: "2"}))

	// Buffer is now full (cap=2). Next TryEnqueue must fail immediately with ErrQueueFull.
	err := q.TryEnqueue(queue.Job{ID: "3"})
	assert.ErrorIs(t, err, queue.ErrQueueFull)

	require.NoError(t, q.Close())
}

func TestChannelQueue_EnqueueContextCancellation(t *testing.T) {
	defer goleak.VerifyNone(t)

	q := queue.NewChannelQueue(1)
	ctx, cancel := context.WithCancel(context.Background())

	require.NoError(t, q.Enqueue(ctx, queue.Job{ID: "1"}))

	// Buffer full. Next Enqueue blocks. Cancel context after 30ms.
	go func() {
		time.Sleep(30 * time.Millisecond)
		cancel()
	}()

	err := q.Enqueue(ctx, queue.Job{ID: "2"})
	assert.ErrorIs(t, err, context.Canceled)

	require.NoError(t, q.Close())
}

func TestChannelQueue_DequeueContextCancellation(t *testing.T) {
	defer goleak.VerifyNone(t)

	q := queue.NewChannelQueue(2)
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Millisecond)
	defer cancel()

	_, err := q.Dequeue(ctx)
	assert.ErrorIs(t, err, context.DeadlineExceeded)

	require.NoError(t, q.Close())
}

func TestChannelQueue_GracefulDrainAfterClose(t *testing.T) {
	defer goleak.VerifyNone(t)

	q := queue.NewChannelQueue(5)
	ctx := context.Background()

	for i := 1; i <= 3; i++ {
		require.NoError(t, q.Enqueue(ctx, queue.Job{ID: fmt.Sprintf("job-%d", i)}))
	}

	// Close queue while items are still buffered
	require.NoError(t, q.Close())

	// Enqueue after close must fail immediately
	err := q.Enqueue(ctx, queue.Job{ID: "job-4"})
	assert.ErrorIs(t, err, queue.ErrQueueClosed)

	// Dequeue must drain all 3 existing items in FIFO order
	for i := 1; i <= 3; i++ {
		job, err := q.Dequeue(ctx)
		require.NoError(t, err)
		assert.Equal(t, fmt.Sprintf("job-%d", i), job.ID)
	}

	// Once drained, Dequeue returns ErrQueueClosed
	_, err = q.Dequeue(ctx)
	assert.ErrorIs(t, err, queue.ErrQueueClosed)
}

func TestChannelQueue_ConcurrentEnqueueAndClose(t *testing.T) {
	defer goleak.VerifyNone(t)

	// Stress test concurrent Enqueue and Close.
	// Must NEVER panic on "send on closed channel" and NEVER deadlock.
	const (
		concurrency = 50
		capacity    = 5
	)

	q := queue.NewChannelQueue(capacity)
	ctx := context.Background()

	var wg sync.WaitGroup
	var successCount atomic.Int64
	var closedCount atomic.Int64
	var stopProducers atomic.Bool

	// Start 50 goroutines trying to enqueue continuously until closed
	for i := 0; i < concurrency; i++ {
		wg.Add(1)
		go func(workerID int) {
			defer wg.Done()
			j := 0
			for {
				if stopProducers.Load() {
					return
				}
				err := q.Enqueue(ctx, queue.Job{ID: fmt.Sprintf("w-%d-j-%d", workerID, j)})
				switch err {
				case nil:
					successCount.Add(1)
					j++
				case queue.ErrQueueClosed:
					closedCount.Add(1)
					return
				}
			}
		}(i)
	}

	// Also start a consumer dequeuing at moderate rate
	drainDone := make(chan struct{})
	go func() {
		defer close(drainDone)
		for {
			_, err := q.Dequeue(context.Background())
			if err != nil {
				return
			}
		}
	}()

	// Allow healthy traffic to flow before closing
	for successCount.Load() < 50 {
		time.Sleep(1 * time.Millisecond)
	}

	// Close concurrently while producers are actively hammering the queue
	require.NoError(t, q.Close())
	stopProducers.Store(true)

	// Wait for all producers to finish
	wg.Wait()
	<-drainDone

	assert.True(t, successCount.Load() >= 50, "expected at least 50 successful enqueues")
	assert.True(t, closedCount.Load() > 0, "expected some closed queue errors")

	// Post-close enqueue must always return ErrQueueClosed
	err := q.Enqueue(ctx, queue.Job{ID: "after-close"})
	assert.ErrorIs(t, err, queue.ErrQueueClosed)
}
