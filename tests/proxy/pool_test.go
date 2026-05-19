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
