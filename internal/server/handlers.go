package server

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"staccato/pkg/models"
)

// handleHome serves the main SPA / index file from the configured static dir.
func (ms *MusicServer) handleHome(w http.ResponseWriter, r *http.Request) {
	http.ServeFile(w, r, filepath.Join(ms.config.Server.StaticDir, "index.html"))
}

// handleGetTracks returns tracks optionally filtered (search) or sorted.
func (ms *MusicServer) handleGetTracks(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	// Check parameters
	searchQuery := r.URL.Query().Get("search")
	sortBy := r.URL.Query().Get("sort")
	var tracks []models.Track
	var err error

	if searchQuery != "" {
		tracks, err = ms.db.SearchTracks(searchQuery)
	} else if sortBy == "album" {
		tracks, err = ms.db.GetTracksSortedByAlbum()
	} else {
		tracks, err = ms.db.GetAllTracks()
	}

	if err != nil {
		http.Error(w, "Error retrieving tracks", http.StatusInternalServerError)
		return
	}

	json.NewEncoder(w).Encode(tracks)
}

// handleGetTrackCount responds with a JSON count of all tracks.
func (ms *MusicServer) handleGetTrackCount(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	tracks, err := ms.db.GetAllTracks()
	if err != nil {
		http.Error(w, "Error retrieving track count", http.StatusInternalServerError)
		return
	}

	response := map[string]int{"count": len(tracks)}
	json.NewEncoder(w).Encode(response)
}

// handleStreamTrack streams an individual track by ID with Range support.
func (ms *MusicServer) handleStreamTrack(w http.ResponseWriter, r *http.Request) {
	// Extract track ID from URL path
	pathParts := strings.Split(r.URL.Path, "/")
	if len(pathParts) < 3 {
		http.Error(w, "Invalid track ID", http.StatusBadRequest)
		return
	}

	trackIDStr := pathParts[2]
	trackID, err := strconv.Atoi(trackIDStr)
	if err != nil {
		http.Error(w, "Invalid track ID", http.StatusBadRequest)
		return
	}

	// Get track from database
	track, err := ms.db.GetTrackByID(trackID)
	if err != nil {
		http.Error(w, "Track not found", http.StatusNotFound)
		return
	}

	// Open the audio file
	file, err := os.Open(track.FilePath)
	if err != nil {
		log.Printf("Error opening file %s: %v", track.FilePath, err)
		http.Error(w, "Error opening audio file", http.StatusInternalServerError)
		return
	}
	defer file.Close()

	// Get file info for content length
	stat, err := file.Stat()
	if err != nil {
		http.Error(w, "Error reading file info", http.StatusInternalServerError)
		return
	}

	// Set appropriate headers for audio streaming
	w.Header().Set("Content-Type", ms.extractor.GetContentType(track.FilePath))
	w.Header().Set("Content-Length", fmt.Sprintf("%d", stat.Size()))
	w.Header().Set("Accept-Ranges", "bytes")
	// CORS header applied by middleware if enabled

	// Handle range requests for seeking support
	rangeHeader := r.Header.Get("Range")
	if rangeHeader != "" {
		ms.handleRangeRequest(w, r, file, stat.Size(), rangeHeader)
		return
	}

	// Stream the entire file
	log.Printf("Streaming track: %s - %s", track.Artist, track.Title)
	_, err = io.Copy(w, file)
	if err != nil {
		log.Printf("Error streaming file: %v", err)
	}
}

// handleRangeRequest implements simple single-range byte serving for seeking.
func (ms *MusicServer) handleRangeRequest(w http.ResponseWriter, _ *http.Request, file *os.File, fileSize int64, rangeHeader string) {
	// Parse range header (e.g., "bytes=0-1023")
	ranges := strings.TrimPrefix(rangeHeader, "bytes=")
	rangeParts := strings.Split(ranges, "-")

	start, err := strconv.ParseInt(rangeParts[0], 10, 64)
	if err != nil {
		start = 0
	}

	var end int64
	if len(rangeParts) > 1 && rangeParts[1] != "" {
		end, err = strconv.ParseInt(rangeParts[1], 10, 64)
		if err != nil {
			end = fileSize - 1
		}
	} else {
		end = fileSize - 1
	}

	// Validate range
	if start < 0 || end >= fileSize || start > end {
		w.Header().Set("Content-Range", fmt.Sprintf("bytes */%d", fileSize))
		http.Error(w, "Range Not Satisfiable", http.StatusRequestedRangeNotSatisfiable)
		return
	}

	// Set partial content headers
	contentLength := end - start + 1
	w.Header().Set("Content-Range", fmt.Sprintf("bytes %d-%d/%d", start, end, fileSize))
	w.Header().Set("Content-Length", fmt.Sprintf("%d", contentLength))
	w.WriteHeader(http.StatusPartialContent)

	// Seek to start position and copy the requested range
	file.Seek(start, io.SeekStart)
	io.CopyN(w, file, contentLength)
}
