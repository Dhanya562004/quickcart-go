package worker

import (
	"context"
	"sync"
)

// BatchItemResult encapsulates the outcome of an individual item processed within a batch.
// Index corresponds strictly to the 0-indexed position of the input item, guaranteeing deterministic ordering.
type BatchItemResult[R any] struct {
	Index int   `json:"index"`
	Value R     `json:"value,omitempty"`
	Err   error `json:"error,omitempty"`
}

type indexedWork[T any] struct {
	index int
	item  T
}

// ProcessBatch executes work items across a bounded pool of worker goroutines using the fan-out / fan-in pattern.
// Concurrency & Correctness Guarantees:
// 1. Fan-out: Distributes items across workerCount concurrent goroutines via a buffered channel.
// 2. Fan-in: Aggregates results through a single results channel into a pre-allocated deterministic slice.
// 3. Deterministic ordering: The returned slice has exactly len(items) elements where result[i].Index == i.
// 4. Per-item errors: If fn returns an error for an item, that error is recorded in result[i].Err without
//    aborting the processing of other items.
// 5. Context Cancellation & Unprocessed items: If ctx is cancelled before or during processing, unprocessed items
//    and items aborted by context are explicitly assigned Err: ctx.Err().
func ProcessBatch[T any, R any](
	ctx context.Context,
	items []T,
	workerCount int,
	fn func(context.Context, T) (R, error),
) []BatchItemResult[R] {
	total := len(items)
	if total == 0 {
		return []BatchItemResult[R]{}
	}

	if workerCount <= 0 {
		workerCount = 1
	}
	if workerCount > total {
		workerCount = total
	}

	results := make([]BatchItemResult[R], total)

	// Fan-out: jobs channel pre-buffered with all items so workers pull without contention
	jobsCh := make(chan indexedWork[T], total)
	for i, item := range items {
		jobsCh <- indexedWork[T]{index: i, item: item}
	}
	close(jobsCh)

	// Fan-in: results channel collecting outcomes from all workers
	resultsCh := make(chan BatchItemResult[R], total)

	var wg sync.WaitGroup
	wg.Add(workerCount)

	for w := 0; w < workerCount; w++ {
		go func() {
			defer wg.Done()
			for work := range jobsCh {
				// Check context before executing item
				select {
				case <-ctx.Done():
					resultsCh <- BatchItemResult[R]{
						Index: work.index,
						Err:   ctx.Err(),
					}
					continue
				default:
				}

				val, err := fn(ctx, work.item)
				resultsCh <- BatchItemResult[R]{
					Index: work.index,
					Value: val,
					Err:   err,
				}
			}
		}()
	}

	// Coordinator goroutine: close resultsCh once all workers finish
	go func() {
		wg.Wait()
		close(resultsCh)
	}()

	// Fan-in collector: assemble results into deterministic order
	for res := range resultsCh {
		results[res.Index] = res
	}

	return results
}
