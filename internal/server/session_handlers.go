package server

import (
	"encoding/json"
	"log"
	"net/http"
	"strings"
)

// handleCreateSession creates a new client session
func (ms *MusicServer) handleCreateSession(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	ms.setCORSHeaders(w)

	var req struct {
		DeviceName string `json:"deviceName,omitempty"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		// If JSON decode fails, just use defaults
		req.DeviceName = "Unknown Device"
	}

	// Extract user agent and IP
	userAgent := r.Header.Get("User-Agent")
	ipAddress := r.RemoteAddr
	if forwarded := r.Header.Get("X-Forwarded-For"); forwarded != "" {
		ipAddress = forwarded
	}

	// If no device name provided, try to guess from user agent
	if req.DeviceName == "" || req.DeviceName == "Unknown Device" {
		req.DeviceName = guessDeviceName(userAgent)
	}

	session := ms.sessionManager.CreateSession(userAgent, ipAddress, req.DeviceName)

	log.Printf("New session created: %s (%s)", session.ID, req.DeviceName)

	json.NewEncoder(w).Encode(map[string]interface{}{
		"sessionId":  session.ID,
		"deviceName": session.DeviceName,
		"isActive":   ms.sessionManager.IsActiveSession(session.ID),
	})
}

// handleGetSessions returns all active sessions
func (ms *MusicServer) handleGetSessions(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	ms.setCORSHeaders(w)

	sessions := ms.sessionManager.GetAllSessions()
	activeSession := ms.sessionManager.GetActiveSession()

	response := make(map[string]interface{})
	response["sessions"] = sessions
	if activeSession != nil {
		response["activeSessionId"] = activeSession.Session.ID
	}

	json.NewEncoder(w).Encode(response)
}

// handleSetActiveSession sets the active session for Discord RPC
func (ms *MusicServer) handleSetActiveSession(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	ms.setCORSHeaders(w)

	var req struct {
		SessionID string `json:"sessionId"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid JSON", http.StatusBadRequest)
		return
	}

	success := ms.sessionManager.SetActiveSession(req.SessionID)
	if !success {
		http.Error(w, "Session not found", http.StatusNotFound)
		return
	}

	log.Printf("Active session changed to: %s", req.SessionID)

	json.NewEncoder(w).Encode(map[string]interface{}{
		"success":         true,
		"activeSessionId": req.SessionID,
	})
}

// handleUpdatePlayerStateSession updates player state for a specific session
func (ms *MusicServer) handleUpdatePlayerStateSession(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	ms.setCORSHeaders(w)

	var req struct {
		SessionID     string   `json:"sessionId"`
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
		log.Printf("Error decoding session player state JSON: %v", err)
		http.Error(w, "Invalid JSON", http.StatusBadRequest)
		return
	}

	if req.SessionID == "" {
		http.Error(w, "Session ID required", http.StatusBadRequest)
		return
	}

	// Update session activity
	ms.sessionManager.UpdateSessionActivity(req.SessionID)

	// Handle track updates
	if req.TrackID != nil {
		if *req.TrackID == 0 {
			// Clear track for this session
			ms.sessionManager.UpdatePlayerState(req.SessionID, nil, false, 0)
		} else {
			// Get track from database
			track, err := ms.db.GetTrackByID(*req.TrackID)
			if err != nil {
				http.Error(w, "Track not found", http.StatusNotFound)
				return
			}

			isPlaying := false
			currentTime := 0
			if req.IsPlaying != nil {
				isPlaying = *req.IsPlaying
			}
			if req.CurrentTime != nil {
				currentTime = *req.CurrentTime
			}

			ms.sessionManager.UpdatePlayerState(req.SessionID, track, isPlaying, currentTime)
		}
	} else if req.IsPlaying != nil || req.CurrentTime != nil {
		// Update playing state/time for existing track
		session := ms.sessionManager.GetSession(req.SessionID)
		if session != nil {
			isPlaying := session.IsPlaying
			currentTime := session.CurrentTime

			if req.IsPlaying != nil {
				isPlaying = *req.IsPlaying
			}
			if req.CurrentTime != nil {
				currentTime = *req.CurrentTime
			}

			ms.sessionManager.UpdatePlayerState(req.SessionID, session.Track, isPlaying, currentTime)
		}
	}

	// Get updated session state
	session := ms.sessionManager.GetSession(req.SessionID)
	if session == nil {
		http.Error(w, "Session not found", http.StatusNotFound)
		return
	}

	response := map[string]interface{}{
		"success":  true,
		"isActive": ms.sessionManager.IsActiveSession(req.SessionID),
		"session":  session,
	}

	json.NewEncoder(w).Encode(response)
}

// guessDeviceName tries to guess device name from user agent
func guessDeviceName(userAgent string) string {
	ua := strings.ToLower(userAgent)

	if strings.Contains(ua, "mobile") || strings.Contains(ua, "android") {
		if strings.Contains(ua, "android") {
			return "Android Device"
		}
		return "Mobile Device"
	}

	if strings.Contains(ua, "iphone") {
		return "iPhone"
	}

	if strings.Contains(ua, "ipad") {
		return "iPad"
	}

	if strings.Contains(ua, "mac") || strings.Contains(ua, "macintosh") {
		return "Mac"
	}

	if strings.Contains(ua, "windows") {
		return "Windows PC"
	}

	if strings.Contains(ua, "linux") {
		return "Linux PC"
	}

	if strings.Contains(ua, "chrome") {
		return "Chrome Browser"
	}

	if strings.Contains(ua, "firefox") {
		return "Firefox Browser"
	}

	if strings.Contains(ua, "safari") {
		return "Safari Browser"
	}

	return "Web Browser"
}
