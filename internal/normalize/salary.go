package normalize

import "github.com/arinbalyan/scrappy/internal/model"

// AnnualizeCompensation normalizes compensation intervals to yearly equivalents.
// It preserves currency and scales min/max amounts based on interval multipliers.
//
// yearly: 1x
// monthly: 12x
// weekly: 52x
// daily: 260x (5 days/week * 52 weeks)
// hourly: 2080x (40 hours/week * 52 weeks)
func AnnualizeCompensation(c *model.Compensation) *model.Compensation {
	if c == nil {
		return nil
	}
	multiplier := 1.0
	switch c.Interval {
	case model.IntervalYearly, "":
		multiplier = 1
	case model.IntervalMonthly:
		multiplier = 12
	case model.IntervalWeekly:
		multiplier = 52
	case model.IntervalDaily:
		multiplier = 260
	case model.IntervalHourly:
		multiplier = 2080
	default:
		multiplier = 1
	}

	out := &model.Compensation{
		Interval: model.IntervalYearly,
		Currency: c.Currency,
	}
	if out.Currency == "" {
		out.Currency = "USD"
	}
	if c.MinAmount != nil {
		v := *c.MinAmount * multiplier
		out.MinAmount = &v
	}
	if c.MaxAmount != nil {
		v := *c.MaxAmount * multiplier
		out.MaxAmount = &v
	}
	return out
}
