# Phase 3: Advanced Concurrency Engine Notes

**Branch**: `feat/phase-3-concurrency`  
**Target Role**: Associate SDE (Go / Concurrency & Systems), Swiggy Instamart  
**Focus**: Bounded Worker Pools, Thread-Safe Channel Queues, Fan-Out / Fan-In Batching, Context-Driven Graceful Shutdown, Zero-Overselling Concurrency Verification.

---

## 1. Architectural Overview & Deliverables

Phase 3 introduces production-grade concurrent asynchronous execution to QuickCart, complementing the Phase 2 ACID order engine with background processing and bounded concurrency primitives:

1. **`internal/queue`**:
   - `Queue` interface: Minimal abstraction (`Enqueue`, `TryEnqueue`, `Dequeue`, `Close`, `Len`, `Cap`) facilitating drop-in replacements for distributed message brokers (NATS JetStream or Redis Streams).
   - `ChannelQueue`: Mutex-and-broadcast guarded buffered channel queue. Safely resolves concurrent `Enqueue` and `Close` calls without deadlocks or send-on-closed-channel panics.
   - Fully tested with `go.uber.org/goleak` to guarantee zero goroutines leaked.

2. **`internal/worker`**:
   - `WorkerPool`: Bounded goroutine supervisor built with `golang.org/x/sync/errgroup`, `sync.RWMutex`, and `sync.Once`.
   - Error Isolation: Individual job failures are logged and tracked without aborting the pool or cancelling neighboring workers.
   - Two-Phase Graceful Shutdown: Phase 1 drains buffered queue jobs; Phase 2 forces context cancellation if drain exceeds `DrainTimeout`.
   - `ProcessBatch`: Generic, deterministic fan-out / fan-in batch processor (`ProcessBatch[T, R]`) preserving input-slice indexing, isolating per-item errors, and explicitly marking cancelled or unprocessed items with `ctx.Err()`.
   - Concrete Job Infrastructure: `JobTypeOrderConfirm` and `JobTypeReservationReclaim` (infrastructure ready for Phase 4 distributed expiry).

3. **Headline Race Test (`internal/order/headline_race_test.go`)**:
   - 1,000 concurrent customer goroutines competing for 10 units of inventory on live PostgreSQL.
   - Bounded connection pool (`maxConns = 25`) with realistic timeouts.
   - Verified 5 consecutive times under `go test -race -count=5`: exactly 10 orders placed, exactly 990 conflict rejections, stock ends at 0 (never negative), reserved quantity ends at 10, and zero orphaned reservations.

4. **HTTP Server & Lifecycle Integration (`cmd/server/main.go`)**:
   - `ChannelQueue` and `WorkerPool` wired into server startup and application shutdown lifecycle.
   - On `SIGTERM` / `SIGINT`, HTTP connections drain first, followed by graceful queue draining and worker termination within `cfg.ShutdownTimeout`.

---

## 2. "Why This Design"

### A. Channels vs. Mutexes
In Go, channels and mutexes solve distinct concurrency problems:
- **Buffered Channels (`chan Job`)**: Used for **transfer of ownership** and **backpressure queueing**. A producer transfers ownership of a `Job` to an available worker goroutine without either party knowing the other's identity. Channels provide natural blocking primitives (`select`) that integrate cleanly with `context.Context` cancellation.
- **`sync.RWMutex`**: Used for **shared state lookup** and **lifecycle barriers**. In `ChannelQueue`, an `RWMutex` protects against send-on-closed-channel panics during concurrent shutdown. In `WorkerPool`, it allows concurrent workers to read registered handlers with zero lock contention while safely permitting dynamic registration during boot.
- **`sync.Once`**: Used to ensure critical teardown operations (`queue.Close()`, `pool.Stop()`) execute exactly once, even when multiple signals or teardown routines fire concurrently.

