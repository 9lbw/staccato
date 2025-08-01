package server

import (
	"encoding/json"
	"log"
	"net/http"
	"strconv"
	"strings"
)

// handleGetPlayerState returns the current player state
func (ms *MusicServer) handleGetPlayerState(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	ms.setCORSHeaders(w)

	state := ms.playerState.GetState()
	json.NewEncoder(w).Encode(state)
}

// handleUpdatePlayerState updates the player state from client
func (ms *MusicServer) handleUpdatePlayerState(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	ms.setCORSHeaders(w)

	var req struct {
		TrackID       *int     `json:"trackId,omitempty"`
		IsPlaying     *bool    `json:"isPlaying,omitempty"`
		CurrentTime   *int     `json:"currentTime,omitempty"`
		TotalDuration *int     `json:"totalDuration,omitempty"`
		Volume        *float64 `json:"volume,omitempty"`
		IsMuted       *bool    `json:"isMuted,omitempty"`
		IsShuffled    *bool    `json:"isShuffled,omitempty"`
		RepeatMode    *int     `json:"repeatMode,omitempty"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		log.Printf("Error decoding player state JSON: %v", err)
		http.Error(w, "Invalid JSON", http.StatusBadRequest)
		return
	}

	log.Printf("Received player state update: %+v", req)

	// Update track if provided
	if req.TrackID != nil {
		if *req.TrackID == 0 {
			// Clear track
			ms.playerState.ClearTrack()
		} else {
			// Get track from database
			track, err := ms.db.GetTrackByID(*req.TrackID)
			if err != nil {
				http.Error(w, "Track not found", http.StatusNotFound)
				return
			}
			ms.playerState.UpdateTrack(track)
		}
	}

	// Update playback state
	if req.IsPlaying != nil {
		ms.playerState.UpdatePlaybackState(*req.IsPlaying)
	}

	// Update time
	if req.CurrentTime != nil || req.TotalDuration != nil {
		currentState := ms.playerState.GetState()
		currentTime := currentState.CurrentTime
		totalDuration := currentState.TotalDuration

		if req.CurrentTime != nil {
			currentTime = *req.CurrentTime
		}
		if req.TotalDuration != nil {
			totalDuration = *req.TotalDuration
		}

		ms.playerState.UpdateTime(currentTime, totalDuration)
	}

	// Update volume
	if req.Volume != nil || req.IsMuted != nil {
		currentState := ms.playerState.GetState()
		volume := currentState.Volume
		isMuted := currentState.IsMuted

		if req.Volume != nil {
			volume = *req.Volume
		}
		if req.IsMuted != nil {
			isMuted = *req.IsMuted
		}

		ms.playerState.UpdateVolume(volume, isMuted)
	}

	// Update settings
	if req.IsShuffled != nil || req.RepeatMode != nil {
		currentState := ms.playerState.GetState()
		isShuffled := currentState.IsShuffled
		repeatMode := currentState.RepeatMode

		if req.IsShuffled != nil {
			isShuffled = *req.IsShuffled
		}
		if req.RepeatMode != nil {
			repeatMode = *req.RepeatMode
		}

		ms.playerState.UpdateSettings(isShuffled, repeatMode)
	}

	// Return updated state
	state := ms.playerState.GetState()
	json.NewEncoder(w).Encode(state)
}

// handleTrackPlay updates player state when a track starts playing
func (ms *MusicServer) handleTrackPlay(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Extract track ID from URL path
	pathParts := strings.Split(r.URL.Path, "/")
	if len(pathParts) < 4 {
		http.Error(w, "Invalid track ID", http.StatusBadRequest)
		return
	}

	trackIDStr := pathParts[3]
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

	// Update player state
	ms.playerState.UpdateTrack(track)
	ms.playerState.UpdatePlaybackState(true)

	w.Header().Set("Content-Type", "application/json")
	ms.setCORSHeaders(w)

	response := map[string]string{"status": "success", "message": "Track play state updated"}
	json.NewEncoder(w).Encode(response)

	log.Printf("Now playing: %s - %s", track.Artist, track.Title)
}
