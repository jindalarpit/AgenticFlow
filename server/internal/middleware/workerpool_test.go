package middleware_test

import (
	"sync/atomic"
	"testing"
	"time"

	"github.com/agenticflow/agenticflow/server/internal/middleware"
	"pgregory.net/rapid"
)

// Property 3: Worker pool boundedness
//
// For any number N of concurrent submissions (where N > pool size), the number
// of concurrently executing work functions never exceeds the configured pool
// size (workers count). When the pool is full, Submit() returns false (work is
// dropped). The pool correctly bounds concurrency regardless of burst size.
//
// **Validates: Requirements 7.1, 7.2**

func TestProperty3_WorkerPoolBoundedness_MaxConcurrencyNeverExceedsPoolSize(t *testing.T) {
	// Property 3: Worker pool boundedness
	// For any burst of submissions exceeding pool capacity, the number of
	// concurrently executing goroutines never exceeds the configured pool size.
	rapid.Check(t, func(t *rapid.T) {
		// Generate a pool size between 1 and 20
		poolSize := rapid.IntRange(1, 20).Draw(t, "poolSize")
		// Generate a burst size that exceeds pool capacity (pool + buffer)
		// The buffer is workers*2, so total capacity is workers + workers*2 = workers*3
		burstSize := rapid.IntRange(poolSize*3+1, poolSize*10).Draw(t, "burstSize")

		pool := middleware.NewTokenUpdatePool(poolSize)

		var maxConcurrent atomic.Int64
		var currentConcurrent atomic.Int64
		done := make(chan struct{}, burstSize)

		// Submit burst of work items that track concurrency
		for i := 0; i < burstSize; i++ {
			pool.Submit(func() {
				cur := currentConcurrent.Add(1)
				// Track the maximum observed concurrency
				for {
					old := maxConcurrent.Load()
					if cur <= old || maxConcurrent.CompareAndSwap(old, cur) {
						break
					}
				}
				// Simulate some work to allow overlap
				time.Sleep(time.Millisecond)
				currentConcurrent.Add(-1)
				done <- struct{}{}
			})
		}

		// Shutdown the pool to ensure all accepted work completes
		pool.Shutdown(5 * time.Second)

		// The maximum concurrent goroutines must never exceed pool size
		observed := maxConcurrent.Load()
		if observed > int64(poolSize) {
			t.Fatalf("max concurrent goroutines %d exceeded pool size %d", observed, poolSize)
		}
	})
}

func TestProperty3_WorkerPoolBoundedness_SubmitReturnsFalseWhenFull(t *testing.T) {
	// Property 3: Worker pool boundedness
	// When the pool is full (workers busy + channel buffer full), Submit()
	// returns false indicating the work was dropped.
	rapid.Check(t, func(t *rapid.T) {
		// Generate a small pool size to make it easy to fill
		poolSize := rapid.IntRange(1, 10).Draw(t, "poolSize")
		// Channel buffer size is workers*2
		bufferSize := poolSize * 2

		pool := middleware.NewTokenUpdatePool(poolSize)

		// Use a channel to confirm workers are blocked
		workerStarted := make(chan struct{}, poolSize)
		blocker := make(chan struct{})

		// Submit enough work to occupy all workers (they signal when started)
		for i := 0; i < poolSize; i++ {
			pool.Submit(func() {
				workerStarted <- struct{}{}
				<-blocker // block until released
			})
		}

		// Wait for all workers to be actively blocked
		for i := 0; i < poolSize; i++ {
			<-workerStarted
		}

		// Now all workers are busy. Fill the channel buffer completely.
		for i := 0; i < bufferSize; i++ {
			accepted := pool.Submit(func() {
				<-blocker
			})
			if !accepted {
				t.Fatalf("expected submission %d to be accepted (buffer not yet full), but was dropped", i)
			}
		}

		// Now the pool is completely full: all workers busy + buffer full.
		// Additional submissions must be dropped.
		extraSubmissions := rapid.IntRange(1, 50).Draw(t, "extraSubmissions")
		for i := 0; i < extraSubmissions; i++ {
			accepted := pool.Submit(func() {})
			if accepted {
				t.Fatalf("expected submission to be dropped when pool is full, but it was accepted (extra submission %d)", i)
			}
		}

		// Release blocked workers and shutdown
		close(blocker)
		pool.Shutdown(5 * time.Second)
	})
}

func TestProperty3_WorkerPoolBoundedness_ConcurrencyBoundedRegardlessOfBurstSize(t *testing.T) {
	// Property 3: Worker pool boundedness
	// The pool correctly bounds concurrency regardless of burst size.
	// Even with very large bursts, concurrency stays within pool size.
	rapid.Check(t, func(t *rapid.T) {
		poolSize := rapid.IntRange(2, 15).Draw(t, "poolSize")
		// Generate a large burst relative to pool size
		burstMultiplier := rapid.IntRange(5, 30).Draw(t, "burstMultiplier")
		burstSize := poolSize * burstMultiplier

		pool := middleware.NewTokenUpdatePool(poolSize)

		var maxConcurrent atomic.Int64
		var currentConcurrent atomic.Int64
		var acceptedCount atomic.Int64

		// Submit all work items as fast as possible
		for i := 0; i < burstSize; i++ {
			accepted := pool.Submit(func() {
				cur := currentConcurrent.Add(1)
				for {
					old := maxConcurrent.Load()
					if cur <= old || maxConcurrent.CompareAndSwap(old, cur) {
						break
					}
				}
				// Small sleep to create concurrency overlap
				time.Sleep(500 * time.Microsecond)
				currentConcurrent.Add(-1)
				acceptedCount.Add(1)
			})
			_ = accepted
		}

		pool.Shutdown(10 * time.Second)

		observed := maxConcurrent.Load()
		if observed > int64(poolSize) {
			t.Fatalf("max concurrent goroutines %d exceeded pool size %d (burst=%d)",
				observed, poolSize, burstSize)
		}

		// Verify that at least some work was accepted and executed
		if acceptedCount.Load() == 0 {
			t.Fatalf("no work was executed, pool may be broken")
		}
	})
}
