package proxy_test

import (
	"testing"

	proxypkg "github.com/arinbalyan/scrappy/internal/proxy"
	"github.com/stretchr/testify/assert"
)

func TestNewProxyURLParsesScheme(t *testing.T) {
	px, err := proxypkg.NewProxyURL("socks5://localhost:7890")
	assert.NoError(t, err)
	assert.Equal(t, "socks5", px.Scheme)
	assert.Equal(t, "localhost:7890", px.HostPort)
	assert.True(t, px.IsHealthy())
}

func TestNewProxyURLRejectsBareHost(t *testing.T) {
	_, err := proxypkg.NewProxyURL("localhost:7890")
	assert.Error(t, err)
}

func TestPoolNextSkipsUnhealthy(t *testing.T) {
	p, err := proxypkg.NewPool([]string{"socks5://localhost:7890", "socks5://localhost:7891"})
	assert.NoError(t, err)

	p.MarkUnhealthy("socks5://localhost:7890")
	assert.Equal(t, "socks5://localhost:7891", p.Next())
}

func TestPoolNextReturnsEmptyWhenAllUnhealthy(t *testing.T) {
	p, _ := proxypkg.NewPool([]string{"socks5://bad1:7890", "socks5://bad2:7890"})
	p.MarkUnhealthy("socks5://bad1:7890")
	p.MarkUnhealthy("socks5://bad2:7890")
	assert.Equal(t, "", p.Next())
}

func TestRecordSuccessIncreasesScore(t *testing.T) {
	px, _ := proxypkg.NewProxyURL("socks5://host:7890")
	assert.Equal(t, 100.0, px.HealthScore())
	for i := 0; i < 5; i++ {
		px.RecordSuccess()
	}
	assert.InDelta(t, 100.0, px.HealthScore(), 0.1)
}

func TestRecordFailureDecreasesScore(t *testing.T) {
	px, _ := proxypkg.NewProxyURL("socks5://host:7890")
	for i := 0; i < 3; i++ {
		px.RecordSuccess()
	}
	for i := 0; i < 3; i++ {
		px.RecordFailure()
	}
	assert.InDelta(t, 50.0, px.HealthScore(), 0.1)
}

func TestRecordFailureMarksUnhealthy(t *testing.T) {
	px, _ := proxypkg.NewProxyURL("socks5://host:7890")
	// 1 success + 9 failures = 10% score — should mark unhealthy
	px.RecordSuccess()
	for i := 0; i < 9; i++ {
		px.RecordFailure()
	}
	assert.False(t, px.IsHealthy())
}

func TestRecordSuccessRevivesProxy(t *testing.T) {
	px, _ := proxypkg.NewProxyURL("socks5://host:7890")
	px.RecordSuccess()
	for i := 0; i < 9; i++ {
		px.RecordFailure()
	}
	assert.False(t, px.IsHealthy())
	// 5 consecutive successes should revive (score > 50%)
	for i := 0; i < 15; i++ {
		px.RecordSuccess()
	}
	assert.True(t, px.IsHealthy())
}

func TestRemoveDeadRemovesLowScore(t *testing.T) {
	p, _ := proxypkg.NewPool([]string{"socks5://good:7890", "socks5://bad:7890"})
	// Good proxy gets 5 successes
	for i := 0; i < 5; i++ {
		p.RecordSuccessFor("socks5://good:7890")
	}
	// Bad proxy gets failures
	for i := 0; i < 10; i++ {
		p.MarkUnhealthy("socks5://bad:7890")
	}
	removed := p.RemoveDead(0.3)
	assert.Equal(t, 1, removed, "should remove the bad proxy")
	stats := p.Stats()
	assert.Equal(t, 1, stats["total"])
}

func TestStatsReturnsHealthyCounts(t *testing.T) {
	p, _ := proxypkg.NewPool([]string{"socks5://p1:7890", "socks5://p2:7890", "socks5://p3:7890"})
	p.RecordSuccessFor("socks5://p1:7890")
	p.MarkUnhealthy("socks5://p2:7890")
	p.RecordSuccessFor("socks5://p3:7890")

	stats := p.Stats()
	assert.Equal(t, 3, stats["total"])
	assert.Equal(t, 2, stats["healthy"])
	assert.Equal(t, 1, stats["dead"])
}

func TestNextSkipsUnhealthyAfterScoring(t *testing.T) {
	p, _ := proxypkg.NewPool([]string{"socks5://dead:7890", "socks5://alive:7890"})
	for i := 0; i < 10; i++ {
		p.MarkUnhealthy("socks5://dead:7890")
	}
	// Next should skip the dead one
	next := p.Next()
	assert.Equal(t, "socks5://alive:7890", next)
}
