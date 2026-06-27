package scrappy

import "errors"

// Error sentinels — consumers can check these to distinguish failure modes
// without parsing log text.
//
// Not every scraper returns these directly; the engine classifies errors
// into kinds (see ErrorKind) and surfaces them in SiteResult.Kind.
var (
	ErrRateLimited = errors.New("rate limited")
	ErrBlocked     = errors.New("blocked by WAF/anti-bot")
	ErrAuthFailure = errors.New("authentication failed")
	ErrTimeout     = errors.New("request timeout")
	ErrNoJobs      = errors.New("no jobs found")
)

// ErrorKind classifies a scraped site error into a machine-readable category.
// Used in SiteResult.Kind so consumers can react programmatically.
func ErrorKind(err error) string {
	if err == nil {
		return ""
	}
	e := err.Error()
	switch {
	case containsAny(e, "captcha", "cloudflare", "attention required", "bot", "blocked", "datadome", "waf"):
		return "blocked"
	case containsAny(e, "429", "too many requests", "rate"):
		return "rate_limited"
	case containsAny(e, "403", "401", "forbidden", "unauthorized", "auth", "api key", "missing env"):
		return "auth_failure"
	case containsAny(e, "timeout", "deadline exceeded"):
		return "timeout"
	case containsAny(e, "no jobs", "no parseable", "no results"):
		return "no_jobs"
	default:
		return "other"
	}
}
