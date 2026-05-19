package model

// SalaryNormalized is a convenience type for always-annual, always-USD compensation.
type SalaryNormalized struct {
	AnnualMin  float64 `json:"annual_min,omitempty"`
	AnnualMax  float64 `json:"annual_max,omitempty"`
	Currency   string  `json:"currency,omitempty"` // always "USD" after conversion
}

// MarshalJSON implements custom JSON serialization for JobPost
// to produce flat rows compatible with scrappy's output column order.
// TODO: wire into export/ when the Writer interface is built.