### B. Two-Phase ChannelQueue Shutdown Barrier
Closing a Go channel while senders are actively transmitting results in a runtime panic (`send on closed channel`). Simply wrapping `ch <- job` in an `RWMutex.RLock` and `close(ch)` in `RWMutex.Lock` deadlocks if the channel is full (the sender blocks holding `RLock`, preventing `Close()` from acquiring `Lock`).

QuickCart solves this with a two-phase broadcast barrier:
1. `closed.Store(true)`: Atomic flag fast-fails new callers.
2. `close(quit)`: Broadcasts to all senders waiting inside `select` on a full channel. Senders wake up on `case <-quit: return ErrQueueClosed`, immediately executing `defer mu.RUnlock()`.
3. `mu.Lock()`: Now proceeds without blocking because all senders have exited.
4. `close(ch)`: Safely closes the backing channel. Dequeue consumers can now drain remaining buffered items until `ok == false`, returning `ErrQueueClosed`.

### C. Bounded Concurrency vs. Unbounded Goroutines
Spawning an unbounded goroutine per incoming job (`go process(job)`) causes severe failure modes under burst traffic:
- Uncontrolled memory consumption (each goroutine allocates stack space).
- CPU cache thrashing due to OS thread context switching.
- Database connection pool exhaustion (`too many connections` / connection timeouts).

QuickCart bounds worker concurrency to `WorkerPoolSize` (default: 4, configurable via `.env`). Excess jobs queue in the buffered `ChannelQueue`. If the buffer saturates, backpressure is exerted on publishers via `ErrQueueFull` or blocking with context timeout.

### D. Separation of Graceful Drain from Forced Cancellation
A production worker pool must never drop in-flight or buffered jobs during standard rolling deployments:
1. **Phase 1 (Graceful Drain)**: On `Stop()`, the queue is closed to reject new work. Worker goroutines continue executing and draining queued jobs.
2. **Phase 2 (Forced Cancellation)**: If processing exceeds `DrainTimeout` or the server's outer shutdown deadline, the worker context is cancelled (`cancelFunc()`), aborting in-flight database transactions and freeing connections.
3. **Goroutine Leak Proof**: All tests enforce `defer goleak.VerifyNone(t)` to prove zero leaked goroutines.

---

## 3. Benchmarks & Performance Metrics

### Environment Specs
- **CPU**: 11th Gen Intel(R) Core(TM) i3-1115G4 @ 3.00GHz (4 logical cores)
- **OS / Arch**: Windows 11 / `windows/amd64`
- **Go Toolchain**: `go version go1.25.0 windows/amd64`
- **Command**: `go test -bench . -benchmem github.com/Dhanya562004/quickcart-go/internal/worker`

### Raw Benchmark Output
```
goos: windows
goarch: amd64
pkg: github.com/Dhanya562004/quickcart-go/internal/worker
cpu: 11th Gen Intel(R) Core(TM) i3-1115G4 @ 3.00GHz
BenchmarkWorkerPool_1Worker-4         2777198        510.3 ns/op          16 B/op        1 allocs/op
BenchmarkWorkerPool_2Workers-4        2679043        547.2 ns/op          16 B/op        1 allocs/op
BenchmarkWorkerPool_4Workers-4        2652694        588.6 ns/op          16 B/op        1 allocs/op
BenchmarkWorkerPool_8Workers-4        2762038        422.5 ns/op          16 B/op        1 allocs/op
BenchmarkWorkerPool_16Workers-4       2837296        474.6 ns/op          16 B/op        1 allocs/op
BenchmarkWorkerPool_32Workers-4       2669544        497.7 ns/op          16 B/op        1 allocs/op
BenchmarkProcessBatch_Scaling/1_workers-4    5612     200233 ns/op       82144 B/op        7 allocs/op
BenchmarkProcessBatch_Scaling/2_workers-4    5556     199827 ns/op       82211 B/op        8 allocs/op
BenchmarkProcessBatch_Scaling/4_workers-4    5527     191671 ns/op       82340 B/op       10 allocs/op
BenchmarkProcessBatch_Scaling/8_workers-4    6056     198032 ns/op       82595 B/op       14 allocs/op
BenchmarkProcessBatch_Scaling/16_workers-4   5140     200412 ns/op       83107 B/op       22 allocs/op
BenchmarkProcessBatch_Scaling/32_workers-4   5606     207640 ns/op       84131 B/op       38 allocs/op
PASS
ok      github.com/Dhanya562004/quickcart-go/internal/worker    24.798s
```

