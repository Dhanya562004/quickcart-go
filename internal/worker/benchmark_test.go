package worker_test

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"testing"
	"time"

	"github.com/Dhanya562004/quickcart-go/internal/queue"
	"github.com/Dhanya562004/quickcart-go/internal/worker"
)

func runWorkerPoolBenchmark(b *testing.B, workerCount int) {
	discardLogger := slog.New(slog.NewTextHandler(io.Discard, nil))
	q := queue.NewChannelQueue(b.N + 100)

	cfg := worker.PoolConfig{
		WorkerCount:  workerCount,
		DrainTimeout: 10 * time.Second,
	}
	pool := worker.NewWorkerPool(cfg, q, discardLogger)

	pool.RegisterHandler(queue.JobTypeOrderConfirm, func(ctx context.Context, job queue.Job) error {
		return nil
	})

	if err := pool.Start(context.Background()); err != nil {
		b.Fatalf("failed to start pool: %v", err)
	}

	job := queue.Job{
		ID:   "bench-job",
		Type: queue.JobTypeOrderConfirm,
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if err := q.Enqueue(context.Background(), job); err != nil {
			b.Fatalf("failed to enqueue: %v", err)
		}
	}

	if err := pool.Stop(context.Background()); err != nil {
		b.Fatalf("failed to stop pool: %v", err)
	}
	b.StopTimer()
}

func BenchmarkWorkerPool_1Worker(b *testing.B) {
	runWorkerPoolBenchmark(b, 1)
}

func BenchmarkWorkerPool_2Workers(b *testing.B) {
	runWorkerPoolBenchmark(b, 2)
}

func BenchmarkWorkerPool_4Workers(b *testing.B) {
	runWorkerPoolBenchmark(b, 4)
}

func BenchmarkWorkerPool_8Workers(b *testing.B) {
	runWorkerPoolBenchmark(b, 8)
}

func BenchmarkWorkerPool_16Workers(b *testing.B) {
	runWorkerPoolBenchmark(b, 16)
}

func BenchmarkWorkerPool_32Workers(b *testing.B) {
	runWorkerPoolBenchmark(b, 32)
}

func BenchmarkProcessBatch_Scaling(b *testing.B) {
	workerCounts := []int{1, 2, 4, 8, 16, 32}
	items := make([]int, 1000)
	for i := range items {
		items[i] = i
	}

	for _, wc := range workerCounts {
		b.Run(fmt.Sprintf("%d_workers", wc), func(b *testing.B) {
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				_ = worker.ProcessBatch(context.Background(), items, wc, func(ctx context.Context, item int) (int, error) {
					return item * 2, nil
				})
			}
		})
	}
}
