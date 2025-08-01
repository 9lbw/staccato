package session

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"log"
	"sync"
	"time"

	"staccato/pkg/models"
)

// PriorityMode defines how session priority is handled
type PriorityMode int

const (
	// PlayPriority: The client that clicks play gets priority, stopping other clients (like Spotify)
	PlayPriority PriorityMode = iota
	// SessionPriority: Current implementation - most recent session gets priority
	SessionPriority
	// SessionPlayPriority: Mix of both - switches other clients to background but keeps music playing
	SessionPlayPriority
)

// Session represents a client session
type Session struct {
	ID           string    `json:"id"`
	UserAgent    string    `json:"userAgent"`
	IPAddress    string    `json:"ipAddress"`
	LastActivity time.Time `json:"lastActivity"`
	IsActive     bool      `json:"isActive"`
	DeviceName   string    `json:"deviceName"`
	IsBackground bool      `json:"isBackground"` // New field for background sessions
}

// PlayerSession represents a player session with its state
type PlayerSession struct {
	Session     *Session      `json:"session"`
	Track       *models.Track `json:"track,omitempty"`
	IsPlaying   bool          `json:"isPlaying"`
	CurrentTime int           `json:"currentTime"`
	LastUpdate  time.Time     `json:"lastUpdate"`
	WasPaused   bool          `json:"wasPaused"`   // Track if session was paused by priority system
	PlayStarted time.Time     `json:"playStarted"` // When this session started playing
}

// SessionManager manages multiple client sessions
type SessionManager struct {
	sessions        map[string]*PlayerSession
	activeSession   string       // Session ID that controls Discord RPC
	priorityMode    PriorityMode // How priority is handled
	mutex           sync.RWMutex
	activityTimeout time.Duration
	onSessionChange func(activeSession *PlayerSession, backgroundSessions []*PlayerSession) // Callback for session changes
	eventStreams    map[string][]chan map[string]interface{}                                // SSE channels per session
}

// NewSessionManager creates a new session manager
func NewSessionManager() *SessionManager {
	return &SessionManager{
		sessions:        make(map[string]*PlayerSession),
		priorityMode:    SessionPriority,                                // Default to current behavior
		activityTimeout: 30 * time.Second,                               // Sessions expire after 30 seconds of inactivity
		eventStreams:    make(map[string][]chan map[string]interface{}), // Initialize event streams
	}
}

// NewSessionManagerWithMode creates a new session manager with specified priority mode
func NewSessionManagerWithMode(mode PriorityMode) *SessionManager {
	return &SessionManager{
		sessions:        make(map[string]*PlayerSession),
		priorityMode:    mode,
		activityTimeout: 30 * time.Second,
		eventStreams:    make(map[string][]chan map[string]interface{}), // Initialize event streams
	}
}

// SetPriorityMode changes the priority mode
func (sm *SessionManager) SetPriorityMode(mode PriorityMode) {
	sm.mutex.Lock()
	defer sm.mutex.Unlock()
	sm.priorityMode = mode
}

// GetPriorityMode returns the current priority mode
func (sm *SessionManager) GetPriorityMode() PriorityMode {
	sm.mutex.RLock()
	defer sm.mutex.RUnlock()
	return sm.priorityMode
}

// SetSessionChangeCallback sets a callback function that gets called when active session changes
func (sm *SessionManager) SetSessionChangeCallback(callback func(activeSession *PlayerSession, backgroundSessions []*PlayerSession)) {
	sm.mutex.Lock()
	defer sm.mutex.Unlock()
	sm.onSessionChange = callback
}

// GenerateSessionID creates a new unique session ID
func (sm *SessionManager) GenerateSessionID() string {
	bytes := make([]byte, 16)
	rand.Read(bytes)
	return hex.EncodeToString(bytes)
}

