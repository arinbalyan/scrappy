package proxy

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"
)

// ProxyURL holds the parsed proxy address and its health state.
type ProxyURL struct {
	Raw      string // e.g. socks5://localhost:7890 or http://user:pass@host:8080
	Scheme   string // "socks5" | "http" | "https"
	HostPort string // "localhost:7890"
	Healthy  bool
	Mu       sync.RWMutex
}

func redactProxyURL(raw string) string {
	if u, err := url.Parse(raw); err == nil {
		return u.Redacted()
	}
	return raw
}

func NewProxyURL(raw string) (*ProxyURL, error) {
	if !strings.Contains(raw, "://") {
		return nil, fmt.Errorf("proxy URL missing scheme: %q", redactProxyURL(raw))
	}
	scheme, rest, ok := strings.Cut(raw, "://")
	if !ok {
		return nil, fmt.Errorf("cannot parse scheme from %q", redactProxyURL(raw))
	}
	// strip optional auth prefix
	hostPort := rest
	if at := strings.Index(rest, "@"); at >= 0 {
		hostPort = rest[at+1:]
	}
	return &ProxyURL{Raw: raw, Scheme: scheme, HostPort: hostPort, Healthy: true}, nil
}

// IsHealthy reports current health state (thread-safe read).
func (p *ProxyURL) IsHealthy() bool {
	p.Mu.RLock()
	defer p.Mu.RUnlock()
	return p.Healthy
}

// SetHealthy updates the health state.
func (p *ProxyURL) SetHealthy(h bool) {
	p.Mu.Lock()
	defer p.Mu.Unlock()
	p.Healthy = h
}

// Pool rotates through a list of proxies, marking unhealthy ones dead.
type Pool struct {
	proxies []*ProxyURL
	idx     int
	mu      sync.Mutex
}

func NewPool(rawList []string) (*Pool, error) {
	var proxies []*ProxyURL
	for _, r := range rawList {
		p, err := NewProxyURL(r)
		if err != nil {
			return nil, err
		}
		proxies = append(proxies, p)
	}
	return &Pool{proxies: proxies, idx: 0}, nil
}

// Next returns the next proxy URL in round-robin order, skipping unhealthy ones.
// Returns empty string if no proxy is healthy.
func (p *Pool) Next() string {
	p.mu.Lock()
	defer p.mu.Unlock()
	n := len(p.proxies)
	if n == 0 {
		return ""
	}
	for i := 0; i < n; i++ {
		candidate := p.proxies[p.idx%n]
		p.idx++
		if candidate.IsHealthy() {
			return candidate.Raw
		}
	}
	return ""
}

// MarkUnhealthy marks a proxy as unhealthy by matching its raw string.
func (p *Pool) MarkUnhealthy(raw string) {
	p.mu.Lock()
	defer p.mu.Unlock()
	for _, px := range p.proxies {
		if px.Raw == raw {
			px.SetHealthy(false)
		}
	}
}

// MarkAllHealthy resets all proxies to healthy (used at start of a new run).
func (p *Pool) MarkAllHealthy() {
	p.mu.Lock()
	defer p.mu.Unlock()
	for _, px := range p.proxies {
		px.SetHealthy(true)
	}
}

// Probe checks whether a proxy can make an outbound HTTP request by doing a HEAD
// to httpbin.org/ip through it. The caller should run this in a goroutine.
func (p *Pool) Probe(ctx context.Context, px *ProxyURL) bool {
	req, err := http.NewRequestWithContext(ctx, http.MethodHead, "https://httpbin.org/ip", nil)
	if err != nil {
		return false
	}
	req.Header.Set("User-Agent", "scrappy-probe/1.0")

	proxyURL, perr := url.Parse(px.Raw)
	if perr != nil {
		return false
	}
	client := &http.Client{
		Timeout: 5 * time.Second,
		Transport: &http.Transport{
			Proxy: func(*http.Request) (*url.URL, error) {
				return proxyURL, nil
			},
		},
	}
	resp, err := client.Do(req)
	if err != nil {
		return false
	}
	resp.Body.Close()
	return resp.StatusCode >= 200 && resp.StatusCode < 400
}