### Key Observations
- The worker pool sustains **~2.0 - 2.3 million ops/sec** throughput with negligible memory overhead (16 bytes/op, exactly 1 allocation/op).
- `ProcessBatch` across 1,000 items processes in ~191 - 207 µs with deterministic ordering and isolated error handling.

---

## 4. Headline Concurrency Test Verification

### Test Details
- **Test**: `TestHeadline_1000UsersReserve10Units` (`internal/order/headline_race_test.go`)
- **Conditions**: Real PostgreSQL 16 on port 5433, pgx pool max connections = 25.
- **Input**: 1 Dark Store, 1 Product seeded with exactly **10 units** of inventory, 1,000 customer goroutines released via synchronization latch.
- **Command**: `go test -v -race -count=5 -run=TestHeadline_1000UsersReserve10Units ./internal/order/...`

### Raw Output (5 Consecutive Executions)
```
=== RUN   TestHeadline_1000UsersReserve10Units
time=2026-10-23T12:36:59.899+05:30 level=INFO msg="Connected to PostgreSQL database" max_conns=25 min_conns=5
time=2026-10-23T12:37:03.442+05:30 level=INFO msg="Closing database connection pool"
--- PASS: TestHeadline_1000UsersReserve10Units (3.63s)
=== RUN   TestHeadline_1000UsersReserve10Units
time=2026-10-23T12:37:03.639+05:30 level=INFO msg="Connected to PostgreSQL database" max_conns=25 min_conns=5
time=2026-10-23T12:37:08.160+05:30 level=INFO msg="Closing database connection pool"
--- PASS: TestHeadline_1000UsersReserve10Units (4.73s)
=== RUN   TestHeadline_1000UsersReserve10Units
time=2026-10-23T12:37:08.308+05:30 level=INFO msg="Connected to PostgreSQL database" max_conns=25 min_conns=5
time=2026-10-23T12:37:12.232+05:30 level=INFO msg="Closing database connection pool"
--- PASS: TestHeadline_1000UsersReserve10Units (4.07s)
=== RUN   TestHeadline_1000UsersReserve10Units
time=2026-10-23T12:37:12.470+05:30 level=INFO msg="Connected to PostgreSQL database" max_conns=25 min_conns=5
time=2026-10-23T12:37:23.373+05:30 level=INFO msg="Closing database connection pool"
--- PASS: TestHeadline_1000UsersReserve10Units (11.14s)
=== RUN   TestHeadline_1000UsersReserve10Units
time=2026-10-23T12:37:23.576+05:30 level=INFO msg="Connected to PostgreSQL database" max_conns=25 min_conns=5
time=2026-10-23T12:37:29.250+05:30 level=INFO msg="Closing database connection pool"
--- PASS: TestHeadline_1000UsersReserve10Units (5.86s)
PASS
ok      github.com/Dhanya562004/quickcart-go/internal/order     35.785s
```

### Invariant Checks Verified Across All 5 Runs
1. **Zero Overselling**: Exactly 10 checkouts succeeded; exactly 990 checkouts received `ErrInsufficientStock` (HTTP 409).
2. **Stock Boundaries**: Available stock ended at 0 (never negative); reserved stock ended at 10.
3. **No Orphaned Reservations**: Exactly 10 active reservation records exist, and 100% of reservations map to the 10 placed orders.
4. **Clean Concurrency**: Zero race warnings, zero deadlocks, zero connection leaks.
