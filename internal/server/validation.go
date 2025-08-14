package server

import (
	"fmt"
	"net/http"
	"net/url"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/sirupsen/logrus"
)

// ValidationError represents a validation error with details
type ValidationError struct {
	Field   string `json:"field"`
	Message string `json:"message"`
	Code    string `json:"code"`
}

// ValidationResult contains validation results
type ValidationResult struct {
	Valid  bool              `json:"valid"`
	Errors []ValidationError `json:"errors,omitempty"`
}

// respondWithValidationError sends a structured validation error response
func (ms *MusicServer) respondWithValidationError(w http.ResponseWriter, r *http.Request, errors []ValidationError) {
	ms.logger.WithFields(logrus.Fields{
		"method": r.Method,
		"path":   r.URL.Path,
		"errors": errors,
	}).Warn("Validation failed")

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusBadRequest)

	result := ValidationResult{
		Valid:  false,
		Errors: errors,
	}

	ms.respondJSON(w, result)
}

// respondWithError sends a structured error response
func (ms *MusicServer) respondWithError(w http.ResponseWriter, r *http.Request, statusCode int, message string, err error) {
	logEntry := ms.logger.WithFields(logrus.Fields{
		"method":      r.Method,
		"path":        r.URL.Path,
		"status_code": statusCode,
		"message":     message,
	})

	if err != nil {
		logEntry = logEntry.WithError(err)
	}

	if statusCode >= 500 {
		logEntry.Error("Server error")
	} else {
		logEntry.Warn("Client error")
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)

	response := map[string]interface{}{
		"error":   message,
		"code":    statusCode,
		"success": false,
	}

	ms.respondJSON(w, response)
}

// validateTrackID validates and parses a track ID from the URL path
func (ms *MusicServer) validateTrackID(pathParts []string, minParts int) (int, *ValidationError) {
	if len(pathParts) < minParts {
		return 0, &ValidationError{
			Field:   "track_id",
			Message: "Track ID is required",
			Code:    "MISSING_TRACK_ID",
		}
	}

	trackIDStr := pathParts[minParts-1]
	if trackIDStr == "" {
		return 0, &ValidationError{
			Field:   "track_id",
			Message: "Track ID cannot be empty",
			Code:    "EMPTY_TRACK_ID",
		}
	}

	trackID, err := strconv.Atoi(trackIDStr)
	if err != nil {
		return 0, &ValidationError{
			Field:   "track_id",
			Message: "Track ID must be a valid integer",
			Code:    "INVALID_TRACK_ID_FORMAT",
		}
	}

	if trackID <= 0 {
		return 0, &ValidationError{
			Field:   "track_id",
			Message: "Track ID must be positive",
			Code:    "INVALID_TRACK_ID_VALUE",
		}
	}

	return trackID, nil
}

// validatePlaylistID validates and parses a playlist ID from the URL path
func (ms *MusicServer) validatePlaylistID(pathParts []string, minParts int) (int, *ValidationError) {
	if len(pathParts) < minParts {
		return 0, &ValidationError{
			Field:   "playlist_id",
			Message: "Playlist ID is required",
			Code:    "MISSING_PLAYLIST_ID",
		}
	}

	playlistIDStr := pathParts[minParts-1]
	if playlistIDStr == "" {
		return 0, &ValidationError{
			Field:   "playlist_id",
			Message: "Playlist ID cannot be empty",
			Code:    "EMPTY_PLAYLIST_ID",
		}
	}

	playlistID, err := strconv.Atoi(playlistIDStr)
	if err != nil {
		return 0, &ValidationError{
			Field:   "playlist_id",
			Message: "Playlist ID must be a valid integer",
			Code:    "INVALID_PLAYLIST_ID_FORMAT",
		}
	}

	if playlistID <= 0 {
		return 0, &ValidationError{
			Field:   "playlist_id",
			Message: "Playlist ID must be positive",
			Code:    "INVALID_PLAYLIST_ID_VALUE",
		}
	}

	return playlistID, nil
}

// validateSearchQuery validates search query parameters
func (ms *MusicServer) validateSearchQuery(query string) *ValidationError {
	if len(query) > 1000 {
		return &ValidationError{
			Field:   "search",
			Message: "Search query too long (max 1000 characters)",
			Code:    "SEARCH_QUERY_TOO_LONG",
		}
	}

	// Check for potentially dangerous characters
	if strings.Contains(query, "\x00") {
		return &ValidationError{
			Field:   "search",
			Message: "Search query contains invalid characters",
			Code:    "INVALID_SEARCH_CHARACTERS",
		}
	}

	return nil
}

