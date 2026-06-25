package util

import (
	"errors"
	"fmt"
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
	dialer := &net.Dialer{Timeout: 8 * time.Second, KeepAlive: 30 * time.Second}
	base := &http.Transport{
		Proxy:               http.ProxyFromEnvironment,
		DialContext:         dialer.DialContext,
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
	// Add browser-like headers that Go's net/http doesn't send by default.
	// These help avoid WAF detection even without strict header ordering.
	if req.Header.Get("Accept") == "" {
		req.Header.Set("Accept", "text/html,application/xhtml+xml,application/xml;q=0.9,*/*;q=0.8")
	}
	if req.Header.Get("Accept-Language") == "" {
		req.Header.Set("Accept-Language", "en-US,en;q=0.9")
	}
	if req.Header.Get("Sec-Fetch-Dest") == "" {
		req.Header.Set("Sec-Fetch-Dest", "document")
	}
	if req.Header.Get("Sec-Fetch-Mode") == "" {
		req.Header.Set("Sec-Fetch-Mode", "navigate")
	}
	if req.Header.Get("Sec-Fetch-Site") == "" {
		req.Header.Set("Sec-Fetch-Site", "none")
	}
	if req.Header.Get("Sec-Fetch-User") == "" {
		req.Header.Set("Sec-Fetch-User", "?1")
	}
	if req.Header.Get("Upgrade-Insecure-Requests") == "" {
		req.Header.Set("Upgrade-Insecure-Requests", "1")
	}
	// Add Sec-Ch-Ua headers (critical for Chrome fingerprint).
	// Extract browser brand from User-Agent to match.
	if req.Header.Get("Sec-Ch-Ua") == "" {
		ua := req.Header.Get("User-Agent")
		brand := detectChromeBrand(ua)
		if brand != "" {
			req.Header.Set("Sec-Ch-Ua", brand)
		}
	}
	if req.Header.Get("Sec-Ch-Ua-Mobile") == "" {
		req.Header.Set("Sec-Ch-Ua-Mobile", "?0")
	}
	if req.Header.Get("Sec-Ch-Ua-Platform") == "" {
		req.Header.Set("Sec-Ch-Ua-Platform", detectPlatform(req.Header.Get("User-Agent")))
	}
	Debug("http_roundtrip_start", map[string]any{"method": req.Method, "url": redactURL(req.URL)})

	attempts := s.opts.Retries + 1
	if attempts < 1 {
		attempts = 1
	}
	if !isRetryableMethod(req.Method) {
		attempts = 1
	}

	var lastErr error
	var saw429 bool
	for i := 0; i < attempts; i++ {
		s.maybeRotateProxy()
		attemptReq := req.Clone(req.Context())
		resp, err := s.base.RoundTrip(attemptReq)
		if err == nil && resp != nil && !isRetryableStatus(resp.StatusCode) {
			Debug("http_roundtrip_success", map[string]any{"method": req.Method, "url": redactURL(req.URL), "status": resp.StatusCode, "attempt": i + 1})
			s.maybeResetCookies(attemptReq)
			return resp, nil
		}

		if err != nil {
			// Permanent errors (NXDOMAIN, TLS cert failure, etc.) — fail immediately,
			// because retrying won't help.
			if isPermanentError(err) {
				Error("http_roundtrip_failed_permanent", map[string]any{"method": req.Method, "url": req.URL.String(), "err": err.Error()})
				return nil, fmt.Errorf("permanent: %w", err)
			}
			Warn("http_roundtrip_error_retry", map[string]any{"method": req.Method, "url": req.URL.String(), "attempt": i + 1, "err": err.Error()})
			lastErr = err
		} else if resp != nil {
			// Repeated 429 — fail fast after the first retry, the server
			// is not going to accept us within this batch window.
			if resp.StatusCode == http.StatusTooManyRequests && saw429 {
				Error("http_roundtrip_failed_rate_limit", map[string]any{"method": req.Method, "url": req.URL.String(), "status": resp.StatusCode, "attempt": i + 1})
				resp.Body.Close()
				return nil, fmt.Errorf("permanent: rate limited (repeated 429)")
			}
			if resp.StatusCode == http.StatusTooManyRequests {
				saw429 = true
			}
			if resp.StatusCode >= 400 && resp.StatusCode < 500 {
				APIMiss("http_client_api_miss", map[string]any{"method": req.Method, "url": req.URL.String(), "status": resp.StatusCode, "attempt": i + 1})
			}
			resp.Body.Close()
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

// isPermanentError returns true for errors that will NOT resolve on retry.
// These include DNS NXDOMAIN (the domain genuinely does not exist),
// expired/mismatched TLS certificates, and unreachable networks.
func isPermanentError(err error) bool {
	if err == nil {
		return false
	}
	// DNS NXDOMAIN — the domain does not exist at all.
	var dnsErr *net.DNSError
	if errors.As(err, &dnsErr) {
		if dnsErr.IsNotFound || dnsErr.Err == "no such host" || strings.Contains(dnsErr.Err, "NXDOMAIN") {
			return true
		}
		// Non-permanent DNS errors (temporary server failures, timeouts) are transient.
		if dnsErr.IsTemporary || dnsErr.Timeout() {
			return false
		}
		// Any other DNS error is permanent (e.g. server failure on a domain that exists).
		return true
	}
	// Wrapped *net.OpError (e.g. from TLS handshake, TCP dial).
	var opErr *net.OpError
	if errors.As(err, &opErr) {
		msg := strings.ToLower(opErr.Err.Error())
		if strings.Contains(msg, "certificate") || strings.Contains(msg, "tls") || strings.Contains(msg, "x509") {
			return true
		}
		// "no route to host", "host unreachable", "network is unreachable"
		if strings.Contains(msg, "unreachable") || strings.Contains(msg, "no route") {
			return true
		}
	}
	return false
}

func isRetryableStatus(code int) bool {
	// 429 (rate-limit) and 5xx (transient server errors) are retryable.
	// 4xx client errors (403, 401, 406) are NOT retried because they
	// indicate a permanent condition — retrying wastes time and the body
	// (which may contain an anti-bot challenge) is lost.
	if code >= 500 && code < 600 {
		return true
	}
	return code == http.StatusTooManyRequests
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
		if !proxyReachable(u) {
			Warn("proxy_unreachable_skip", map[string]any{"proxy": redactURL(u)})
			continue
		}
		out = append(out, u)
	}
	return out
}

// proxyReachable checks whether the proxy host:port is reachable via TCP.
// For HTTP/HTTPS proxies this is a reasonable health probe.  For SOCKS5
// proxies a simple TCP dial only confirms the port is open — it does not
// validate that a full SOCKS handshake succeeds.  Full SOCKS handshake
// validation requires a SOCKS library and is not attempted here.
func proxyReachable(u *url.URL) bool {
	if u == nil {
		return false
	}
	host := u.Hostname()
	if host == "" {
		return false
	}
	port := u.Port()
	if port == "" {
		switch u.Scheme {
		case "http", "https":
			port = "80"
		case "socks5", "socks5h":
			port = "1080"
		default:
			port = "80"
		}
	}
	addr := net.JoinHostPort(host, port)
	c, err := net.DialTimeout("tcp", addr, 500*time.Millisecond)
	if err != nil {
		return false
	}
	_ = c.Close()
	return true
}

func (s *smartRT) maybeRotateProxy() {
	if len(s.proxyList) <= 1 {
		return
	}
	rotateEveryN := s.opts.ProxyRotateEveryN
	stickyN := s.opts.ProxyStickyWindowN
	if rotateEveryN <= 0 && stickyN <= 0 {
		return
	}

	n := atomic.AddInt64(&s.proxyReqN, 1)

	// Determine if this request should trigger a rotation.
	rotate := false
	if rotateEveryN > 0 && n%rotateEveryN == 0 {
		rotate = true
	}
	if stickyN > 0 && n%stickyN == 0 {
		rotate = true
	}

	if rotate {
		next := atomic.AddInt64(&s.proxyIdx, 1) % int64(len(s.proxyList))
		s.base.Proxy = http.ProxyURL(s.proxyList[next])
	}
}

// redactURL strips credentials from a URL for safe logging.
// Uses url.URL.Redacted() which replaces userinfo with "xxxxx".
func redactURL(u *url.URL) string {
	if u == nil {
		return ""
	}
	return u.Redacted()
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
		if resp.StatusCode == http.StatusForbidden || resp.StatusCode == http.StatusUnauthorized || resp.StatusCode == http.StatusNotAcceptable {
			jitter += 600 * time.Millisecond
		}
	}
	return d + jitter
}

// detectChromeBrand returns the Sec-CH-UA header value based on the User-Agent.
func detectChromeBrand(ua string) string {
	if ua == "" {
		return ""
	}
	// Chrome 124+
	if strings.Contains(ua, "Chrome/124") {
		return `"Chromium";v="124", "Google Chrome";v="124", "Not-A.Brand";v="99"`
	}
	if strings.Contains(ua, "Chrome/125") {
		return `"Chromium";v="125", "Google Chrome";v="125", "Not-A.Brand";v="99"`
	}
	if strings.Contains(ua, "Chrome/126") {
		return `"Chromium";v="126", "Google Chrome";v="126", "Not-A.Brand";v="99"`
	}
	if strings.Contains(ua, "Chrome/130") {
		return `"Chromium";v="130", "Google Chrome";v="130", "Not-A.Brand";v="99"`
	}
	if strings.Contains(ua, "Chrome/1") {
		return `"Chromium";v="128", "Google Chrome";v="128", "Not-A.Brand";v="99"`
	}
	// Generic Safari fallback
	if strings.Contains(ua, "Safari/") && !strings.Contains(ua, "Chrome") {
		return `"Not=A?Brand";v="99", "Safari";v="17"`
	}
	return `"Chromium";v="128", "Google Chrome";v="128", "Not-A.Brand";v="99"`
}

// detectPlatform returns the Sec-CH-UA-Platform value based on the User-Agent.
func detectPlatform(ua string) string {
	if strings.Contains(ua, "Windows") {
		return "\"Windows\""
	}
	if strings.Contains(ua, "Macintosh") || strings.Contains(ua, "Darwin") {
		return "\"macOS\""
	}
	if strings.Contains(ua, "Linux") {
		return "\"Linux\""
	}
	return "\"macOS\""
}