// CreateSession creates a new session
func (sm *SessionManager) CreateSession(userAgent, ipAddress, deviceName string) *Session {
	sm.mutex.Lock()
	defer sm.mutex.Unlock()

	session := &Session{
		ID:           sm.GenerateSessionID(),
		UserAgent:    userAgent,
		IPAddress:    ipAddress,
		DeviceName:   deviceName,
		LastActivity: time.Now(),
		IsActive:     true,
		IsBackground: false,
	}

	sm.sessions[session.ID] = &PlayerSession{
		Session:    session,
		LastUpdate: time.Now(),
		WasPaused:  false,
	}

	// If this is the first session, make it active
	if sm.activeSession == "" {
		sm.activeSession = session.ID
		sm.triggerSessionChangeCallback()
	}

	return session
}

// GetSession retrieves a session by ID
func (sm *SessionManager) GetSession(sessionID string) *PlayerSession {
	sm.mutex.RLock()
	defer sm.mutex.RUnlock()

	return sm.sessions[sessionID]
}

// UpdateSessionActivity updates the last activity time for a session
func (sm *SessionManager) UpdateSessionActivity(sessionID string) {
	sm.mutex.Lock()
	defer sm.mutex.Unlock()

	if session, exists := sm.sessions[sessionID]; exists {
		session.Session.LastActivity = time.Now()
		session.LastUpdate = time.Now()
	}
}

// UpdatePlayerState updates the player state for a session
func (sm *SessionManager) UpdatePlayerState(sessionID string, track *models.Track, isPlaying bool, currentTime int) {
	sm.mutex.Lock()
	defer sm.mutex.Unlock()

	if session, exists := sm.sessions[sessionID]; exists {
		// Store previous playing state
		wasPlaying := session.IsPlaying

		session.Track = track
		session.IsPlaying = isPlaying
		session.CurrentTime = currentTime
		session.LastUpdate = time.Now()
		session.Session.LastActivity = time.Now()

		log.Printf("DEBUG: Session %s state change: wasPlaying=%v, nowPlaying=%v, priorityMode=%v",
			sessionID, wasPlaying, isPlaying, sm.priorityMode)

		// Handle priority logic based on mode
		if isPlaying && !wasPlaying {
			// Session just started playing
			session.PlayStarted = time.Now()
			log.Printf("DEBUG: Session %s just started playing, triggering priority logic", sessionID)

			switch sm.priorityMode {
			case PlayPriority:
				sm.handlePlayPriority(sessionID, isPlaying)
			case SessionPlayPriority:
				sm.handleSessionPlayPriority(sessionID, isPlaying)
			default: // SessionPriority
				// If this session just started playing and no active session, make it active
				if sm.activeSession == "" || !sm.isSessionActive(sm.activeSession) {
					sm.activeSession = sessionID
					sm.triggerSessionChangeCallback()
				}
			}
		} else if !isPlaying && wasPlaying {
			// Session just stopped playing, clear the paused flag
			session.WasPaused = false
			log.Printf("DEBUG: Session %s stopped playing, cleared paused flag", sessionID)
		} else if isPlaying {
			log.Printf("DEBUG: Session %s is playing but was already playing (wasPlaying=%v)", sessionID, wasPlaying)
		}
	}
}

// GetActiveSession returns the currently active session
func (sm *SessionManager) GetActiveSession() *PlayerSession {
	sm.mutex.Lock() // Changed to write lock for cleanup
	defer sm.mutex.Unlock()

	// Clean up expired sessions first
	sm.cleanupExpiredSessions()

	if sm.activeSession == "" {
		return nil
	}

	session := sm.sessions[sm.activeSession]
	if session == nil || !sm.isSessionActive(sm.activeSession) {
		// Active session is gone or inactive, find a new one
		sm.findNewActiveSession()
		if sm.activeSession == "" {
			return nil
		}
		session = sm.sessions[sm.activeSession]
	}

	return session
}

