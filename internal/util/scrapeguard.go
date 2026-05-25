package util

import (
	"context"
	"encoding/json"
	"fmt"
	"hash/fnv"
	"io"
	"math/rand"
	"regexp"
	"strings"
	"time"

	"github.com/arinbalyan/scrappy/internal/model"
)

// DetectAntiBotChallenge checks body bytes for known anti-bot / WAF challenge
// page markers.  Returns the challenge type ("datadome", "cloudflare",
// "javascript_required") or "" when the page appears to be legitimate HTML.
func DetectAntiBotChallenge(body []byte) string {
	if len(body) < 256 {
		return ""
	}
	s := string(body)

	switch {
	case strings.Contains(s, "x-datadome") || strings.Contains(s, "datadome") || strings.Contains(s, "#cmsg"):
		return "datadome"
	case strings.Contains(s, "__cf_chl_opt") || strings.Contains(s, "cf-mitigated") || strings.Contains(s, "/cdn-cgi/") && strings.Contains(s, "Just a moment"):
		return "cloudflare"
	case strings.Contains(s, "Please enable JavaScript") || strings.Contains(s, "enable javascript") || strings.Contains(s, "enable JS") || strings.Contains(s, "js challenge") || strings.Contains(s, "js-support"):
		return "javascript_required"
	default:
		return ""
	}
}

const DefaultMaxBodyBytes int64 = 4 * 1024 * 1024

func ReadBodyLimited(r io.Reader, maxBytes int64) ([]byte, error) {
	if maxBytes <= 0 {
		maxBytes = DefaultMaxBodyBytes
	}
	lr := io.LimitReader(r, maxBytes+1)
	b, err := io.ReadAll(lr)
	if err != nil {
		return nil, err
	}
	if int64(len(b)) > maxBytes {
		return nil, fmt.Errorf("response body exceeds %d bytes", maxBytes)
	}
	return b, nil
}

// jsonLDRe matches <script type="application/ld+json"> blocks.
var jsonLDRe = regexp.MustCompile(`(?is)<script\s+type=["']application/ld\+json["'][^>]*>(.*?)</script>`)

// ldJobPosting is a partial schema.org/JobPosting shape used for JSON-LD extraction.
type ldJobPosting struct {
	Context           string      `json:"@context"`
	Type              string      `json:"@type"`
	Title             string      `json:"title"`
	Description       string      `json:"description"`
	DatePosted        string      `json:"datePosted"`
	EmploymentType    string      `json:"employmentType"`
	HiringOrganization struct {
		Name string `json:"name"`
		URL  string `json:"url"`
	} `json:"hiringOrganization"`
	JobLocation struct {
		Type     string `json:"@type"`
		Address  struct {
			AddressLocality string `json:"addressLocality"`
			AddressRegion   string `json:"addressRegion"`
			AddressCountry  string `json:"addressCountry"`
		} `json:"address"`
	} `json:"jobLocation"`
	BaseSalary json.RawMessage `json:"baseSalary"`
	URL        string          `json:"url"`
	DirectApply string         `json:"directApply"`
}

// ExtractJobPostingsJSONLD extracts JobPosting entries from schema.org
// JSON-LD <script> blocks embedded in HTML.  Returns nil when none are found.
func ExtractJobPostingsJSONLD(body []byte) []model.JobPost {
	blocks := jsonLDRe.FindAllStringSubmatch(string(body), -1)
	if len(blocks) == 0 {
		return nil
	}

	var jobs []model.JobPost
	for _, m := range blocks {
		if len(m) < 2 {
			continue
		}
		var posting ldJobPosting
		if err := json.Unmarshal([]byte(m[1]), &posting); err != nil {
			continue
		}
		if posting.Type != "JobPosting" && posting.Type != "JobPostingShell" {
			continue
		}
		title := strings.TrimSpace(posting.Title)
		if title == "" {
			continue
		}
		company := strings.TrimSpace(posting.HiringOrganization.Name)
		jobURL := strings.TrimSpace(posting.URL)
		desc := strings.TrimSpace(posting.Description)

		var datePosted *time.Time
		if dp := strings.TrimSpace(posting.DatePosted); dp != "" {
			if t, err := time.Parse(time.RFC3339, dp); err == nil {
				datePosted = &t
			} else if t, err := time.Parse("2006-01-02", dp); err == nil {
				datePosted = &t
			}
		}

		jobType := ""
		if et := strings.TrimSpace(posting.EmploymentType); et != "" {
			jobType = normalizeLDEmploymentType(et)
		}

		location := model.Location{}
		if addr := posting.JobLocation.Address; addr.AddressLocality != "" {
			location.City = addr.AddressLocality
			location.State = addr.AddressRegion
			location.Country = addr.AddressCountry
		}

		job := model.JobPost{
			Title:       title,
			CompanyName: company,
			JobURL:      jobURL,
			Description: desc,
			DatePosted:  datePosted,
			JobType:     jobType,
			Location:    location,
			ApplyMethod: "external_url",
		}
		if jobURL != "" {
			job.ID = fmt.Sprintf("ld-%s", hashShort(jobURL))
		} else {
			job.ID = fmt.Sprintf("ld-%s-%d", hashShort(title+company), len(jobs))
		}
		jobs = append(jobs, job)
	}
	return jobs
}

// hashShort returns the first 12 characters of a FNV-64a hex string.
func hashShort(s string) string {
	h := fnv.New64a()
	h.Write([]byte(s))
	hex := fmt.Sprintf("%x", h.Sum64())
	if len(hex) > 12 {
		hex = hex[:12]
	}
	return hex
}

func normalizeLDEmploymentType(et string) string {
	switch strings.ToLower(strings.TrimSpace(et)) {
	case "full-time", "fulltime", "permanent":
		return "fulltime"
	case "part-time", "parttime":
		return "parttime"
	case "contract", "temporary", "contractor":
		return "contract"
	case "internship", "intern":
		return "internship"
	default:
		return et
	}
}

func HasMeaningfulJobs(jobs []model.JobPost) bool {
	if len(jobs) == 0 {
		return false
	}
	for _, j := range jobs {
		t := strings.ToLower(strings.TrimSpace(j.Title))
		u := strings.ToLower(strings.TrimSpace(j.JobURL))
		c := strings.TrimSpace(j.CompanyName)
		if t == "" || (u == "" && c == "") {
			continue
		}
		if strings.Contains(u, "/cdn-cgi/l/email-protection") {
			continue
		}
		if t == "latest jobs" || t == "work archive" || t == "archive" || t == "home" || t == "login" || t == "sign in" {
			continue
		}
		if strings.Contains(t, "email protected") || strings.Contains(t, "[email") {
			continue
		}
		return true
	}
	return false
}

// SleepWithContext sleeps for the given duration, returning early if ctx is cancelled.
func SleepWithContext(ctx context.Context, d time.Duration) error {
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-time.After(d):
		return nil
	}
}

// JitterSleep sleeps for a random duration between base and base+spread, returning
// early if ctx is cancelled. Useful for human-like delay patterns that are harder
// to fingerprint than fixed intervals.
func JitterSleep(ctx context.Context, base, spread time.Duration) error {
	if spread <= 0 {
		return SleepWithContext(ctx, base)
	}
	d := base + time.Duration(rand.Int63n(int64(spread)))
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-time.After(d):
		return nil
	}
}
