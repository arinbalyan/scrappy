package rate_test

import (
	"context"
	"testing"
	"time"

	"github.com/arinbalyan/scrappy/internal/rate"
	"github.com/stretchr/testify/assert"
)

func TestPoolGetCreatesLimiter(t *testing.T) {
	p := rate.NewPool()
	lim := p.Get("indeed", 3)
	assert.NotNil(t, lim)
	assert.Equal(t, 3, int(lim.Limit()))
}

func TestPoolGetReusesLimiter(t *testing.T) {
	p := rate.NewPool()
	a := p.Get("indeed", 3)
	b := p.Get("indeed", 5)
	assert.Same(t, a, b, "second Get should return the same limiter")
	assert.Equal(t, 3, int(a.Limit()), "rate should not change on second Get")
}

func TestPoolGetDistinctKeys(t *testing.T) {
	p := rate.NewPool()
	li := p.Get("linkedin", 1)
	in := p.Get("indeed", 10)
	assert.NotSame(t, li, in)
}

func TestPoolWaitBlocksUntilToken(t *testing.T) {
	p := rate.NewPool()
	ctx := context.Background()
	start := time.Now()
	e := p.Wait(ctx, "slow", 1) // 1 req/s
	elapsed := time.Since(start)
	assert.NoError(t, e)
	assert.Less(t, elapsed, 2*time.Second, "single token at 1 rps should not take > 2s")
}
