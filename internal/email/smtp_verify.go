package email

import (
	"context"
	"strings"
	"sync"
	"time"

	emailverifier "github.com/AfterShip/email-verifier"
)

// SMTPVerifier performs SMTP RCPT TO verification of email addresses via
// the AfterShip email-verifier library. It returns a richer result than the
// DNS-only MXVerifier: deliverable (RCPT TO 250), catch-all (server returns
// 250 for any mailbox), and the SMTP reason string.
//
// Ponytail: 2-step (MX then SMTP), bounded goroutine pool, context-aware.
// Gmail/Outlook caveat: both providers return 250 for every well-formed
// address. The verifier reports catch_all=true in that case; the caller
// decides how to weight the result.
type SMTPVerifier struct {
	verifier      *emailverifier.Verifier
	concurrency   int
	connectTimeout time.Duration
	helloName     string
	fromEmail     string
	mu            sync.Mutex // guards verifier reconfiguration
}

// SMTPResult is the outcome of a single SMTP verification. Mirrors the
// fields we care about from emailverifier.Result.
type SMTPResult struct {
	Deliverable bool
	CatchAll    bool
	HasMX       bool
	HostExists  bool
	Reason      string
	Free        bool
	RoleAccount bool
	Disposable  bool
	Duration    time.Duration
}

// NewSMTPVerifier returns a verifier with sensible defaults:
// 10-second connect timeout, 3-concurrent worker pool, SMTP check enabled,
// catch-all check enabled (so the result distinguishes catch-all from
// true deliverable).
func NewSMTPVerifier() *SMTPVerifier {
	v := emailverifier.NewVerifier()
	v.EnableSMTPCheck()
	v.EnableCatchAllCheck()
	return &SMTPVerifier{
		verifier:       v,
		concurrency:    3,
		connectTimeout: 10 * time.Second,
		helloName:      "scrappy.local",
		fromEmail:      "verify@scrappy.local",
	}
}

// WithConcurrency sets the per-call worker pool size. Minimum 1.
func (s *SMTPVerifier) WithConcurrency(n int) *SMTPVerifier {
	if n < 1 {
		n = 1
	}
	s.mu.Lock()
	s.concurrency = n
	s.mu.Unlock()
	return s
}

// WithHelloName sets the EHLO/MAIL FROM identity used during SMTP
// transactions. Some providers reject unknown identities.
func (s *SMTPVerifier) WithHelloName(name, fromEmail string) *SMTPVerifier {
	s.mu.Lock()
	s.helloName = name
	s.fromEmail = fromEmail
	s.verifier.HelloName(name)
	s.verifier.FromEmail(fromEmail)
	s.mu.Unlock()
	return s
}

// WithTimeout sets the SMTP connect and operation timeouts.
func (s *SMTPVerifier) WithTimeout(connect, operation time.Duration) *SMTPVerifier {
	s.mu.Lock()
	s.connectTimeout = connect
	s.verifier.ConnectTimeout(connect)
	s.verifier.OperationTimeout(operation)
	s.mu.Unlock()
	return s
}

// Verify performs SMTP verification on a single email address. Context
// cancellation is checked once before delegation; the underlying
// AfterShip call does not honour a context, so a long SMTP transaction
// may run to completion even after ctx is cancelled.
func (s *SMTPVerifier) Verify(ctx context.Context, addr string) SMTPResult {
	addr = normalizeAddr(addr)
	if addr == "" {
		return SMTPResult{Reason: "empty_address"}
	}
	if isBlockedDomain(addr) {
		return SMTPResult{Reason: "blocked_domain"}
	}

	start := time.Now()
	res, err := s.verifier.Verify(addr)
	dur := time.Since(start)

	out := SMTPResult{Duration: dur, Reason: "smtp_ok"}
	if err != nil {
		out.Reason = "smtp_error:" + err.Error()
		return out
	}
	if res == nil {
		out.Reason = "smtp_nil_result"
		return out
	}

	out.HasMX = res.HasMxRecords
	out.Free = res.Free
	out.RoleAccount = res.RoleAccount
	out.Disposable = res.Disposable
	if res.SMTP != nil {
		out.HostExists = res.SMTP.HostExists
		out.CatchAll = res.SMTP.CatchAll
		// AfterShip already computed Deliverable (RCPT TO 250) for us.
		out.Deliverable = res.SMTP.Deliverable
		if !out.Deliverable {
			if out.CatchAll {
				// The provider returned a positive answer for a random
				// mailbox, so individual mailboxes can't be distinguished.
				// Treat as "unknown" rather than rejected.
				out.Reason = "catch_all"
			} else {
				// res.Reachable is "no" or "unknown" depending on what
				// the SMTP transaction reported.
				switch res.Reachable {
				case "no":
					out.Reason = "rcpt_550"
				default:
					out.Reason = "reachable_unknown"
				}
			}
		}
	} else {
		out.Reason = "smtp_no_response"
	}
	return out
}

// VerifyAll runs Verify concurrently across the input list, returning
// a map from address (lowercased) to result. The pool is bounded by
// the verifier's concurrency setting.
func (s *SMTPVerifier) VerifyAll(ctx context.Context, addrs []string) map[string]SMTPResult {
	out := make(map[string]SMTPResult, len(addrs))
	if len(addrs) == 0 {
		return out
	}

	s.mu.Lock()
	concurrency := s.concurrency
	if concurrency < 1 {
		concurrency = 1
	}
	s.mu.Unlock()

	// Dedup input: a downstream dedup is the caller's job; we just
	// avoid running the same address through the SMTP pipeline twice.
	unique := make([]string, 0, len(addrs))
	seen := make(map[string]bool, len(addrs))
	for _, a := range addrs {
		key := strings.ToLower(strings.TrimSpace(a))
		if key == "" || seen[key] {
			continue
		}
		seen[key] = true
		unique = append(unique, key)
	}

	type result struct {
		addr string
		res  SMTPResult
	}
	results := make(chan result, len(unique))
	sem := make(chan struct{}, concurrency)

	var wg sync.WaitGroup
	for _, a := range unique {
		if err := ctx.Err(); err != nil {
			break
		}
		wg.Add(1)
		sem <- struct{}{}
		go func(addr string) {
			defer wg.Done()
			defer func() { <-sem }()
			results <- result{addr: addr, res: s.Verify(ctx, addr)}
		}(a)
	}
	wg.Wait()
	close(results)

	for r := range results {
		out[r.addr] = r.res
	}
	return out
}
