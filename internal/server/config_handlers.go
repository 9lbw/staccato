package server

import (
	"encoding/json"
	"net/http"
)

// ConfigResponse represents the public configuration sent to the frontend
type ConfigResponse struct {
	Auth AuthConfigResponse `json:"auth"`
}

// AuthConfigResponse represents auth-related configuration for the frontend
type AuthConfigResponse struct {
	Enabled       bool  `json:"enabled"`
	UserFolders   bool  `json:"user_folders"`
	AllowUploads  bool  `json:"allow_uploads"`
	MaxUploadSize int64 `json:"max_upload_size_mb"`
}

// handleGetConfig returns public configuration settings for the frontend
func (ms *MusicServer) handleGetConfig(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		ms.respondWithError(w, r, http.StatusMethodNotAllowed, "Method not allowed", nil)
		return
	}

	config := ConfigResponse{
		Auth: AuthConfigResponse{
			Enabled:       ms.config.Auth.Enabled,
			UserFolders:   ms.config.Auth.UserFolders,
			AllowUploads:  ms.config.Auth.AllowUploads,
			MaxUploadSize: ms.config.Auth.MaxUploadSize,
		},
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(config)
}
