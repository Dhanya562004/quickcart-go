package worker_test

import (
	"context"
	"errors"
	"fmt"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/goleak"

	"github.com/Dhanya562004/quickcart-go/internal/worker"
)

func TestProcessBatch_DeterministicOrdering(t *testing.T) {
	defer goleak.VerifyNone(t)

	ctx := context.Background()
	const itemCount = 50
	items := make([]int, itemCount)
	for i := 0; i < itemCount; i++ {
		items[i] = i
	}

	// Each worker introduces varying delays to prove ordering is independent of completion time
	results := worker.ProcessBatch(ctx, items, 8, func(ctx context.Context, item int) (string, error) {
		time.Sleep(time.Duration(item%5) * time.Millisecond)
		return fmt.Sprintf("val-%d", item), nil
	})

	require.Len(t, results, itemCount)
	for i, res := range results {
		assert.Equal(t, i, res.Index, "Index must strictly match slice position")
		assert.Equal(t, fmt.Sprintf("val-%d", i), res.Value)
		assert.NoError(t, res.Err)
	}
}

func TestProcessBatch_PerItemErrors(t *testing.T) {
	defer goleak.VerifyNone(t)

	ctx := context.Background()
	items := []int{1, 2, 3, 4, 5}
	errOdd := errors.New("odd numbers fail")

	results := worker.ProcessBatch(ctx, items, 4, func(ctx context.Context, item int) (int, error) {
		if item%2 != 0 {
			return 0, errOdd
		}
		return item * 10, nil
	})

	require.Len(t, results, 5)

	// Item 0 (1): error
	assert.Equal(t, 0, results[0].Index)
	assert.ErrorIs(t, results[0].Err, errOdd)

	// Item 1 (2): success 20
	assert.Equal(t, 1, results[1].Index)
	assert.NoError(t, results[1].Err)
	assert.Equal(t, 20, results[1].Value)

	// Item 2 (3): error
	assert.Equal(t, 2, results[2].Index)
	assert.ErrorIs(t, results[2].Err, errOdd)

	// Item 3 (4): success 40
	assert.Equal(t, 3, results[3].Index)
	assert.NoError(t, results[3].Err)
	assert.Equal(t, 40, results[3].Value)

	// Item 4 (5): error
	assert.Equal(t, 4, results[4].Index)
	assert.ErrorIs(t, results[4].Err, errOdd)
}

func TestProcessBatch_ContextCancellation(t *testing.T) {
	defer goleak.VerifyNone(t)

	ctx, cancel := context.WithCancel(context.Background())
	items := make([]int, 20)
	for i := range items {
		items[i] = i
	}

	var processedCount atomic.Int32

	results := worker.ProcessBatch(ctx, items, 2, func(ctx context.Context, item int) (int, error) {
		count := processedCount.Add(1)
		if count >= 3 {
			cancel() // cancel after 3 items are picked up
		}
		time.Sleep(10 * time.Millisecond)
		return item, nil
	})

	require.Len(t, results, 20)
	cancelledFound := false
	for _, res := range results {
		if errors.Is(res.Err, context.Canceled) {
			cancelledFound = true
			break
		}
	}
	assert.True(t, cancelledFound, "expected at least some items to be marked context.Canceled")
}

func TestProcessBatch_EmptyItems(t *testing.T) {
	defer goleak.VerifyNone(t)

	results := worker.ProcessBatch(context.Background(), []string{}, 4, func(ctx context.Context, s string) (string, error) {
		return s, nil
	})
	assert.Empty(t, results)
}