// SetActiveSession manually sets the active session
func (sm *SessionManager) SetActiveSession(sessionID string) bool {
	sm.mutex.Lock()
	defer sm.mutex.Unlock()

	if _, exists := sm.sessions[sessionID]; exists {
		oldActive := sm.activeSession
		sm.activeSession = sessionID

		// Update background status based on priority mode
		if sm.priorityMode == SessionPlayPriority {
			// Mark old active as background
			if oldActive != "" && oldActive != sessionID {
				if oldSession, exists := sm.sessions[oldActive]; exists {
					oldSession.Session.IsBackground = true
				}
			}
			// Mark new active as foreground
			sm.sessions[sessionID].Session.IsBackground = false
		}

		sm.triggerSessionChangeCallback()
		return true
	}
	return false
}

// GetAllSessions returns all active sessions
func (sm *SessionManager) GetAllSessions() map[string]*PlayerSession {
	sm.mutex.Lock() // Changed to write lock for cleanup
	defer sm.mutex.Unlock()

	// Clean up first
	sm.cleanupExpiredSessions()

	// Return a copy to avoid race conditions
	result := make(map[string]*PlayerSession)
	for id, session := range sm.sessions {
		result[id] = session
	}
	return result
}

// RemoveSession removes a session
func (sm *SessionManager) RemoveSession(sessionID string) {
	sm.mutex.Lock()
	defer sm.mutex.Unlock()

	delete(sm.sessions, sessionID)

	// If this was the active session, find a new one
	if sm.activeSession == sessionID {
		sm.findNewActiveSession()
		sm.triggerSessionChangeCallback()
	}
}

// isSessionActive checks if a session is still active (must be called with lock held)
func (sm *SessionManager) isSessionActive(sessionID string) bool {
	session, exists := sm.sessions[sessionID]
	if !exists {
		return false
	}

	return time.Since(session.Session.LastActivity) < sm.activityTimeout
}

// cleanupExpiredSessions removes inactive sessions (must be called with lock held)
func (sm *SessionManager) cleanupExpiredSessions() {
	now := time.Now()
	for id, session := range sm.sessions {
		if now.Sub(session.Session.LastActivity) > sm.activityTimeout {
			delete(sm.sessions, id)
			if sm.activeSession == id {
				sm.activeSession = ""
			}
		}
	}
}

// findNewActiveSession finds a new active session from playing sessions (must be called with lock held)
func (sm *SessionManager) findNewActiveSession() {
	oldActive := sm.activeSession
	sm.activeSession = ""

	// Priority 1: Find a playing session
	var playingSessions []*PlayerSession
	var playingIDs []string

	for id, session := range sm.sessions {
		if session.IsPlaying && sm.isSessionActive(id) {
			playingSessions = append(playingSessions, session)
			playingIDs = append(playingIDs, id)
		}
	}

	// If we have playing sessions, pick the most recently started
	if len(playingSessions) > 0 {
		mostRecentID := playingIDs[0]
		mostRecentStartTime := playingSessions[0].PlayStarted

		for i := 1; i < len(playingSessions); i++ {
			if playingSessions[i].PlayStarted.After(mostRecentStartTime) {
				mostRecentID = playingIDs[i]
				mostRecentStartTime = playingSessions[i].PlayStarted
			}
		}

		sm.activeSession = mostRecentID
	} else {
		// Priority 2: Find the most recently active session
		var mostRecent *PlayerSession
		var mostRecentID string

		for id, session := range sm.sessions {
			if sm.isSessionActive(id) {
				if mostRecent == nil || session.Session.LastActivity.After(mostRecent.Session.LastActivity) {
					mostRecent = session
					mostRecentID = id
				}
			}
		}

		if mostRecent != nil {
			sm.activeSession = mostRecentID
		}
	}

	// Update background status if needed
	if sm.priorityMode == SessionPlayPriority {
		for id, session := range sm.sessions {
			session.Session.IsBackground = (id != sm.activeSession)
		}
	}

	// Trigger callback if active session changed
	if sm.activeSession != oldActive {
		sm.triggerSessionChangeCallback()
	}
}

