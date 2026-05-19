package rate

import (
	"context"
	"sync"
	"golang.org/x/time/rate"
)

// Pool manages a token-bucket limiter per hostname/site.
type Pool struct {
	mu   sync.Mutex
	pool map[string]*rate.Limiter
}

// NewPool creates an empty pool.
func NewPool() *Pool {
	return &Pool{pool: make(map[string]*rate.Limiter)}
}

// Get returns the limiter for key, creating one at rps if absent.
func (p *Pool) Get(key string, rps int) *rate.Limiter {
	p.mu.Lock()
	defer p.mu.Unlock()
	if lim, ok := p.pool[key]; ok {
		return lim
	}
	lim := rate.NewLimiter(rate.Limit(rps), rps)
	p.pool[key] = lim
	return lim
}

// Wait blocks until the limiter for key allows an event.
func (p *Pool) Wait(ctx context.Context, key string, rps int) error {
	return p.Get(key, rps).Wait(ctx)
}
