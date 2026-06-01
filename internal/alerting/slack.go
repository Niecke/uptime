package alerting

import (
	"fmt"
	"log/slog"
	"net/http"
	"strings"
)

func Notify(slack_api string, url string, url_status_code int, url_error string) {
	slog.Debug("Notify Slack channel", "slack_api", slack_api)
	//TODO: use json.Marshal here; otherwise we can not add url_error here
	body := fmt.Sprintf(`{"text":"%s is down: %d"}`, url, url_status_code)
	resp, err := http.Post(slack_api, "application/json", strings.NewReader((body)))
	if err != nil {
		slog.Error("There was an error while sending slack notification", "error", err)
	}
	slog.Debug("reponse from slack", "status_code", resp.StatusCode, "resp", resp.Body)
	slog.Info("Sent notify to slack", "url", url)

	defer resp.Body.Close()
}
