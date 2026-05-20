package util

import (
	"errors"
	"math/rand"
	"net"
	"net/http"
	"net/http/cookiejar"
	"net/url"
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
	Retries            int
	CookieResetEveryN  int64
	UserAgentRotateEveryN int64
	ProxyURL           string
	UserAgents         []string
	BaseDelay          time.Duration
	MaxDelay           time.Duration
	Timeout            time.Duration
}

func NewHTTPClient(opts ClientOptions) *http.Client {
	if opts.Timeout <= 0 { opts.Timeout = 20 * time.Second }
	if opts.BaseDelay <= 0 { opts.BaseDelay = 300 * time.Millisecond }
	if opts.MaxDelay <= 0 { opts.MaxDelay = 4 * time.Second }
	if opts.CookieResetEveryN <= 0 { opts.CookieResetEveryN = 200 }
	if opts.UserAgentRotateEveryN <= 0 { opts.UserAgentRotateEveryN = 1 }
	if len(opts.UserAgents) == 0 { opts.UserAgents = defaultUA }

	jar, _ := cookiejar.New(nil)
	base := &http.Transport{
		Proxy: http.ProxyFromEnvironment,
		DialContext: (&net.Dialer{Timeout: 8 * time.Second, KeepAlive: 30 * time.Second}).DialContext,
		TLSHandshakeTimeout: 8 * time.Second,
		MaxIdleConns: 100,
		MaxIdleConnsPerHost: 10,
		IdleConnTimeout: 90 * time.Second,
	}
	if strings.TrimSpace(opts.ProxyURL) != "" {
		if proxyURL, err := url.Parse(strings.TrimSpace(opts.ProxyURL)); err == nil {
			base.Proxy = http.ProxyURL(proxyURL)
		}
	}
	rt := &smartRT{base: base, opts: opts, jar: jar}
	return &http.Client{Timeout: opts.Timeout, Transport: rt, Jar: jar}
}

type smartRT struct {
	base    http.RoundTripper
	opts    ClientOptions
	jar     http.CookieJar
	count   int64
	uaCount int64
	uaIndex int64
}

func (s *smartRT) RoundTrip(req *http.Request) (*http.Response, error) {
	if req.Header.Get("User-Agent") == "" {
		req.Header.Set("User-Agent", s.nextUserAgent())
	}

	attempts := s.opts.Retries + 1
	if attempts < 1 {
		attempts = 1
	}
	if !isRetryableMethod(req.Method) {
		attempts = 1
	}

	var lastErr error
	for i := 0; i < attempts; i++ {
		attemptReq := req.Clone(req.Context())
		resp, err := s.base.RoundTrip(attemptReq)
		if err == nil && resp != nil && resp.StatusCode != http.StatusTooManyRequests && resp.StatusCode < 500 {
			s.maybeResetCookies(attemptReq)
			return resp, nil
		}
		if err == nil && resp != nil {
			resp.Body.Close()
		}
		if err != nil {
			lastErr = err
		} else {
			lastErr = errors.New("retryable status")
		}
		d := s.opts.BaseDelay * time.Duration(1<<i)
		if d > s.opts.MaxDelay {
			d = s.opts.MaxDelay
		}
		time.Sleep(d + time.Duration(rand.Intn(120))*time.Millisecond)
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
	if n%s.opts.CookieResetEveryN != 0 || s.jar == nil || req.URL == nil { return }
	s.jar.SetCookies(req.URL, nil)
}
