package api

import (
	"database/sql"
	"embed"
	"encoding/json"
	"fmt"
	"io/fs"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/cors"
	"niecke-it.de/uptime/internal/db"
	"niecke-it.de/uptime/internal/models"
	"niecke-it.de/uptime/internal/sse"
)

//go:embed web
var webFiles embed.FS

type APIHandler struct {
	database    *sql.DB
	broadcaster sse.Broadcaster
}

func SetupAPI(database *sql.DB, broadcaster sse.Broadcaster) {
	h := APIHandler{database: database, broadcaster: broadcaster}
	r := chi.NewRouter()

	// TODO: CORS is disabled for now
	r.Use(cors.Handler(cors.Options{
		AllowedOrigins: []string{"*"},
		AllowedMethods: []string{"GET"},
	}))

	r.Mount("/endpoints", endpointRouter(&h))
	r.Mount("/events", eventRouter(&h))

	subFS, _ := fs.Sub(webFiles, "web")
	r.Get("/*", func(w http.ResponseWriter, r *http.Request) {
		http.FileServer(http.FS(subFS)).ServeHTTP(w, r)
	})

	err := http.ListenAndServe(":3333", r)
	if err != nil {
		fmt.Printf("There was an error starting the api: %v", err.Error())
	}
}

func endpointRouter(h *APIHandler) chi.Router {
	r := chi.NewRouter()

	r.Get("/", h.listEndpoints)

	r.Get("/{endpointId}/history", h.historyEndpoint)

	return r
}

func (h *APIHandler) listEndpoints(w http.ResponseWriter, r *http.Request) {
	endpoints, err := db.ListEndpoints(h.database)
	if err != nil {
		fmt.Printf("There was a db error: %v", err.Error())
		http.Error(w, "internal server error", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(endpoints)
}

func (h *APIHandler) historyEndpoint(w http.ResponseWriter, r *http.Request) {
	endpointId, err := strconv.Atoi(chi.URLParam(r, "endpointId"))
	if err != nil {
		fmt.Printf("There was a param parsing error: %v", err.Error())
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}

	endpointHistory, err := db.HistoryEndpoints(h.database, int64(endpointId))
	if err != nil {
		if err == models.ErrNotFound {
			http.Error(w, "endpoint id unkown", http.StatusNotFound)
			return
		}
		fmt.Printf("There was a db error: %v", err.Error())
		http.Error(w, "internal server error", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(endpointHistory)
}

func eventRouter(h *APIHandler) chi.Router {
	r := chi.NewRouter()

	r.Get("/", h.event)

	return r
}

func (h *APIHandler) event(w http.ResponseWriter, r *http.Request) {
	client := make(chan string)

	h.broadcaster.Register <- client

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")

	for {
		select {
		case event := <-client:
			// process event
			fmt.Fprintf(w, "data: %s\n\n", event)
			if f, ok := w.(http.Flusher); ok {
				f.Flush()
			}
		case <-r.Context().Done():
			h.broadcaster.Unregister <- client
			return
		}
	}
}
