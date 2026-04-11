package api

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"

	"github.com/go-chi/chi/v5"
	"niecke-it.de/uptime/internal/db"
	"niecke-it.de/uptime/internal/models"
	"niecke-it.de/uptime/internal/sse"
)

func TestListEndpoint(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/endpoints", nil)
	w := httptest.NewRecorder()

	h := newTestHandler()
	h.listEndpoints(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}
}

func TestHistoryEndpoint(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/endpoints/999/history", nil)
	w := httptest.NewRecorder()

	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("endpointId", "999")
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))

	h := newTestHandler()
	h.historyEndpoint(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("expected 404, got %d", w.Code)
	}
}

func TestHistoryEndpointExists(t *testing.T) {
	h := newTestHandler()
	url := "https://test.local"

	endpointId, err := db.InsertEndpoint(h.database, url)
	if err != nil {
		t.Errorf("error while inserting endpoint")
		return
	}

	hr := models.HealthResult{URL: url, StatusCode: 200, Duration: 2, Err: nil}
	if err := db.InsertCheckResult(h.database, endpointId, hr); err != nil {
		t.Errorf("error while inserting HealthResult")
	}

	// check that the endpoint returns a history object
	req := httptest.NewRequest(http.MethodGet, "/endpoints/999/history", nil)
	w := httptest.NewRecorder()

	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("endpointId", strconv.Itoa(int(endpointId)))
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))

	h.historyEndpoint(w, req)
	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}

	response := models.EndpointHistory{}
	s := fmt.Sprintf("%v", w.Body)
	if err := json.Unmarshal([]byte(s), &response); err != nil {
		t.Errorf("error parsing result")
	}

	if len(response.History) != 1 {
		t.Errorf("expected 1 history entry, got %d", len(response.History))
	}
}

func newTestHandler() APIHandler {
	database := db.SetupDatabaseWithPath(":memory:")
	b := sse.NewBroadcaster()
	return APIHandler{database: database, broadcaster: b}
}
