package models

import "time"

type HealthResult struct {
	URL        string        `json:"url"`
	StatusCode int           `json:"status_code"`
	Duration   time.Duration `json:"duration_ms"`
	Err        error         `json:"err,omitempty"`
}
