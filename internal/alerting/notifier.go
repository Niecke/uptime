package alerting

import (
	"encoding/json"
	"log/slog"

	"niecke-it.de/uptime/internal/models"
	"niecke-it.de/uptime/internal/sse"
)

type NotifyHandler struct {
	broadcaster sse.Broadcaster
}

func SetupNotifier(broadcaster sse.Broadcaster, cfg models.Config) {
	// TODO: add internal memory containing fail counts and only notify when threshold reached
	h := NotifyHandler{broadcaster: broadcaster}

	client := make(chan string)
	h.broadcaster.Register <- client

	for e := range client {
		var event models.SSEEvent
		json.Unmarshal([]byte(e), &event)
		if event.Error != "" {
			slog.Debug("error event received from broadcaster")
			for _, a := range cfg.Alertings {
				if a.Type == "slack" {
					Notify(a.Address, event.URL, event.StatusCode, event.Error)
				}
			}
		}
	}
}
