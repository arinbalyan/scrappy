package email

import (
	"fmt"
	"sort"
	"strings"
)

// CommonPatterns returns the corporate email patterns in hit-rate order.
// Source: prospeo.io/s/email-permutator (combines Tomba + internal data).
// Order: first.last > flast > firstlast > first > first_last > f.last > last.first
// Ponytail: a hard-coded slice, no config, no per-domain override here.
func CommonPatterns() []string {
	return []string{
		"{first}.{last}",
		"{f}{last}",
		"{first}{last}",
		"{first}",
		"{first}_{last}",
		"{f}.{last}",
		"{last}.{first}",
		"{first}-{last}",
		"{first}{l}",
		"{f}{l}",
	}
}

// PatternTokens used in templates.
const (
	tokFirst = "{first}"
	tokF     = "{f}"
	tokLast  = "{last}"
	tokL     = "{l}"
)

// Permute generates candidate addresses from firstName, lastName, domain
// and patterns. Empty parts are skipped (e.g. Permute("john","","acme.com")
// returns {john@acme.com}). Returns at most len(patterns) addresses,
// deduplicated, in input pattern order.
//
// Names are lowercased and stripped of diacritics is left to the caller;
// this function does only the most minimal normalisation (lowercase +
// trim) so a caller that wants to preserve a custom normalisation can do
// so before calling.
//
// If domain is empty, returns nil (no candidates). If both firstName and
// lastName are empty, returns nil.
func Permute(first, last, domain string, patterns []string) []string {
	first = strings.ToLower(strings.TrimSpace(first))
	last = strings.ToLower(strings.TrimSpace(last))
	domain = strings.ToLower(strings.TrimSpace(domain))

	if domain == "" || (first == "" && last == "") {
		return nil
	}

	if len(patterns) == 0 {
		patterns = CommonPatterns()
	}

	seen := make(map[string]bool, len(patterns))
	out := make([]string, 0, len(patterns))

	for _, p := range patterns {
		cand := applyPattern(p, first, last)
		if cand == "" {
			continue
		}
		addr := cand + "@" + domain
		if seen[addr] {
			continue
		}
		seen[addr] = true
		out = append(out, addr)
	}
	return out
}

// applyPattern expands a single pattern with the given name parts.
// Returns "" if a required token is missing (e.g. {last} requested but
// last is empty). Tokens are case-insensitive; the function lowercases
// the first character of {first}/{last} before expansion.
func applyPattern(pattern, first, last string) string {
	if strings.Contains(pattern, tokFirst) && first == "" {
		return ""
	}
	if strings.Contains(pattern, tokLast) && last == "" {
		return ""
	}
	if strings.Contains(pattern, tokF) && first == "" {
		return ""
	}
	if strings.Contains(pattern, tokL) && last == "" {
		return ""
	}

	var b strings.Builder
	for i := 0; i < len(pattern); {
		// Match a token or a literal segment.
		switch {
		case strings.HasPrefix(pattern[i:], tokFirst):
			b.WriteString(first)
			i += len(tokFirst)
		case strings.HasPrefix(pattern[i:], tokF):
			b.WriteString(firstLetter(first))
			i += len(tokF)
		case strings.HasPrefix(pattern[i:], tokLast):
			b.WriteString(last)
			i += len(tokLast)
		case strings.HasPrefix(pattern[i:], tokL):
			b.WriteString(firstLetter(last))
			i += len(tokL)
		default:
			b.WriteByte(pattern[i])
			i++
		}
	}
	return b.String()
}

// firstLetter returns the first character of s, or "" if s is empty.
func firstLetter(s string) string {
	if s == "" {
		return ""
	}
	return s[:1]
}

// PatternSample is one (first, last, addr) entry used by InferPattern.
type PatternSample struct {
	First, Last, Addr string
}

// InferPattern returns the most likely pattern for a domain given two
// known (firstName, lastName) -> addr samples. Returns "" if no two-sample
// match is possible (different patterns, or fewer than 2 samples).
//
// The function tests every CommonPattern against each sample and finds
// the one that matches all samples. If multiple patterns match (which
// can happen for short or empty names), the first match in
// CommonPatterns order wins.
func InferPattern(known map[string][2]string) string {
	if len(known) < 2 {
		return ""
	}

	// Sort samples for determinism.
	samples := make([]PatternSample, 0, len(known))
	for addr, fl := range known {
		samples = append(samples, PatternSample{
			First: strings.ToLower(strings.TrimSpace(fl[0])),
			Last:  strings.ToLower(strings.TrimSpace(fl[1])),
			Addr:  strings.ToLower(strings.TrimSpace(addr)),
		})
	}
	sort.Slice(samples, func(i, j int) bool {
		return samples[i].Addr < samples[j].Addr
	})

	for _, p := range CommonPatterns() {
		if matchesAll(p, samples) {
			return p
		}
	}
	return ""
}

// matchesAll returns true if applying the pattern to every sample
// reproduces the sample's local part. Used by InferPattern.
func matchesAll(pattern string, samples []PatternSample) bool {
	for _, s := range samples {
		cand := applyPattern(pattern, s.First, s.Last)
		if cand == "" {
			return false
		}
		// Compare against the local part of the sample address.
		at := strings.Index(s.Addr, "@")
		if at < 0 {
			return false
		}
		local := s.Addr[:at]
		if cand != local {
			return false
		}
	}
	return true
}

// String returns a human-readable summary of the pattern set.
func PatternsString() string {
	return strings.Join(CommonPatterns(), ", ")
}

// FormatPattern is a convenience: Permute with a single pattern.
func FormatPattern(first, last, domain, pattern string) string {
	out := Permute(first, last, domain, []string{pattern})
	if len(out) == 0 {
		return ""
	}
	return out[0]
}

// ValidatePattern returns nil if the pattern is in the supported set,
// otherwise an error explaining why.
func ValidatePattern(p string) error {
	if p == "" {
		return fmt.Errorf("empty pattern")
	}
	for _, supported := range CommonPatterns() {
		if p == supported {
			return nil
		}
	}
	return fmt.Errorf("unsupported pattern %q (supported: %s)", p, PatternsString())
}
