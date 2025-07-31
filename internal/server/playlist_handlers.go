package server

import (
	"encoding/json"
	"net/http"
	"strconv"
	"strings"
)

// handleGetPlaylists handles the GET request for playlists
func (ms *MusicServer) handleGetPlaylists(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	ms.setCORSHeaders(w)

	playlists, err := ms.db.GetAllPlaylists()
	if err != nil {
		http.Error(w, "Error retrieving playlists", http.StatusInternalServerError)
		return
	}

	json.NewEncoder(w).Encode(playlists)
}

// handleCreatePlaylist handles playlist creation
func (ms *MusicServer) handleCreatePlaylist(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	ms.setCORSHeaders(w)

	var req struct {
		Name        string `json:"name"`
		Description string `json:"description"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid JSON", http.StatusBadRequest)
		return
	}

	if req.Name == "" {
		http.Error(w, "Playlist name is required", http.StatusBadRequest)
		return
	}

	id, err := ms.db.CreatePlaylist(req.Name, req.Description)
	if err != nil {
		http.Error(w, "Error creating playlist", http.StatusInternalServerError)
		return
	}

	response := map[string]interface{}{
		"id":      id,
		"message": "Playlist created successfully",
	}
	json.NewEncoder(w).Encode(response)
}

// handleGetPlaylistTracks handles getting tracks from a playlist
func (ms *MusicServer) handleGetPlaylistTracks(w http.ResponseWriter, r *http.Request) {
	pathParts := strings.Split(r.URL.Path, "/")
	if len(pathParts) < 4 {
		http.Error(w, "Invalid playlist ID", http.StatusBadRequest)
		return
	}

	playlistID, err := strconv.Atoi(pathParts[3])
	if err != nil {
		http.Error(w, "Invalid playlist ID", http.StatusBadRequest)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	ms.setCORSHeaders(w)

	tracks, err := ms.db.GetPlaylistTracks(playlistID)
	if err != nil {
		http.Error(w, "Error retrieving playlist tracks", http.StatusInternalServerError)
		return
	}

	json.NewEncoder(w).Encode(tracks)
}

// handleAddTrackToPlaylist handles adding a track to a playlist
func (ms *MusicServer) handleAddTrackToPlaylist(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	pathParts := strings.Split(r.URL.Path, "/")
	if len(pathParts) < 4 {
		http.Error(w, "Invalid playlist ID", http.StatusBadRequest)
		return
	}

	playlistID, err := strconv.Atoi(pathParts[3])
	if err != nil {
		http.Error(w, "Invalid playlist ID", http.StatusBadRequest)
		return
	}

	var req struct {
		TrackID int `json:"trackId"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid JSON", http.StatusBadRequest)
		return
	}

	err = ms.db.AddTrackToPlaylist(playlistID, req.TrackID)
	if err != nil {
		http.Error(w, "Error adding track to playlist", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	ms.setCORSHeaders(w)
	json.NewEncoder(w).Encode(map[string]string{"message": "Track added to playlist"})
}

// handleRemoveTrackFromPlaylist handles removing a track from a playlist
func (ms *MusicServer) handleRemoveTrackFromPlaylist(w http.ResponseWriter, r *http.Request) {
	if r.Method != "DELETE" {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	pathParts := strings.Split(r.URL.Path, "/")
	if len(pathParts) < 6 {
		http.Error(w, "Invalid playlist or track ID", http.StatusBadRequest)
		return
	}

	playlistID, err := strconv.Atoi(pathParts[3])
	if err != nil {
		http.Error(w, "Invalid playlist ID", http.StatusBadRequest)
		return
	}

	trackID, err := strconv.Atoi(pathParts[5])
	if err != nil {
		http.Error(w, "Invalid track ID", http.StatusBadRequest)
		return
	}

	err = ms.db.RemoveTrackFromPlaylist(playlistID, trackID)
	if err != nil {
		http.Error(w, "Error removing track from playlist", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	ms.setCORSHeaders(w)
	json.NewEncoder(w).Encode(map[string]string{"message": "Track removed from playlist"})
}

// handleDeletePlaylist handles deleting a playlist
func (ms *MusicServer) handleDeletePlaylist(w http.ResponseWriter, r *http.Request) {
	if r.Method != "DELETE" {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	pathParts := strings.Split(r.URL.Path, "/")
	if len(pathParts) < 4 {
		http.Error(w, "Invalid playlist ID", http.StatusBadRequest)
		return
	}

	playlistID, err := strconv.Atoi(pathParts[3])
	if err != nil {
		http.Error(w, "Invalid playlist ID", http.StatusBadRequest)
		return
	}

	err = ms.db.DeletePlaylist(playlistID)
	if err != nil {
		http.Error(w, "Error deleting playlist", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	ms.setCORSHeaders(w)
	json.NewEncoder(w).Encode(map[string]string{"message": "Playlist deleted"})
}
