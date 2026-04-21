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

type EndpointHistoryEntry struct {
	StatusCode int       `json:"status_code"`
	CheckedAt  time.Time `json:"checked_at"`
	Duration   int64     `json:"duration_ms"`
}

type EndpointHistory struct {
	ID      int64                  `json:"id"`
	URL     string                 `json:"url"`
	History []EndpointHistoryEntry `json:"history"`
}
