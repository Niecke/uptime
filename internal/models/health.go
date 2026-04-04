package models

import "time"

type HealthResult struct {
	URL        string
	StatusCode int
	Duration   time.Duration
	Err        error
}
