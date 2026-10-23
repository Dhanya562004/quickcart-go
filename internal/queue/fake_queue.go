package queue

import (
	"context"
	"sync"
)

// FakeQueue is a thread-safe in-memory queue implementation for testing.
type FakeQueue struct {
	mu     sync.Mutex
	jobs   []Job
	closed bool
}

// NewFakeQueue initializes a FakeQueue with pre-populated jobs if needed.
func NewFakeQueue(initialJobs ...Job) *FakeQueue {
	return &FakeQueue{
		jobs: append([]Job{}, initialJobs...),
	}
}

func (f *FakeQueue) Enqueue(ctx context.Context, job Job) error {
	f.mu.Lock()
	defer f.mu.Unlock()

	if f.closed {
		return ErrQueueClosed
	}
	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
		f.jobs = append(f.jobs, job)
		return nil
	}
}

func (f *FakeQueue) TryEnqueue(job Job) error {
	f.mu.Lock()
	defer f.mu.Unlock()

	if f.closed {
		return ErrQueueClosed
	}
	f.jobs = append(f.jobs, job)
	return nil
}

func (f *FakeQueue) Dequeue(ctx context.Context) (Job, error) {
	f.mu.Lock()
	defer f.mu.Unlock()

	if len(f.jobs) == 0 {
		if f.closed {
			return Job{}, ErrQueueClosed
		}
		select {
		case <-ctx.Done():
			return Job{}, ctx.Err()
		default:
			return Job{}, ErrQueueClosed
		}
	}

	job := f.jobs[0]
	f.jobs = f.jobs[1:]
	return job, nil
}

func (f *FakeQueue) Close() error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.closed = true
	return nil
}

func (f *FakeQueue) Len() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return len(f.jobs)
}

func (f *FakeQueue) Cap() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return len(f.jobs)
}
