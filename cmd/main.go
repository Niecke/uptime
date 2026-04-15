package main

import (
	"database/sql"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"

	"niecke-it.de/uptime/internal/api"
	"niecke-it.de/uptime/internal/config"
	"niecke-it.de/uptime/internal/db"
	models "niecke-it.de/uptime/internal/models"
	"niecke-it.de/uptime/internal/sse"
)

func main() {
	fmt.Println("Starting the uptime checker...")

	configPtr := flag.String("config", "", "The path of the config file.")
	flag.Parse()
	cfg, err := config.LoadConfig(*configPtr)
	if err != nil {
		fmt.Println(err.Error())
		os.Exit(1)
	}

	broadcaster := sse.NewBroadcaster()
	go broadcaster.Run()

	database := db.SetupDatabase()
	go db.CompactDatabase(database)

	// setup http client
	httpClient := &http.Client{
		Timeout: time.Duration(cfg.Global.TimeoutSeconds) * time.Second,
		Transport: &http.Transport{
			IdleConnTimeout:     90 * time.Second,
			TLSHandshakeTimeout: 10 * time.Second,
		},
	}

	fmt.Println("Starting the api...")
	go api.SetupAPI(database, broadcaster)
	fmt.Println("Done")

	endpointIDs := map[string]int64{}

	for _, url := range cfg.Endpoints {
		id, err := db.InsertEndpoint(database, url)
		if err != nil {
			fmt.Printf("URL %v could not be stored in the db and will be skipped.", url)
		}
		endpointIDs[url] = id
	}

	ticker := time.NewTicker(time.Duration(cfg.Global.IntervalSeconds) * time.Second)
	defer ticker.Stop()

	runChecks(cfg, database, endpointIDs, broadcaster, httpClient)

	for range ticker.C {
		runChecks(cfg, database, endpointIDs, broadcaster, httpClient)
	}
}

var capturedHeaders = []string{
	"cf-cache-status",
	"cf-ray",
	"server-timing",
	"age",
	"server",
}

func extractHeaders(h http.Header) (map[string]string, error) {
	captured := make(map[string]string, len(capturedHeaders))
	if h == nil {
		return captured, nil
	}
	for _, name := range capturedHeaders {
		if v := h.Get(name); v != "" {
			captured[strings.ToLower(name)] = v
		}
	}
	if len(captured) == 0 {
		return captured, nil
	}

	return captured, nil
}

func runChecks(cfg models.Config, database *sql.DB, endpointIDs map[string]int64, broadcaster sse.Broadcaster, httpClient *http.Client) {
	fmt.Printf("##########################################################################\n\n")
	fmt.Printf("Check run: %v\n\n", time.Now().Format(time.TimeOnly))

	var wg sync.WaitGroup
	data := make(chan models.HealthResult)

	for _, url := range cfg.Endpoints {
		wg.Go(func() {
			runGet(url, data, httpClient)
		})
	}

	go func() {
		wg.Wait()
		close(data)
	}()

	for msg := range data {
		if err := db.InsertCheckResult(database, endpointIDs[msg.URL], msg); err != nil {
			fmt.Printf("Failed to store result for %v: %v\n", msg.URL, err)
		}

		errMsg := ""
		if msg.Err != nil {
			errMsg = msg.Err.Error()
		}
		event := models.SSEEvent{
			URL:        msg.URL,
			StatusCode: msg.StatusCode,
			DurationMs: msg.Duration.Milliseconds(),
			Error:      errMsg,
		}
		b, err := json.Marshal(event)
		if err != nil {
			fmt.Printf("Erro while json proccesing %v", err)
		}

		broadcaster.Broadcast <- string(b)

		if msg.Err != nil {
			fmt.Printf("%v\n X Error (%v)\n | Status: %v\n | Duration: %v\n", msg.URL, msg.Err, msg.StatusCode, msg.Duration)
		} else if msg.StatusCode >= 200 && msg.StatusCode <= 299 {
			fmt.Printf("%v\n | Status: %v\n | Duration: %v\n", msg.URL, msg.StatusCode, msg.Duration)
		} else {
			fmt.Printf("%v\n X Status: %v -> ERROR\n | Duration: %v\n", msg.URL, msg.StatusCode, msg.Duration)
		}
	}

}

func runGet(url string, data chan models.HealthResult, httpClient *http.Client) {
	start := time.Now()
	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		data <- models.HealthResult{URL: url, StatusCode: -1, Duration: time.Since(start), Err: err}
		return
	}

	resp, err := httpClient.Do(req)
	d := time.Since(start)

	headersJSON := map[string]string{}
	var hErr error
	if resp != nil {
		headersJSON, hErr = extractHeaders(resp.Header)
		if hErr != nil {
			fmt.Printf("header extraction failed for endpoint %v: %v", url, hErr)
		} else {
			fmt.Printf("header extracted for %v: %v", url, headersJSON)
		}
	}

	if err != nil {
		data <- models.HealthResult{URL: url, StatusCode: 0, Duration: d, Headers: headersJSON, Err: err}
		return
	}

	defer func() {
		io.Copy(io.Discard, resp.Body)
		resp.Body.Close()
	}()

	data <- models.HealthResult{URL: url, StatusCode: resp.StatusCode, Duration: d, Headers: headersJSON, Err: nil}
}
