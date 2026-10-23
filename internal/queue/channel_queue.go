package queue

import (
	"context"
	"sync"
	"sync/atomic"
)

// ChannelQueue is an in-memory, thread-safe implementation of Queue backed by a buffered Go channel.
// Concurrency guarantee:
// - Uses a two-phase shutdown (quit broadcast + sync.RWMutex write-lock barrier) to guarantee that
//   concurrent Enqueue and Close operations never deadlock or cause a "send on closed channel" panic.
// - Supports graceful draining: Dequeue will continue to yield pending jobs until the channel is
//   completely empty before returning ErrQueueClosed.
type ChannelQueue struct {
	ch        chan Job
	quit      chan struct{}
	mu        sync.RWMutex
	closed    atomic.Bool
	closeOnce sync.Once
}

// NewChannelQueue creates a new buffered ChannelQueue with the specified capacity.
// If capacity is <= 0, a default buffer of 1 is allocated.
func NewChannelQueue(capacity int) *ChannelQueue {
	if capacity <= 0 {
		capacity = 1
	}
	return &ChannelQueue{
		ch:   make(chan Job, capacity),
		quit: make(chan struct{}),
	}
}

// Enqueue adds a job to the buffer. If the buffer is full, it blocks until:
// 1. Space becomes available (returns nil)
// 2. The provided context is cancelled (returns ctx.Err())
// 3. The queue is closed (returns ErrQueueClosed)
//
// Concurrency note:
// Holding an RLock guarantees that Close cannot close the backing channel while a sender
// is actively writing to it. If the buffer is full when Close() is invoked, the close(q.quit)
// broadcast immediately unblocks the select, releasing the RLock and letting Close() proceed.
func (q *ChannelQueue) Enqueue(ctx context.Context, job Job) error {
	if q.closed.Load() {
		return ErrQueueClosed
	}

	q.mu.RLock()
	defer q.mu.RUnlock()

	if q.closed.Load() {
		return ErrQueueClosed
	}

	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-q.quit:
		return ErrQueueClosed
	case q.ch <- job:
		return nil
	}
}

// TryEnqueue attempts to enqueue without blocking. If the buffer is full,
// it returns ErrQueueFull immediately.
func (q *ChannelQueue) TryEnqueue(job Job) error {
	if q.closed.Load() {
		return ErrQueueClosed
	}

	q.mu.RLock()
	defer q.mu.RUnlock()

	if q.closed.Load() {
		return ErrQueueClosed
	}

	select {
	case <-q.quit:
		return ErrQueueClosed
	case q.ch <- job:
		return nil
	default:
		return ErrQueueFull
	}
}

// Dequeue retrieves the next job from the queue. It blocks until:
// 1. A job is available (returns the Job and nil)
// 2. The provided context is cancelled (returns empty Job and ctx.Err())
// 3. The queue is closed AND all buffered jobs have been drained (returns empty Job and ErrQueueClosed)
func (q *ChannelQueue) Dequeue(ctx context.Context) (Job, error) {
	select {
	case <-ctx.Done():
		return Job{}, ctx.Err()
	case job, ok := <-q.ch:
		if !ok {
			return Job{}, ErrQueueClosed
		}
		return job, nil
	}
}

// Close gracefully closes the queue.
// 1. Marks closed atomic flag so new Enqueue callers fast-fail.
// 2. Broadcasts to quit channel to unblock any Enqueue callers waiting on a full buffer.
// 3. Acquires write lock barrier to ensure all in-flight Enqueues have released their RLocks.
// 4. Closes the job channel, allowing Dequeue consumers to drain all remaining items.
func (q *ChannelQueue) Close() error {
	q.closeOnce.Do(func() {
		q.closed.Store(true)
		close(q.quit)

		q.mu.Lock()
		defer q.mu.Unlock()

		close(q.ch)
	})
	return nil
}

// Len returns the number of buffered jobs currently waiting to be dequeued.
func (q *ChannelQueue) Len() int {
	return len(q.ch)
}

// Cap returns the maximum buffer capacity of the channel.
func (q *ChannelQueue) Cap() int {
	return cap(q.ch)
}
