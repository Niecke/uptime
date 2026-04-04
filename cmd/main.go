package main

import (
	"context"
	"database/sql"
	"fmt"
	"net/http"
	"sync"
	"time"

	"niecke-it.de/uptime/internal/api"
	"niecke-it.de/uptime/internal/db"
	models "niecke-it.de/uptime/internal/models"
)

func main() {
	fmt.Println("Starting the uptime checker...")

	database := db.SetupDatabase()

	fmt.Println("Starting the api...")
	go api.SetupAPI(database)
	fmt.Println("Done")

	var s []string
	s = make([]string, 4)

	s[0] = "https://niecke-it.de"
	s[1] = "https://google.com"
	s[2] = "https://i-dont-exists.local"
	s[3] = "https://lumios-app.niecke-it.de/test"

	endpointIDs := map[string]int64{}

	for _, url := range s {
		id, err := db.InsertEndpoint(database, url)
		if err != nil {
			fmt.Printf("URL %v could not be stored in the db and will be skipped.", url)
		}
		endpointIDs[url] = id
	}

	ticker := time.NewTicker(10 * time.Second)
	defer ticker.Stop()

	runChecks(s, database, endpointIDs)

	for range ticker.C {
		runChecks(s, database, endpointIDs)
	}
}

func runChecks(urls []string, database *sql.DB, endpointIDs map[string]int64) {
	fmt.Printf("##########################################################################\n\n")
	fmt.Printf("Check run: %v\n\n", time.Now().Format(time.TimeOnly))

	var wg sync.WaitGroup
	data := make(chan models.HealthResult)

	for _, url := range urls {
		wg.Go(func() {
			runGet(url, data)
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

		if msg.Err != nil {
			fmt.Printf("%v\n X Error (%v)\n | Status: %v\n | Duration: %v\n", msg.URL, msg.Err, msg.StatusCode, msg.Duration)
		} else if msg.StatusCode >= 200 && msg.StatusCode <= 299 {
			fmt.Printf("%v\n | Status: %v\n | Duration: %v\n", msg.URL, msg.StatusCode, msg.Duration)
		} else {
			fmt.Printf("%v\n X Status: %v -> ERROR\n | Duration: %v\n", msg.URL, msg.StatusCode, msg.Duration)
		}
	}

}

func runGet(url string, data chan models.HealthResult) {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	start := time.Now()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		t := models.HealthResult{URL: url, StatusCode: -1, Duration: time.Since(start), Err: err}
		data <- t
		return
	}
	client := http.DefaultClient
	resp, err := client.Do(req)

	d := time.Since(start)

	// handle total failure of request; no conneection could be established
	if err != nil {
		t := models.HealthResult{URL: url, StatusCode: 0, Duration: d, Err: err}
		data <- t
		return
	}

	defer resp.Body.Close()

	t := models.HealthResult{URL: url, StatusCode: resp.StatusCode, Duration: d, Err: nil}
	data <- t
}
