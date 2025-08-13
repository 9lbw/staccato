package server

import (
	"encoding/json"
	"net/http"
	"time"
)

// HealthStatus represents operational status for the /health endpoint.
type HealthStatus struct {
	Status    string                 `json:"status"`
	Timestamp time.Time              `json:"timestamp"`
	Database  string                 `json:"database"`
	Storage   string                 `json:"storage"`
	Sessions  int                    `json:"activeSessions"`
	Tracks    int                    `json:"trackCount"`
	Details   map[string]interface{} `json:"details,omitempty"`
}

// handleHealthCheck returns basic liveness + dependency checks.
func (ms *MusicServer) handleHealthCheck(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	health := &HealthStatus{
		Status:    "healthy",
		Timestamp: time.Now(),
		Database:  "ok",
		Storage:   "ok",
		Details:   make(map[string]interface{}),
	}

	// Check database connectivity
	if err := ms.checkDatabaseHealth(); err != nil {
		health.Status = "unhealthy"
		health.Database = "error"
		health.Details["database_error"] = err.Error()
	}

	// Check storage accessibility
	if err := ms.checkStorageHealth(); err != nil {
		health.Status = "unhealthy"
		health.Storage = "error"
		health.Details["storage_error"] = err.Error()
	}

	// Get track count
	tracks, err := ms.db.GetAllTracks()
	if err != nil {
		health.Details["track_count_error"] = err.Error()
	} else {
		health.Tracks = len(tracks)
	}

	// Set appropriate HTTP status code
	if health.Status == "unhealthy" {
		w.WriteHeader(http.StatusServiceUnavailable)
	}

	json.NewEncoder(w).Encode(health)
}

// checkDatabaseHealth performs a trivial query to validate DB access.
func (ms *MusicServer) checkDatabaseHealth() error {
	// Try a simple query to check database connectivity
	_, err := ms.db.GetAllTracks()
	return err
}

// checkStorageHealth validates storage accessibility (placeholder for future enhancements).
func (ms *MusicServer) checkStorageHealth() error {
	// Check if music library path exists and is accessible
	_, err := ms.db.GetAllTracks()
	if err != nil {
		return err
	}
	// Could add more storage checks here
	return nil
}
