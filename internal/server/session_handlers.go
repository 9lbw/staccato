package server

import (
	"encoding/json"
	"log"
	"net/http"
	"strings"

	"staccato/internal/session"
) // handleCreateSession creates a new client session
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

	log.Printf("Session update from %s: trackId=%v, isPlaying=%v, currentTime=%v",
		req.SessionID, req.TrackID, req.IsPlaying, req.CurrentTime)

	// Handle track updates
	if req.TrackID != nil {
		if *req.TrackID == 0 {
			// Clear track for this session
			log.Printf("Clearing track for session %s", req.SessionID)
			ms.sessionManager.UpdatePlayerState(req.SessionID, nil, false, 0)
		} else {
			// Get track from database
			track, err := ms.db.GetTrackByID(*req.TrackID)
			if err != nil {
				log.Printf("Track not found for ID %d: %v", *req.TrackID, err)
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

			log.Printf("Updating session %s: track %s - %s, playing=%v",
				req.SessionID, track.Artist, track.Title, isPlaying)
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

// handleSessionPriority handles changing session priority mode
func (ms *MusicServer) handleSessionPriority(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	ms.setCORSHeaders(w)

	if r.Method == "GET" {
		// Get current priority mode
		mode := ms.sessionManager.GetPriorityMode()
		modeStr := "session"
		switch mode {
		case session.PlayPriority:
			modeStr = "play"
		case session.SessionPlayPriority:
			modeStr = "session_play"
		}

		response := map[string]interface{}{
			"priorityMode": modeStr,
		}
		json.NewEncoder(w).Encode(response)
		return
	}

	if r.Method != "POST" {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req struct {
		PriorityMode string `json:"priorityMode"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		log.Printf("Error decoding priority mode JSON: %v", err)
		http.Error(w, "Invalid JSON", http.StatusBadRequest)
		return
	}

	var mode session.PriorityMode
	switch req.PriorityMode {
	case "play":
		mode = session.PlayPriority
	case "session_play":
		mode = session.SessionPlayPriority
	case "session":
		mode = session.SessionPriority
	default:
		http.Error(w, "Invalid priority mode. Must be 'play', 'session', or 'session_play'", http.StatusBadRequest)
		return
	}

	ms.sessionManager.SetPriorityMode(mode)

	log.Printf("Session priority mode changed to: %s", req.PriorityMode)

	response := map[string]interface{}{
		"success":      true,
		"priorityMode": req.PriorityMode,
		"message":      "Priority mode updated successfully",
	}
	json.NewEncoder(w).Encode(response)
}

// handleSessionConfig handles getting and updating session configuration
func (ms *MusicServer) handleSessionConfig(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	ms.setCORSHeaders(w)

	if r.Method == "GET" {
		// Get current session configuration
		mode := ms.sessionManager.GetPriorityMode()
		modeStr := "session"
		switch mode {
		case session.PlayPriority:
			modeStr = "play"
		case session.SessionPlayPriority:
			modeStr = "session_play"
		}

		response := map[string]interface{}{
			"priorityMode":    modeStr,
			"discordRpcMode":  ms.config.Session.DiscordRPCMode,
			"activityTimeout": ms.config.Session.ActivityTimeout,
			"modes": map[string]string{
				"play":         "Play Priority - Client that clicks play gets priority, pausing others",
				"session":      "Session Priority - Most recent session controls Discord RPC",
				"session_play": "Session + Play Priority - Mix of both, background sessions continue playing",
			},
		}
		json.NewEncoder(w).Encode(response)
		return
	}

	if r.Method != "POST" {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req struct {
		PriorityMode    *string `json:"priorityMode,omitempty"`
		DiscordRPCMode  *string `json:"discordRpcMode,omitempty"`
		ActivityTimeout *int    `json:"activityTimeout,omitempty"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		log.Printf("Error decoding session config JSON: %v", err)
		http.Error(w, "Invalid JSON", http.StatusBadRequest)
		return
	}

	// Update priority mode if provided
	if req.PriorityMode != nil {
		var mode session.PriorityMode
		switch *req.PriorityMode {
		case "play":
			mode = session.PlayPriority
		case "session_play":
			mode = session.SessionPlayPriority
		case "session":
			mode = session.SessionPriority
		default:
			http.Error(w, "Invalid priority mode. Must be 'play', 'session', or 'session_play'", http.StatusBadRequest)
			return
		}
		ms.sessionManager.SetPriorityMode(mode)
	}

	// Update config values if provided
	if req.DiscordRPCMode != nil {
		ms.config.Session.DiscordRPCMode = *req.DiscordRPCMode
	}
	if req.ActivityTimeout != nil {
		ms.config.Session.ActivityTimeout = *req.ActivityTimeout
	}

	response := map[string]interface{}{
		"success": true,
		"message": "Session configuration updated successfully",
	}
	json.NewEncoder(w).Encode(response)
}

// handleSessionEvents provides session state polling for cross-session communication
func (ms *MusicServer) handleSessionEvents(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	ms.setCORSHeaders(w)

	// Get session ID from query params
	sessionID := r.URL.Query().Get("sessionId")
	if sessionID == "" {
		http.Error(w, "Session ID required", http.StatusBadRequest)
		return
	}

	// Get all sessions and active session
	sessions := ms.sessionManager.GetAllSessions()
	activeSession := ms.sessionManager.GetActiveSession()
	mySession := ms.sessionManager.GetSession(sessionID)

	// Check if this session should pause due to priority rules
	shouldPause, pauseReason := ms.sessionManager.ShouldSessionPause(sessionID)

	response := map[string]interface{}{
		"sessionId":     sessionID,
		"isActive":      ms.sessionManager.IsActiveSession(sessionID),
		"shouldPause":   shouldPause,
		"pauseReason":   pauseReason,
		"activeSession": activeSession,
		"mySession":     mySession,
		"totalSessions": len(sessions),
		"priorityMode": func() string {
			switch ms.sessionManager.GetPriorityMode() {
			case session.PlayPriority:
				return "play"
			case session.SessionPlayPriority:
				return "session_play"
			default:
				return "session"
			}
		}(),
	}

	json.NewEncoder(w).Encode(response)
}

// handleSessionStream provides real-time session events via Server-Sent Events
func (ms *MusicServer) handleSessionStream(w http.ResponseWriter, r *http.Request) {
	// Get session ID from query params
	sessionID := r.URL.Query().Get("sessionId")
	if sessionID == "" {
		http.Error(w, "Session ID required", http.StatusBadRequest)
		return
	}

	// Set SSE headers
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	ms.setCORSHeaders(w)

	// Create a channel for this client
	clientChan := make(chan map[string]interface{}, 10)

	// Register this client with the session manager
	ms.sessionManager.RegisterEventStream(sessionID, clientChan)

	// Clean up when client disconnects
	defer func() {
		ms.sessionManager.UnregisterEventStream(sessionID, clientChan)
		close(clientChan)
	}()

	// Send initial state
	shouldPause, pauseReason := ms.sessionManager.ShouldSessionPause(sessionID)
	initialEvent := map[string]interface{}{
		"type":        "sessionState",
		"sessionId":   sessionID,
		"isActive":    ms.sessionManager.IsActiveSession(sessionID),
		"shouldPause": shouldPause,
		"pauseReason": pauseReason,
	}

	eventData, _ := json.Marshal(initialEvent)
	w.Write([]byte("data: " + string(eventData) + "\n\n"))
	w.(http.Flusher).Flush()

	// Listen for events or client disconnect
	for {
		select {
		case event, ok := <-clientChan:
			if !ok {
				return // Channel closed
			}

			eventData, err := json.Marshal(event)
			if err != nil {
				log.Printf("Error marshaling SSE event: %v", err)
				continue
			}

			_, err = w.Write([]byte("data: " + string(eventData) + "\n\n"))
			if err != nil {
				log.Printf("SSE client disconnected: %v", err)
				return
			}

			w.(http.Flusher).Flush()

		case <-r.Context().Done():
			// Client disconnected
			return
		}
	}
}
