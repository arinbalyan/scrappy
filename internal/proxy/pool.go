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

// ProxyURL holds the parsed proxy address and its health state with scoring.
type ProxyURL struct {
	Raw      string
	Scheme   string
	HostPort string
	Healthy  bool

	// Health scoring
	Successes int     `json:"successes"`
	Failures  int     `json:"failures"`
	Score     float64 `json:"score"` // 0.0 to 1.0

	mu sync.RWMutex
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
	hostPort := rest
	if at := strings.Index(rest, "@"); at >= 0 {
		hostPort = rest[at+1:]
	}
	return &ProxyURL{Raw: raw, Scheme: scheme, HostPort: hostPort, Healthy: true, Score: 1.0}, nil
}

// IsHealthy reports current health state (thread-safe read).
func (p *ProxyURL) IsHealthy() bool {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.Healthy
}

// SetHealthy updates the health state.
func (p *ProxyURL) SetHealthy(h bool) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.Healthy = h
}

// RecordSuccess increments success counter and updates health score.
func (p *ProxyURL) RecordSuccess() {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.Successes++
	total := p.Successes + p.Failures
	if total > 0 {
		p.Score = float64(p.Successes) / float64(total)
	}
	if p.Score >= 0.5 && !p.Healthy {
		p.Healthy = true
	}
}

// RecordFailure increments failure counter, updates score, and marks unhealthy if below threshold.
func (p *ProxyURL) RecordFailure() {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.Failures++
	total := p.Successes + p.Failures
	if total > 0 {
		p.Score = float64(p.Successes) / float64(total)
	}
	// Mark unhealthy if < 25% success rate with at least 5 requests
	if total >= 5 && p.Score < 0.25 {
		p.Healthy = false
	}
}

// HealthScore returns the success ratio as a percentage (0-100).
func (p *ProxyURL) HealthScore() float64 {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.Score * 100
}

// Pool rotates through a list of proxies with health scoring.
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

// Next returns the next healthy proxy URL in round-robin order.
// Returns empty string if no proxy is healthy.
func (p *Pool) Next() string {
	p.mu.Lock()
	defer p.mu.Unlock()
	n := len(p.proxies)
	if n == 0 {
		return ""
	}
	start := p.idx
	for i := 0; i < n; i++ {
		candidate := p.proxies[p.idx%n]
		p.idx++
		if candidate.IsHealthy() {
			return candidate.Raw
		}
		// Avoid infinite loop on fully dead pool
		if p.idx%n == start%n {
			break
		}
	}
	return ""
}

// MarkUnhealthy immediately marks a proxy as unhealthy and records a failure.
func (p *Pool) MarkUnhealthy(raw string) {
	p.mu.Lock()
	defer p.mu.Unlock()
	for _, px := range p.proxies {
		if px.Raw == raw {
			px.Failures++
			px.Score = float64(px.Successes) / float64(px.Successes+px.Failures)
			px.Healthy = false
			break
		}
	}
}

// RecordSuccessFor records a successful request for a proxy.
func (p *Pool) RecordSuccessFor(raw string) {
	p.mu.Lock()
	defer p.mu.Unlock()
	for _, px := range p.proxies {
		if px.Raw == raw {
			px.RecordSuccess()
			break
		}
	}
}

// MarkAllHealthy resets all proxies to healthy and resets scores.
func (p *Pool) MarkAllHealthy() {
	p.mu.Lock()
	defer p.mu.Unlock()
	for _, px := range p.proxies {
		px.Healthy = true
		px.Successes = 0
		px.Failures = 0
		px.Score = 1.0
	}
}

// RemoveDead removes proxies whose health score is below the threshold.
// Returns the number of removed proxies.
func (p *Pool) RemoveDead(threshold float64) int {
	p.mu.Lock()
	defer p.mu.Unlock()
	before := len(p.proxies)
	alive := make([]*ProxyURL, 0, before)
	for _, px := range p.proxies {
		score := px.Score
		if score >= threshold || (px.Successes+px.Failures) < 3 {
			alive = append(alive, px)
		}
	}
	p.proxies = alive
	return before - len(alive)
}

// Stats returns a summary of pool health.
func (p *Pool) Stats() map[string]any {
	p.mu.Lock()
	defer p.mu.Unlock()
	total := len(p.proxies)
	healthy := 0
	var avgScore float64
	for _, px := range p.proxies {
		if px.Healthy {
			healthy++
		}
		avgScore += px.Score
	}
	if total > 0 {
		avgScore /= float64(total)
	}
	return map[string]any{
		"total":      total,
		"healthy":    healthy,
		"dead":       total - healthy,
		"avg_score":  fmt.Sprintf("%.1f%%", avgScore*100),
	}
}

// Probe checks whether a proxy can make an outbound HTTP request.
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

// ProbeAndRecover tests all unhealthy proxies and revives any that respond.
func (p *Pool) ProbeAndRecover(ctx context.Context) int {
	p.mu.Lock()
	dead := make([]*ProxyURL, 0)
	for _, px := range p.proxies {
		if !px.Healthy {
			dead = append(dead, px)
		}
	}
	p.mu.Unlock()

	revived := 0
	for _, px := range dead {
		if p.Probe(ctx, px) {
			px.SetHealthy(true)
			px.RecordSuccess()
			revived++
		}
	}
	return revived
}
