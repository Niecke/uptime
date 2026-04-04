package api

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"
	"niecke-it.de/uptime/internal/db"
	"niecke-it.de/uptime/internal/models"
)

type APIHandler struct {
	database *sql.DB
}

func SetupAPI(database *sql.DB) {
	h := APIHandler{database: database}
	r := chi.NewRouter()

	r.Get("/", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("Uptime checker"))
	})

	r.Mount("/endpoints", endpointRouter(&h))

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
