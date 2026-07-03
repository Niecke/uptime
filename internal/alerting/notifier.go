package alerting

import (
	"encoding/json"
	"log/slog"
	"math"

	"niecke-it.de/uptime/internal/models"
	"niecke-it.de/uptime/internal/sse"
)

// thresholds are stored in a map where the url is the key
// value an object to keep track of failed requests and if the alert already triggered
// once there is a success the count should be set to zero and fired to false
type NotifyHandler struct {
	broadcaster  sse.Broadcaster
	alertStorage map[string]*alertObject
}

type alertObject struct {
	count uint8
	fired bool
}

// maybe it would make more sense to set threshold per url and not alert
func SetupNotifier(broadcaster sse.Broadcaster, cfg models.Config) {
	h := NotifyHandler{
		broadcaster:  broadcaster,
		alertStorage: make(map[string]*alertObject),
	}

	client := make(chan string)
	h.broadcaster.Register <- client

	for e := range client {
		var event models.SSEEvent
		if err := json.Unmarshal([]byte(e), &event); err != nil {
			slog.Error("failed to unmarshal event", "error", err)
			continue
		}

		if h.alertStorage[event.URL] == nil {
			h.alertStorage[event.URL] = &alertObject{}
		}

		if event.Error != "" {
			// increase count
			slog.Debug("error event received from broadcaster")
			if h.alertStorage[event.URL].count < math.MaxUint8 {
				// only increase until we haven't reached max to prevent overflow
				h.alertStorage[event.URL].count++
			}

			for _, a := range cfg.Alertings {
				if h.alertStorage[event.URL].count >= a.Threshold && !h.alertStorage[event.URL].fired {
					if a.Type == "slack" {
						Notify(a.Address, event.URL, event.StatusCode, event.Error, a.Threshold)
						h.alertStorage[event.URL].fired = true
					}
				}
			}
		} else {
			// reset count to zero
			if h.alertStorage[event.URL].count != 0 {
				slog.Debug("resetting threshold")
				h.alertStorage[event.URL].count = 0
				h.alertStorage[event.URL].fired = false
			}
		}
	}
}
