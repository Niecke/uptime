package main

import (
	"fmt"
	"net/http"
	"time"
)

func main() {
	fmt.Println("Starting the uptime checker...")

	var s []string
	s = make([]string, 4)

	s[0] = "https://niecke-it.de"
	s[1] = "https://google.com"
	s[2] = "https://i-dont-exists.local"
	s[3] = "https://lumios-app.niecke-it.de/test"

	for _, v := range s {
		runGet(v)	
	}
}

func runGet(url string) int {
	start := time.Now()
	resp, err := http.Get(url)

	// handle total failure of request; no conneection could be established
	if err != nil {
		fmt.Printf("%v\n X Error (%v)\n", url, err)
		return 1
	}

	defer resp.Body.Close()

	d := time.Since(start)
	if resp.StatusCode >= 200 && resp.StatusCode <= 299 {
		fmt.Printf("%v\n | Status: %v\n | Duration: %v\n", url, resp.StatusCode, d)
	} else {
		fmt.Printf("%v\n X Status: %v -> ERROR\n | Duration: %v\n", url, resp.StatusCode, d)
	}
	

	return 0
}