// IsActiveSession checks if the given session ID is the active one
func (sm *SessionManager) IsActiveSession(sessionID string) bool {
	sm.mutex.RLock()
	defer sm.mutex.RUnlock()

	return sm.activeSession == sessionID && sm.isSessionActive(sessionID)
}

// ShouldSessionPause checks if a session should be paused due to priority rules
func (sm *SessionManager) ShouldSessionPause(sessionID string) (bool, string) {
	sm.mutex.RLock()
	defer sm.mutex.RUnlock()

	session := sm.sessions[sessionID]
	if session == nil {
		return false, ""
	}

	// Check if this session was paused by another session taking priority
	if session.WasPaused {
		// Clear the WasPaused flag after reporting it once
		sm.mutex.RUnlock()
		sm.mutex.Lock()
		session.WasPaused = false
		sm.mutex.Unlock()
		sm.mutex.RLock()

		activeSession := sm.sessions[sm.activeSession]
		if activeSession != nil {
			return true, fmt.Sprintf("Paused by %s", activeSession.Session.DeviceName)
		}
		return true, "Paused by another session"
	}

	// Original logic for checking if session should be paused based on priority mode
	if !session.IsPlaying {
		return false, ""
	}

	switch sm.priorityMode {
	case PlayPriority:
		// In Play Priority mode, only the active session should play
		if sm.activeSession != sessionID {
			activeSession := sm.sessions[sm.activeSession]
			if activeSession != nil {
				return true, fmt.Sprintf("Paused by %s", activeSession.Session.DeviceName)
			}
			return true, "Paused by another session"
		}
	case SessionPlayPriority:
		// In Session + Play Priority mode, non-active sessions can play but are marked as background
		// No need to pause, just mark as background
		return false, ""
	default: // SessionPriority
		// In Session Priority mode, all sessions can play simultaneously
		return false, ""
	}

	return false, ""
}

// GetBackgroundSessions returns all non-active sessions
func (sm *SessionManager) GetBackgroundSessions() []*PlayerSession {
	sm.mutex.RLock()
	defer sm.mutex.RUnlock()

	background := make([]*PlayerSession, 0)
	for id, session := range sm.sessions {
		if id != sm.activeSession && sm.isSessionActive(id) {
			background = append(background, session)
		}
	}
	return background
}

// PauseOtherSessions pauses all sessions except the specified one (for PlayPriority mode)
func (sm *SessionManager) PauseOtherSessions(exceptSessionID string) []*PlayerSession {
	sm.mutex.Lock()
	defer sm.mutex.Unlock()

	paused := make([]*PlayerSession, 0)
	for id, session := range sm.sessions {
		if id != exceptSessionID && session.IsPlaying {
			session.IsPlaying = false
			session.WasPaused = true
			session.LastUpdate = time.Now()
			paused = append(paused, session)
		}
	}
	return paused
}

// triggerSessionChangeCallback calls the session change callback if set (must be called with lock held)
func (sm *SessionManager) triggerSessionChangeCallback() {
	if sm.onSessionChange != nil {
		active := sm.sessions[sm.activeSession]
		background := make([]*PlayerSession, 0)

		for id, session := range sm.sessions {
			if id != sm.activeSession && sm.isSessionActive(id) {
				background = append(background, session)
			}
		}

		// Call callback without lock to avoid deadlock
		go sm.onSessionChange(active, background)
	}
}