// validateFilePath ensures file path is within the configured music directory
func (ms *MusicServer) validateFilePath(filePath string) *ValidationError {
	// Clean and resolve the path
	cleanPath := filepath.Clean(filePath)
	absPath, err := filepath.Abs(cleanPath)
	if err != nil {
		return &ValidationError{
			Field:   "file_path",
			Message: "Invalid file path",
			Code:    "INVALID_FILE_PATH",
		}
	}

	// Get absolute music directory path
	absMusicDir, err := filepath.Abs(ms.config.Music.LibraryPath)
	if err != nil {
		return &ValidationError{
			Field:   "file_path",
			Message: "Server configuration error",
			Code:    "CONFIG_ERROR",
		}
	}

	// Check if file is within music directory
	relPath, err := filepath.Rel(absMusicDir, absPath)
	if err != nil || strings.HasPrefix(relPath, "..") {
		return &ValidationError{
			Field:   "file_path",
			Message: "File path outside allowed directory",
			Code:    "PATH_TRAVERSAL_DENIED",
		}
	}

	return nil
}

// validateURL validates download URLs
func (ms *MusicServer) validateURL(urlStr string) *ValidationError {
	if urlStr == "" {
		return &ValidationError{
			Field:   "url",
			Message: "URL is required",
			Code:    "MISSING_URL",
		}
	}

	if len(urlStr) > 2048 {
		return &ValidationError{
			Field:   "url",
			Message: "URL too long (max 2048 characters)",
			Code:    "URL_TOO_LONG",
		}
	}

	parsedURL, err := url.Parse(urlStr)
	if err != nil {
		return &ValidationError{
			Field:   "url",
			Message: "Invalid URL format",
			Code:    "INVALID_URL_FORMAT",
		}
	}

	if parsedURL.Scheme != "http" && parsedURL.Scheme != "https" {
		return &ValidationError{
			Field:   "url",
			Message: "URL must use HTTP or HTTPS protocol",
			Code:    "INVALID_URL_PROTOCOL",
		}
	}

	return nil
}

// validatePlaylistName validates playlist name
func (ms *MusicServer) validatePlaylistName(name string) *ValidationError {
	if name == "" {
		return &ValidationError{
			Field:   "name",
			Message: "Playlist name is required",
			Code:    "MISSING_PLAYLIST_NAME",
		}
	}

	if len(name) > 255 {
		return &ValidationError{
			Field:   "name",
			Message: "Playlist name too long (max 255 characters)",
			Code:    "PLAYLIST_NAME_TOO_LONG",
		}
	}

	// Check for dangerous characters
	if strings.Contains(name, "\x00") || strings.Contains(name, "\n") || strings.Contains(name, "\r") {
		return &ValidationError{
			Field:   "name",
			Message: "Playlist name contains invalid characters",
			Code:    "INVALID_PLAYLIST_NAME_CHARACTERS",
		}
	}

	return nil
}

// validatePlaylistDescription validates playlist description
func (ms *MusicServer) validatePlaylistDescription(description string) *ValidationError {
	if len(description) > 1000 {
		return &ValidationError{
			Field:   "description",
			Message: "Playlist description too long (max 1000 characters)",
			Code:    "PLAYLIST_DESCRIPTION_TOO_LONG",
		}
	}

	return nil
}

// validateContentType validates content types for streaming
func (ms *MusicServer) validateContentType(filePath string) *ValidationError {
	ext := strings.ToLower(filepath.Ext(filePath))

	if !ms.extractor.IsAudioFile(filePath) {
		return &ValidationError{
			Field:   "file_type",
			Message: fmt.Sprintf("Unsupported file type: %s", ext),
			Code:    "UNSUPPORTED_FILE_TYPE",
		}
	}

	return nil
}

// sanitizeInput sanitizes user input to prevent injection attacks
func sanitizeInput(input string) string {
	// Remove null bytes
	input = strings.ReplaceAll(input, "\x00", "")

	// Trim whitespace
	input = strings.TrimSpace(input)

	return input
}
