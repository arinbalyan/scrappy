// Package quality provides a deterministic quality score (0-100) for job postings.
//
// Score factors (100 points total):
//
//  20  Salary mentioned (non-zero min or max amount)
//  15  Direct or external apply link present
//  15  Email domain matches company domain
//  15  Freshness (scaled): < 24h=15, < 72h=10, < 7d=5, older=0
//  10  At least one MX-verified email address
//  10  Description length (scaled): > 2000=10, > 500=7, > 200=5
//  10  NOT a staffing/agency posting
//   5  Two or more distinct email addresses
//     ---
// 100  Total
package quality

import (
	"strings"
	"time"
	"unicode"

	"github.com/arinbalyan/scrappy/internal/model"
)

const maxScore = 100

// Score computes a deterministic quality score (0-100) for a single job posting.
// Returns 0 for a nil receiver.
func Score(job *model.JobPost) int {
	if job == nil {
		return 0
	}
	score := 0

	if hasSalary(job) {
		score += 20
	}

	if hasDirectApply(job) {
		score += 15
	}

	if emailMatchesCompanyDomain(job) {
		score += 15
	}

	score += freshnessScore(job)

	score += verifiedEmailScore(job)

	score += descriptionLengthScore(job)

	if !isAgency(job) {
		score += 10
	}

	if multipleEmails(job) {
		score += 5
	}

	if score > maxScore {
		score = maxScore
	}
	if score < 0 {
		score = 0
	}
	return score
}

// hasSalary returns true when the job lists a non-zero salary range.
func hasSalary(job *model.JobPost) bool {
	if job.Compensation == nil {
		return false
	}
	if job.Compensation.MinAmount != nil && *job.Compensation.MinAmount > 0 {
		return true
	}
	return job.Compensation.MaxAmount != nil && *job.Compensation.MaxAmount > 0
}

// hasDirectApply returns true for known direct-apply method strings.
func hasDirectApply(job *model.JobPost) bool {
	switch strings.ToLower(strings.TrimSpace(job.ApplyMethod)) {
	case "easy_apply", "email", "direct_url", "direct", "external_url":
		return true
	default:
		return false
	}
}

// emailMatchesCompanyDomain returns true when at least one email's domain
// matches the job's company domain or is a subdomain of it.
func emailMatchesCompanyDomain(job *model.JobPost) bool {
	if len(job.Emails) == 0 || job.Domain == "" {
		return false
	}
	domain := strings.ToLower(strings.TrimSpace(job.Domain))
	for _, email := range job.Emails {
		addr := strings.ToLower(strings.TrimSpace(email.Addr))
		parts := strings.Split(addr, "@")
		if len(parts) != 2 {
			continue
		}
		emailDomain := parts[1]
		if emailDomain == domain || strings.HasSuffix(emailDomain, "."+domain) {
			return true
		}
	}
	return false
}

// freshnessScore returns points based on how recently the job was posted.
//   < 24h : 15
//   < 72h : 10
//   < 7d  :  5
//   older :  0
// Future dates (clock skew, timezone mismatch) score 0.
func freshnessScore(job *model.JobPost) int {
	if job.DatePosted == nil || job.DatePosted.IsZero() {
		return 0
	}
	diff := time.Since(*job.DatePosted)
	switch {
	case diff < 0:
		// Future date — treat as unknown freshness, score 0.
		return 0
	case diff <= 24*time.Hour:
		return 15
	case diff <= 72*time.Hour:
		return 10
	case diff <= 7*24*time.Hour:
		return 5
	default:
		return 0
	}
}

// verifiedEmailScore returns 10 when at least one email has Verified=true.
func verifiedEmailScore(job *model.JobPost) int {
	for _, email := range job.Emails {
		if email.Verified {
			return 10
		}
	}
	return 0
}

// descriptionLengthScore returns points for description length:
//   > 2000 chars : 10
//   > 500  chars :  7
//   > 200  chars :  5
//   ≤ 200  chars :  0
func descriptionLengthScore(job *model.JobPost) int {
	n := len(strings.TrimSpace(job.Description))
	switch {
	case n > 2000:
		return 10
	case n > 500:
		return 7
	case n > 200:
		return 5
	default:
		return 0
	}
}

// multipleEmails returns true when the job has at least two distinct email addresses.
func multipleEmails(job *model.JobPost) bool {
	seen := make(map[string]bool)
	count := 0
	for _, e := range job.Emails {
		addr := strings.ToLower(strings.TrimSpace(e.Addr))
		if addr == "" || seen[addr] {
			continue
		}
		seen[addr] = true
		count++
	}
	return count >= 2
}

// agencyDomains lists known staffing/agency company domains.
var agencyDomains = []string{
	"aerotek.com",
	"adecco.com",
	"ciber.com",
	"collateraledge.com",
	"experis.com",
	"hays.com",
	"insightglobal.com",
	"kellyservices.com",
	"kforce.com",
	"manpower.com",
	"michaelpage.com",
	"modis.com",
	"randstad.com",
	"roberthalf.com",
	"robertwalters.com",
	"spencerogden.com",
	"teksystems.com",
}

// isAgency returns true when the job's company domain or name suggests a
// staffing / recruiting agency rather than a direct employer.
func isAgency(job *model.JobPost) bool {
	domain := strings.ToLower(strings.TrimSpace(job.Domain))
	if domain != "" {
		for _, d := range agencyDomains {
			if domain == d || strings.HasSuffix(domain, "."+d) {
				return true
			}
		}
	}

	name := strings.ToLower(strings.TrimSpace(job.CompanyName))
	return hasToken(name, "staffing") ||
		hasToken(name, "recruiting") ||
		hasToken(name, "recruitment") ||
		hasToken(name, "agency") ||
		hasToken(name, "talent") ||
		hasToken(name, "workforce") ||
		hasToken(name, "placement")
}

// hasToken reports whether token appears as a standalone word in s.
func hasToken(s, token string) bool {
	fields := strings.FieldsFunc(s, func(r rune) bool {
		return !unicode.IsLetter(r) && !unicode.IsNumber(r)
	})
	for _, f := range fields {
		if f == token {
			return true
		}
	}
	return false
}
