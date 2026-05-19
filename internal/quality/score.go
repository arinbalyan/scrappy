package quality

import (
	"strings"
	"time"
	"unicode"

	"github.com/arinbalyan/scrappy/internal/model"
)

const maxScore = 100

func Score(job *model.JobPost) int {
	score := 0

	if hasSalary(job) {
		score += 30
	}

	if hasDirectApply(job) {
		score += 20
	}

	if emailMatchesCompanyDomain(job) {
		score += 15
	}

	if isFresh(job) {
		score += 15
	}

	if len(strings.TrimSpace(job.Description)) > 200 {
		score += 10
	}

	if !isAgency(job) {
		score += 10
	}

	if score > maxScore {
		score = maxScore
	}

	return score
}

func hasSalary(job *model.JobPost) bool {
	if job.Compensation == nil {
		return false
	}
	if job.Compensation.MinAmount != nil && *job.Compensation.MinAmount > 0 {
		return true
	}
	return job.Compensation.MaxAmount != nil && *job.Compensation.MaxAmount > 0
}

func hasDirectApply(job *model.JobPost) bool {
	switch strings.ToLower(strings.TrimSpace(job.ApplyMethod)) {
	case "easy_apply", "email", "direct_url", "direct", "external_url":
		return true
	default:
		return false
	}
}

func emailMatchesCompanyDomain(job *model.JobPost) bool {
	if len(job.Emails) == 0 || job.Domain == "" {
		return false
	}
	for _, email := range job.Emails {
		if strings.Contains(email.Addr, "@"+job.Domain) {
			return true
		}
	}
	return false
}

func isFresh(job *model.JobPost) bool {
	if job.DatePosted == nil || job.DatePosted.IsZero() {
		return false
	}
	return time.Since(*job.DatePosted) <= 24*time.Hour
}

func isAgency(job *model.JobPost) bool {
	agencyDomains := []string{
		"randstad.com",
		"manpower.com",
		"adecco.com",
		"kellyservices.com",
		"hays.com",
		"michaelpage.com",
		"robertwalters.com",
		"spencerogden.com",
	}

	domain := strings.ToLower(strings.TrimSpace(job.Domain))
	if domain != "" {
		for _, d := range agencyDomains {
			if domain == d || strings.HasSuffix(domain, "."+d) {
				return true
			}
		}
	}

	name := strings.ToLower(strings.TrimSpace(job.CompanyName))
	return hasToken(name, "staffing") || hasToken(name, "recruiting") || hasToken(name, "recruitment") || hasToken(name, "talent")
}

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
