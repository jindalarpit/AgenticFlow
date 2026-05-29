package middleware

import (
	"sync"
	"time"
)

// TokenUpdatePool manages a bounded set of worker goroutines for
// asynchronous last_used_at token updates. It prevents unbounded goroutine
// spawning under high authentication load.
type TokenUpdatePool struct {
	work chan func()
	wg   sync.WaitGroup
}

// NewTokenUpdatePool creates a pool with the given number of workers.
// The work channel is buffered at workers*2 to absorb short bursts.
func NewTokenUpdatePool(workers int) *TokenUpdatePool {
	p := &TokenUpdatePool{
		work: make(chan func(), workers*2),
	}
	for i := 0; i < workers; i++ {
		p.wg.Add(1)
		go p.worker()
	}
	return p
}

func (p *TokenUpdatePool) worker() {
	defer p.wg.Done()
	for fn := range p.work {
		fn()
	}
}

// Submit enqueues work to the pool. Returns true if the work was accepted,
// or false if the pool is full (the update is dropped without blocking).
func (p *TokenUpdatePool) Submit(fn func()) bool {
	select {
	case p.work <- fn:
		return true
	default:
		return false // pool full, drop update
	}
}

// Shutdown closes the work channel and waits for all workers to drain
// pending work. If workers don't finish within the timeout, Shutdown returns.
func (p *TokenUpdatePool) Shutdown(timeout time.Duration) {
	close(p.work)
	done := make(chan struct{})
	go func() { p.wg.Wait(); close(done) }()
	select {
	case <-done:
	case <-time.After(timeout):
	}
}
