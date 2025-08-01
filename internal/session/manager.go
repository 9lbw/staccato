package session

import (
	"crypto/rand"
	"encoding/hex"
	"sync"
	"time"

	"staccato/pkg/models"
)

// Session represents a client session
type Session struct {
	ID           string    `json:"id"`
	UserAgent    string    `json:"userAgent"`
	IPAddress    string    `json:"ipAddress"`
	LastActivity time.Time `json:"lastActivity"`
	IsActive     bool      `json:"isActive"`
	DeviceName   string    `json:"deviceName"`
}

// PlayerSession represents a player session with its state
type PlayerSession struct {
	Session     *Session      `json:"session"`
	Track       *models.Track `json:"track,omitempty"`
	IsPlaying   bool          `json:"isPlaying"`
	CurrentTime int           `json:"currentTime"`
	LastUpdate  time.Time     `json:"lastUpdate"`
}

// SessionManager manages multiple client sessions
type SessionManager struct {
	sessions        map[string]*PlayerSession
	activeSession   string // Session ID that controls Discord RPC
	mutex           sync.RWMutex
	activityTimeout time.Duration
}

// NewSessionManager creates a new session manager
func NewSessionManager() *SessionManager {
	return &SessionManager{
		sessions:        make(map[string]*PlayerSession),
		activityTimeout: 30 * time.Second, // Sessions expire after 30 seconds of inactivity
	}
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
	}

	sm.sessions[session.ID] = &PlayerSession{
		Session:    session,
		LastUpdate: time.Now(),
	}

	// If this is the first session, make it active
	if sm.activeSession == "" {
		sm.activeSession = session.ID
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
		session.Track = track
		session.IsPlaying = isPlaying
		session.CurrentTime = currentTime
		session.LastUpdate = time.Now()
		session.Session.LastActivity = time.Now()

		// If this session just started playing and no active session, make it active
		if isPlaying && (sm.activeSession == "" || !sm.isSessionActive(sm.activeSession)) {
			sm.activeSession = sessionID
		}
	}
}

// GetActiveSession returns the currently active session
func (sm *SessionManager) GetActiveSession() *PlayerSession {
	sm.mutex.RLock()
	defer sm.mutex.RUnlock()

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
		sm.activeSession = sessionID
		return true
	}
	return false
}

// GetAllSessions returns all active sessions
func (sm *SessionManager) GetAllSessions() map[string]*PlayerSession {
	sm.mutex.RLock()
	defer sm.mutex.RUnlock()

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
	sm.activeSession = ""

	// First, try to find a playing session
	for id, session := range sm.sessions {
		if session.IsPlaying && sm.isSessionActive(id) {
			sm.activeSession = id
			return
		}
	}

	// If no playing session, just pick the most recent active one
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

// IsActiveSession checks if the given session ID is the active one
func (sm *SessionManager) IsActiveSession(sessionID string) bool {
	sm.mutex.RLock()
	defer sm.mutex.RUnlock()

	return sm.activeSession == sessionID && sm.isSessionActive(sessionID)
}
