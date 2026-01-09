package health

import (
	"encoding/json"
	"net/http"
	"time"
)

// Status represents the health status response
type Status struct {
	Status    string            `json:"status"`
	Timestamp string            `json:"timestamp"`
	Checks    map[string]string `json:"checks,omitempty"`
}

// Handler handles health check endpoints
type Handler struct {
	opcuaReady bool
	startTime  time.Time
}

// NewHandler creates a new health handler
func NewHandler() *Handler {
	return &Handler{
		startTime: time.Now(),
	}
}

// SetOPCUAReady sets the OPC UA server readiness status
func (h *Handler) SetOPCUAReady(ready bool) {
	h.opcuaReady = ready
}

// HandleLive handles the liveness probe
// Returns 200 if the application is running
func (h *Handler) HandleLive(w http.ResponseWriter, r *http.Request) {
	status := Status{
		Status:    "alive",
		Timestamp: time.Now().UTC().Format(time.RFC3339),
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(status)
}

// HandleReady handles the readiness probe
// Returns 200 if the application is ready to serve traffic
func (h *Handler) HandleReady(w http.ResponseWriter, r *http.Request) {
	checks := make(map[string]string)
	allHealthy := true

	// Check OPC UA server
	if h.opcuaReady {
		checks["opcua_server"] = "healthy"
	} else {
		checks["opcua_server"] = "not_ready"
		allHealthy = false
	}

	// Check uptime (give 5 seconds for startup)
	uptime := time.Since(h.startTime)
	if uptime > 5*time.Second {
		checks["startup"] = "complete"
	} else {
		checks["startup"] = "in_progress"
		allHealthy = false
	}

	status := Status{
		Timestamp: time.Now().UTC().Format(time.RFC3339),
		Checks:    checks,
	}

	w.Header().Set("Content-Type", "application/json")

	if allHealthy {
		status.Status = "ready"
		w.WriteHeader(http.StatusOK)
	} else {
		status.Status = "not_ready"
		w.WriteHeader(http.StatusServiceUnavailable)
	}

	json.NewEncoder(w).Encode(status)
}

// HandleHealth handles the combined health endpoint (for Docker HEALTHCHECK)
func (h *Handler) HandleHealth(w http.ResponseWriter, r *http.Request) {
	h.HandleReady(w, r)
}
