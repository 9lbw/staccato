package server

import (
	"net/http"
	"strings"
)

// handleAlbumArt serves album art images
func (ms *MusicServer) handleAlbumArt(w http.ResponseWriter, r *http.Request) {
	// Extract album art ID from URL path
	pathParts := strings.Split(r.URL.Path, "/")
	if len(pathParts) < 3 {
		http.Error(w, "Invalid album art ID", http.StatusBadRequest)
		return
	}

	artID := pathParts[2]
	if artID == "" {
		http.Error(w, "Invalid album art ID", http.StatusBadRequest)
		return
	}

	// Get album art from metadata extractor cache
	artData, exists := ms.extractor.GetAlbumArt(artID)
	if !exists {
		http.Error(w, "Album art not found", http.StatusNotFound)
		return
	}

	// Set appropriate content type
	contentType := ms.extractor.GetAlbumArtMimeType(artData)
	w.Header().Set("Content-Type", contentType)
	w.Header().Set("Cache-Control", "public, max-age=3600") // Cache for 1 hour

	// Serve the image data
	w.Write(artData)
}