// handlePlayPriority implements play priority logic (must be called with lock held)
func (sm *SessionManager) handlePlayPriority(sessionID string, isPlaying bool) {
	if !isPlaying {
		return // Only handle when starting to play
	}

	// If a session starts playing, it becomes active and pauses all others
	if sm.activeSession != sessionID {
		pauseReason := fmt.Sprintf("Paused by %s", sm.sessions[sessionID].Session.DeviceName)

		// Pause ALL other sessions that are playing
		for id, session := range sm.sessions {
			if id != sessionID && session.IsPlaying {
				session.IsPlaying = false
				session.WasPaused = true
				session.LastUpdate = time.Now()
				log.Printf("Pausing session %s (%s) due to play priority from %s",
					id, session.Session.DeviceName, sm.sessions[sessionID].Session.DeviceName)

				// Immediately broadcast pause event to this session
				go sm.broadcastPauseEvent(id, pauseReason)
			}
		}

		// Set new active session
		sm.activeSession = sessionID
		sm.sessions[sessionID].PlayStarted = time.Now()
		sm.triggerSessionChangeCallback()
		log.Printf("Session %s (%s) took play priority", sessionID, sm.sessions[sessionID].Session.DeviceName)
	}
}

// handleSessionPlayPriority implements session + play priority logic (must be called with lock held)
func (sm *SessionManager) handleSessionPlayPriority(sessionID string, isPlaying bool) {
	if !isPlaying {
		return // Only handle when starting to play
	}

	// If a session starts playing, it becomes active but others continue in background
	if sm.activeSession != sessionID {
		oldActive := sm.activeSession

		// Set new active session for Discord RPC
		sm.activeSession = sessionID
		sm.sessions[sessionID].PlayStarted = time.Now()

		// Mark old active session as background but keep it playing
		if oldActive != "" {
			if oldSession, exists := sm.sessions[oldActive]; exists {
				oldSession.Session.IsBackground = true
			}
		}

		// Mark new active session as foreground
		sm.sessions[sessionID].Session.IsBackground = false

		sm.triggerSessionChangeCallback()
	}
}

// RegisterEventStream registers a Server-Sent Events channel for a session
func (sm *SessionManager) RegisterEventStream(sessionID string, eventChan chan map[string]interface{}) {
	sm.mutex.Lock()
	defer sm.mutex.Unlock()

	if sm.eventStreams[sessionID] == nil {
		sm.eventStreams[sessionID] = make([]chan map[string]interface{}, 0)
	}
	sm.eventStreams[sessionID] = append(sm.eventStreams[sessionID], eventChan)
	log.Printf("Registered event stream for session %s (total streams: %d)", sessionID, len(sm.eventStreams[sessionID]))
}

// UnregisterEventStream removes a Server-Sent Events channel for a session
func (sm *SessionManager) UnregisterEventStream(sessionID string, eventChan chan map[string]interface{}) {
	sm.mutex.Lock()
	defer sm.mutex.Unlock()

	if streams, exists := sm.eventStreams[sessionID]; exists {
		for i, ch := range streams {
			if ch == eventChan {
				// Remove this channel from the slice
				sm.eventStreams[sessionID] = append(streams[:i], streams[i+1:]...)
				break
			}
		}

		// Clean up empty session entries
		if len(sm.eventStreams[sessionID]) == 0 {
			delete(sm.eventStreams, sessionID)
		}

		log.Printf("Unregistered event stream for session %s", sessionID)
	}
}

// broadcastPauseEvent sends pause commands to sessions via their event streams
func (sm *SessionManager) broadcastPauseEvent(sessionID string, pauseReason string) {
	if streams, exists := sm.eventStreams[sessionID]; exists {
		event := map[string]interface{}{
			"type":        "pauseCommand",
			"sessionId":   sessionID,
			"shouldPause": true,
			"pauseReason": pauseReason,
		}

		// Send to all event streams for this session
		for _, eventChan := range streams {
			select {
			case eventChan <- event:
				// Event sent successfully
			default:
				// Channel full or closed, skip
				log.Printf("Failed to send pause event to session %s (channel full/closed)", sessionID)
			}
		}
	}
}
