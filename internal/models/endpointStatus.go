package models

import (
	"errors"
	"time"
)

var ErrNotFound = errors.New("not found")

type EndpointStatus struct {
	ID               int64     `json:"id"`
	URL              string    `json:"url"`
	StatusCode       int       `json:"status_code"`
	CheckedAt        time.Time `json:"checked_at"`
	Duration         int64     `json:"duration_ms"`
	UptimePercentage float32   `json:"uptime"`
}

type EndpointHistoryBucket struct {
	Hour        string `json:"hour"` // "2026-08-17T17"
	Total       int    `json:"total"`
	Failures    int    `json:"failures"`
	AvgDuration int64  `json:"avg_duration_ms"`
}

type EndpointHistory struct {
	ID      int64                   `json:"id"`
	URL     string                  `json:"url"`
	History []EndpointHistoryBucket `json:"history"`
}
