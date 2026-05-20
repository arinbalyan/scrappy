package util

import (
	"errors"
	"math/rand"
	"net"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"os"
	"strconv"
	"strings"
	"sync/atomic"
	"time"
)

var defaultUA = []string{
	"Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/124.0.0.0 Safari/537.36",
	"Mozilla/5.0 (Macintosh; Intel Mac OS X 14_4) AppleWebKit/605.1.15 (KHTML, like Gecko) Version/17.4 Safari/605.1.15",
	"Mozilla/5.0 (X11; Linux x86_64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/124.0.0.0 Safari/537.36",
}

type ClientOptions struct {
	Retries               int
	CookieResetEveryN     int64
	UserAgentRotateEveryN int64
	ProxyURL              string
	ProxyRotateEveryN     int64
	ProxyStickyWindowN    int64
	UserAgents            []string
	BaseDelay             time.Duration
	MaxDelay              time.Duration
	Timeout               time.Duration
}

func NewHTTPClient(opts ClientOptions) *http.Client {
	if strings.TrimSpace(opts.ProxyURL) == "" {
		opts.ProxyURL = strings.TrimSpace(os.Getenv("SCRAPPY_PROXIES"))
	}
	if opts.ProxyRotateEveryN <= 0 {
		if v, err := strconv.ParseInt(strings.TrimSpace(os.Getenv("SCRAPPY_PROXY_ROTATE_EVERY_N")), 10, 64); err == nil && v > 0 {
			opts.ProxyRotateEveryN = v
		}
	}
	if opts.ProxyStickyWindowN <= 0 {
		if v, err := strconv.ParseInt(strings.TrimSpace(os.Getenv("SCRAPPY_PROXY_STICKY_WINDOW_N")), 10, 64); err == nil && v > 0 {
			opts.ProxyStickyWindowN = v
		}
	}
	if opts.Timeout <= 0 {
		opts.Timeout = 20 * time.Second
	}
	if opts.BaseDelay <= 0 {
		opts.BaseDelay = 300 * time.Millisecond
	}
	if opts.MaxDelay <= 0 {
		opts.MaxDelay = 4 * time.Second
	}
	if opts.CookieResetEveryN <= 0 {
		opts.CookieResetEveryN = 200
	}
	if opts.UserAgentRotateEveryN <= 0 {
		opts.UserAgentRotateEveryN = 1
	}
	if opts.ProxyStickyWindowN <= 0 {
		opts.ProxyStickyWindowN = 20
	}
	if len(opts.UserAgents) == 0 {
		opts.UserAgents = defaultUA
	}

	jar, _ := cookiejar.New(nil)
	base := &http.Transport{
		Proxy:               http.ProxyFromEnvironment,
		DialContext:         (&net.Dialer{Timeout: 8 * time.Second, KeepAlive: 30 * time.Second}).DialContext,
		TLSHandshakeTimeout: 8 * time.Second,
		MaxIdleConns:        100,
		MaxIdleConnsPerHost: 10,
		IdleConnTimeout:     90 * time.Second,
	}
	proxyList := parseProxyList(opts.ProxyURL)
	if len(proxyList) > 0 {
		base.Proxy = http.ProxyURL(proxyList[0])
	}
	rt := &smartRT{base: base, opts: opts, jar: jar, proxyList: proxyList}
	return &http.Client{Timeout: opts.Timeout, Transport: rt, Jar: jar}
}

type smartRT struct {
	base      *http.Transport
	opts      ClientOptions
	jar       http.CookieJar
	count     int64
	uaCount   int64
	uaIndex   int64
	proxyList []*url.URL
	proxyIdx  int64
	proxyReqN int64
}

