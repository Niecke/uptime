package api

import (
	"database/sql"
	"embed"
	"encoding/json"
	"io/fs"
	"log/slog"
	"net/http"
	"os"
	"strconv"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
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
	config      models.Config
}

func SetupAPI(database *sql.DB, broadcaster sse.Broadcaster, config models.Config) {
	h := APIHandler{database: database, broadcaster: broadcaster, config: config}
	r := chi.NewRouter()

	// TODO: CORS is disabled for now
	r.Use(cors.Handler(cors.Options{
		AllowedOrigins: []string{"*"},
		AllowedMethods: []string{"GET"},
	}))
	r.Use(slogLogger)

	r.Mount("/endpoints", endpointRouter(&h))
	r.Mount("/events", eventRouter(&h))

	// TODO: move to config setup
	var devMode = os.Getenv("DEV") == "true"

	var fileSystem http.FileSystem
	if devMode {
		fileSystem = http.Dir("internal/api/web")
	} else {
		subFS, _ := fs.Sub(webFiles, "web")
		fileSystem = http.FS(subFS)
	}

	r.Get("/*", func(w http.ResponseWriter, r *http.Request) {
		http.FileServer(fileSystem).ServeHTTP(w, r)
	})

	slog.Info("API started on http://0.0.0.0:3333")
	err := http.ListenAndServe(":3333", r)
	if err != nil {
		slog.Error("There was an error starting the api", "error", err.Error())
	}
}

func endpointRouter(h *APIHandler) chi.Router {
	r := chi.NewRouter()

	r.Get("/", h.listEndpoints)

	r.Get("/{endpointId}/history", h.historyEndpoint)

	return r
}

func (h *APIHandler) listEndpoints(w http.ResponseWriter, r *http.Request) {
	endpoints, err := db.ListEndpoints(h.database, h.config.Global.RetentionDays)
	if err != nil {
		slog.Error("There was a db error", "error", err.Error())
		http.Error(w, "internal server error", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(endpoints)
}

func (h *APIHandler) historyEndpoint(w http.ResponseWriter, r *http.Request) {
	endpointId, err := strconv.Atoi(chi.URLParam(r, "endpointId"))
	if err != nil {
		slog.Error("There was a param parsing error", "error", err.Error())
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}

	endpointHistory, err := db.HistoryEndpoints(h.database, int64(endpointId))
	if err != nil {
		if err == models.ErrNotFound {
			http.Error(w, "endpoint id unkown", http.StatusNotFound)
			return
		}
		slog.Error("There was a db error", "error", err.Error())
		http.Error(w, "internal server error", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(endpointHistory)
}

func slogLogger(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ww := middleware.NewWrapResponseWriter(w, r.ProtoMajor)
		start := time.Now()
		next.ServeHTTP(ww, r)
		slog.Info("request",
			"method", r.Method,
			"path", r.URL.Path,
			"status", ww.Status(),
			"duration_ms", time.Since(start).Milliseconds(),
		)
	})
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
			slog.Debug("sending data via sse", "event", event)
			if f, ok := w.(http.Flusher); ok {
				f.Flush()
			}
		case <-r.Context().Done():
			h.broadcaster.Unregister <- client
			return
		}
	}
}
