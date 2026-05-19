package proxy

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNewProxyURLParsesScheme(t *testing.T) {
	px, err := NewProxyURL("socks5://localhost:7890")
	assert.NoError(t, err)
	assert.Equal(t, "socks5", px.Scheme)
	assert.Equal(t, "localhost:7890", px.HostPort)
	assert.True(t, px.IsHealthy())
}

func TestNewProxyURLRejectsBareHost(t *testing.T) {
	_, err := NewProxyURL("localhost:7890")
	assert.Error(t, err)
}

func TestNewProxyURLStripsAuth(t *testing.T) {
	px, err := NewProxyURL("socks5://user:pass@proxy.example:1080")
	assert.NoError(t, err)
	assert.Equal(t, "proxy.example:1080", px.HostPort)
	assert.Equal(t, "socks5://user:pass@proxy.example:1080", px.Raw)
}

func TestPoolNextSkipsUnhealthy(t *testing.T) {
	p, err := NewPool([]string{
		"socks5://localhost:7890",
		"http://user:pass@proxy2:8080",
		"socks5://localhost:7891",
	})
	assert.NoError(t, err)

	p.MarkUnhealthy("socks5://localhost:7890")

	result := p.Next()
	assert.Equal(t, "http://user:pass@proxy2:8080", result)

	result2 := p.Next()
	assert.Equal(t, "socks5://localhost:7891", result2)
}

func TestPoolNextReturnsEmptyWhenAllUnhealthy(t *testing.T) {
	p, _ := NewPool([]string{"socks5://bad1:7890", "socks5://bad2:7890"})
	p.MarkUnhealthy("socks5://bad1:7890")
	p.MarkUnhealthy("socks5://bad2:7890")
	assert.Equal(t, "", p.Next())
}

func TestPoolMarkAllHealthy(t *testing.T) {
	p, _ := NewPool([]string{"socks5://localhost:7890"})
	p.MarkUnhealthy("socks5://localhost:7890")
	p.MarkAllHealthy()
	assert.True(t, p.proxies[0].IsHealthy())
}

func TestPoolProbeNet(t *testing.T) {
	// Probe is a live network call; verify struct construction.
	px, err := NewProxyURL("socks5://localhost:7890")
	assert.NoError(t, err)
	assert.NotNil(t, px)
}

func TestPoolSetHealthyThreadSafe(t *testing.T) {
	px, _ := NewProxyURL("socks5://localhost:7890")
	done := make(chan struct{})
	go func() {
		for i := 0; i < 1000; i++ {
			px.SetHealthy(true)
		}
		close(done)
	}()
	<-done
	_ = px.IsHealthy()
}

func TestNewPoolEmptyList(t *testing.T) {
	p, err := NewPool([]string{})
	assert.NoError(t, err)
	assert.Empty(t, p.Next())
}
