package models

type SSEEvent struct {
	URL        string `json:"url"`
	StatusCode int    `json:"status_code"`
	DurationMs int64  `json:"duration_ms"`
	Error      string `json:"error,omitempty"`
}