func (s *smartRT) RoundTrip(req *http.Request) (*http.Response, error) {
	if req.Header.Get("User-Agent") == "" {
		req.Header.Set("User-Agent", s.nextUserAgent())
	}
	Debug("http_roundtrip_start", map[string]any{"method": req.Method, "url": req.URL.String()})

	attempts := s.opts.Retries + 1
	if attempts < 1 {
		attempts = 1
	}
	if !isRetryableMethod(req.Method) {
		attempts = 1
	}

	var lastErr error
	for i := 0; i < attempts; i++ {
		s.maybeRotateProxy()
		attemptReq := req.Clone(req.Context())
		resp, err := s.base.RoundTrip(attemptReq)
		if err == nil && resp != nil && resp.StatusCode != http.StatusTooManyRequests && resp.StatusCode < 500 {
			Debug("http_roundtrip_success", map[string]any{"method": req.Method, "url": req.URL.String(), "status": resp.StatusCode, "attempt": i + 1})
			s.maybeResetCookies(attemptReq)
			return resp, nil
		}
		if err == nil && resp != nil {
			if resp.StatusCode >= 400 && resp.StatusCode < 500 {
				APIMiss("http_client_api_miss", map[string]any{"method": req.Method, "url": req.URL.String(), "status": resp.StatusCode, "attempt": i + 1})
			}
			resp.Body.Close()
		}
		if err != nil {
			Warn("http_roundtrip_error_retry", map[string]any{"method": req.Method, "url": req.URL.String(), "attempt": i + 1, "err": err.Error()})
			lastErr = err
		} else {
			Warn("http_roundtrip_status_retry", map[string]any{"method": req.Method, "url": req.URL.String(), "attempt": i + 1, "status": resp.StatusCode})
			lastErr = errors.New("retryable status")
		}
		d := s.retryDelay(i, nil)
		if resp != nil {
			d = s.retryDelay(i, resp)
		}
		Debug("http_retry_backoff", map[string]any{"method": req.Method, "url": req.URL.String(), "attempt": i + 1, "sleep_ms": d.Milliseconds()})
		time.Sleep(d)
	}
	if lastErr != nil {
		Error("http_roundtrip_failed", map[string]any{"method": req.Method, "url": req.URL.String(), "err": lastErr.Error()})
	}
	return nil, lastErr
}

func isRetryableMethod(method string) bool {
	switch strings.ToUpper(strings.TrimSpace(method)) {
	case http.MethodGet, http.MethodHead, http.MethodOptions:
		return true
	default:
		return false
	}
}

func (s *smartRT) nextUserAgent() string {
	if len(s.opts.UserAgents) == 0 {
		return "Mozilla/5.0"
	}
	if s.opts.UserAgentRotateEveryN <= 1 {
		idx := rand.Intn(len(s.opts.UserAgents))
		return s.opts.UserAgents[idx]
	}
	c := atomic.AddInt64(&s.uaCount, 1)
	if c == 1 {
		idx := rand.Intn(len(s.opts.UserAgents))
		atomic.StoreInt64(&s.uaIndex, int64(idx))
		return s.opts.UserAgents[idx]
	}
	if c%s.opts.UserAgentRotateEveryN == 0 {
		next := (atomic.LoadInt64(&s.uaIndex) + 1) % int64(len(s.opts.UserAgents))
		atomic.StoreInt64(&s.uaIndex, next)
	}
	return s.opts.UserAgents[atomic.LoadInt64(&s.uaIndex)]
}

func (s *smartRT) maybeResetCookies(req *http.Request) {
	n := atomic.AddInt64(&s.count, 1)
	if n%s.opts.CookieResetEveryN != 0 || s.jar == nil || req.URL == nil {
		return
	}
	s.jar.SetCookies(req.URL, nil)
}

func parseProxyList(raw string) []*url.URL {
	parts := strings.Split(raw, ",")
	out := make([]*url.URL, 0, len(parts))
	for _, p := range parts {
		v := strings.TrimSpace(p)
		if v == "" {
			continue
		}
		u, err := url.Parse(v)
		if err != nil || u == nil || u.Scheme == "" || u.Host == "" {
			continue
		}
		out = append(out, u)
	}
	return out
}

func (s *smartRT) maybeRotateProxy() {
	if len(s.proxyList) <= 1 {
		return
	}
	stickyN := s.opts.ProxyStickyWindowN
	if stickyN <= 0 {
		stickyN = 1
	}
	n := atomic.AddInt64(&s.proxyReqN, 1)
	if n == 1 || (s.opts.ProxyRotateEveryN > 0 && n%s.opts.ProxyRotateEveryN == 0) || n%stickyN == 0 {
		next := (atomic.LoadInt64(&s.proxyIdx) + 1) % int64(len(s.proxyList))
		atomic.StoreInt64(&s.proxyIdx, next)
		s.base.Proxy = http.ProxyURL(s.proxyList[next])
	}
}

func (s *smartRT) retryDelay(attempt int, resp *http.Response) time.Duration {
	d := s.opts.BaseDelay * time.Duration(1<<attempt)
	if d > s.opts.MaxDelay {
		d = s.opts.MaxDelay
	}
	jitter := time.Duration(rand.Intn(120)) * time.Millisecond
	if resp != nil {
		if resp.StatusCode == http.StatusTooManyRequests {
			jitter += 300 * time.Millisecond
		}
		if resp.StatusCode >= 500 {
			jitter += 80 * time.Millisecond
		}
	}
	return d + jitter
}
