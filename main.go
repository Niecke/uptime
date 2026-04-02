package main

import (
	"fmt"
	"net/http"
	"time"
	"sync"
)

type healthResult struct {
	url string
	statusCode int
	duration time.Duration
	err error
}

func main() {
	fmt.Println("Starting the uptime checker...")

	var s []string
	s = make([]string, 4)

	s[0] = "https://niecke-it.de"
	s[1] = "https://google.com"
	s[2] = "https://i-dont-exists.local"
	s[3] = "https://lumios-app.niecke-it.de/test"

	var wg sync.WaitGroup
	data := make(chan healthResult)

	for _, v := range s {
		wg.Go(func() {
			runGet(v, data)
		})	
	}

	go func() {
    	wg.Wait()
		close(data)
	}()

	for msg := range data{
		if msg.err != nil {
			fmt.Printf("%v\n X Error (%v)\n", msg.url, msg.err)
		} else if msg.statusCode >= 200 && msg.statusCode <= 299 {
			fmt.Printf("%v\n | Status: %v\n | Duration: %v\n", msg.url, msg.statusCode, msg.duration)
		} else {
			fmt.Printf("%v\n X Status: %v -> ERROR\n | Duration: %v\n", msg.url, msg.statusCode, msg.duration)
		}
	}
	
}

func runGet(url string, data chan healthResult) {
	start := time.Now()
	resp, err := http.Get(url)
	d := time.Since(start)

	// handle total failure of request; no conneection could be established
	if err != nil {
		t := healthResult{url: url, statusCode: 0, duration: d, err: err}
		data <- t
		return
	}

	defer resp.Body.Close()

	t := healthResult{url: url, statusCode: resp.StatusCode, duration: d, err: nil}
	data <- t
}