package export

import (
	"strconv"
	"strings"
	"time"

	"github.com/arinbalyan/scrappy/internal/model"
)

func formatDate(t *time.Time) string {
	if t == nil || t.IsZero() {
		return ""
	}
	return t.UTC().Format(time.RFC3339)
}

func joinEmails(emails []model.Email) string {
	vals := make([]string, 0, len(emails))
	for _, e := range emails {
		if e.Addr != "" {
			vals = append(vals, e.Addr)
		}
	}
	return strings.Join(vals, ";")
}

func joinEmailVerified(emails []model.Email) string {
	vals := make([]string, 0, len(emails))
	for _, e := range emails {
		vals = append(vals, strconv.FormatBool(e.Verified))
	}
	return strings.Join(vals, ";")
}

func joinEmailSources(emails []model.Email) string {
	vals := make([]string, 0, len(emails))
	for _, e := range emails {
		if e.Source != "" {
			vals = append(vals, e.Source)
		}
	}
	return strings.Join(vals, ";")
}

func formatRating(r *float64) string {
	if r == nil {
		return ""
	}
	return strconv.FormatFloat(*r, 'f', -1, 64)
}

func formatCompInterval(c *model.Compensation) string {
	if c == nil {
		return ""
	}
	return string(c.Interval)
}

func formatCompMin(c *model.Compensation) string {
	if c == nil || c.MinAmount == nil {
		return ""
	}
	return strconv.FormatFloat(*c.MinAmount, 'f', 2, 64)
}

func formatCompMax(c *model.Compensation) string {
	if c == nil || c.MaxAmount == nil {
		return ""
	}
	return strconv.FormatFloat(*c.MaxAmount, 'f', 2, 64)
}

func formatCompCurrency(c *model.Compensation) string {
	if c == nil {
		return ""
	}
	return c.Currency
}
