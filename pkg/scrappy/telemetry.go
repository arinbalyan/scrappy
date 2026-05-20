package scrappy

import "github.com/arinbalyan/scrappy/internal/model"

type SiteTelemetry struct {
	Site            model.Site  `json:"site"`
	Attempted       bool        `json:"attempted"`
	Success         bool        `json:"success"`
	Error           string      `json:"error,omitempty"`
	ResultCount     int         `json:"result_count"`
	EmptyPageRate   float64     `json:"empty_page_rate"`
	CaptchaRate     float64     `json:"captcha_rate"`
	CursorStalls    int         `json:"cursor_stalls"`
	StatusCodeCount map[int]int `json:"status_code_counts,omitempty"`
}

type RunTelemetry struct {
	Sites            []SiteTelemetry    `json:"sites"`
	SuggestedSiteRPS map[model.Site]int `json:"suggested_site_rps,omitempty"`
}
