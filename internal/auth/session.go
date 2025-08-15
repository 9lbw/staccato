package auth

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"net/http"
	"sync"
	"time"
)

// Session represents an active user session
type Session struct {
	ID        string
	Username  string
	CreatedAt time.Time
	ExpiresAt time.Time
}

// SessionManager manages user sessions
type SessionManager struct {
	sessions      map[string]*Session
	mutex         sync.RWMutex
	duration      time.Duration
	cookieName    string
	secureCookies bool
}

// NewSessionManager creates a new session manager
func NewSessionManager(duration time.Duration, secureCookies bool) *SessionManager {
	sm := &SessionManager{
		sessions:      make(map[string]*Session),
		duration:      duration,
		cookieName:    "staccato_session",
		secureCookies: secureCookies,
	}

	// Start cleanup goroutine
	go sm.cleanupExpiredSessions()

	return sm
}

// CreateSession creates a new session for the user
func (sm *SessionManager) CreateSession(username string) (*Session, error) {
	sessionID, err := generateSessionID()
	if err != nil {
		return nil, fmt.Errorf("failed to generate session ID: %w", err)
	}

	now := time.Now()
	session := &Session{
		ID:        sessionID,
		Username:  username,
		CreatedAt: now,
		ExpiresAt: now.Add(sm.duration),
	}

	sm.mutex.Lock()
	sm.sessions[sessionID] = session
	sm.mutex.Unlock()

	return session, nil
}

// GetSession retrieves a session by ID
func (sm *SessionManager) GetSession(sessionID string) (*Session, bool) {
	sm.mutex.RLock()
	session, exists := sm.sessions[sessionID]
	sm.mutex.RUnlock()

	if !exists {
		return nil, false
	}

	// Check if session is expired
	if time.Now().After(session.ExpiresAt) {
		sm.DeleteSession(sessionID)
		return nil, false
	}

	return session, true
}

// DeleteSession removes a session
func (sm *SessionManager) DeleteSession(sessionID string) {
	sm.mutex.Lock()
	delete(sm.sessions, sessionID)
	sm.mutex.Unlock()
}

// DeleteUserSessions removes all sessions for a specific user
func (sm *SessionManager) DeleteUserSessions(username string) {
	sm.mutex.Lock()
	defer sm.mutex.Unlock()

	for id, session := range sm.sessions {
		if session.Username == username {
			delete(sm.sessions, id)
		}
	}
}

// RefreshSession extends the session expiration time
func (sm *SessionManager) RefreshSession(sessionID string) bool {
	sm.mutex.Lock()
	defer sm.mutex.Unlock()

	session, exists := sm.sessions[sessionID]
	if !exists {
		return false
	}

	// Check if session is expired
	if time.Now().After(session.ExpiresAt) {
		delete(sm.sessions, sessionID)
		return false
	}

	// Extend expiration
	session.ExpiresAt = time.Now().Add(sm.duration)
	return true
}

// SetSessionCookie sets the session cookie on the response
func (sm *SessionManager) SetSessionCookie(w http.ResponseWriter, session *Session) {
	cookie := &http.Cookie{
		Name:     sm.cookieName,
		Value:    session.ID,
		Expires:  session.ExpiresAt,
		HttpOnly: true,
		Secure:   sm.secureCookies,
		SameSite: http.SameSiteStrictMode,
		Path:     "/",
	}

	http.SetCookie(w, cookie)
}

// ClearSessionCookie removes the session cookie
func (sm *SessionManager) ClearSessionCookie(w http.ResponseWriter) {
	cookie := &http.Cookie{
		Name:     sm.cookieName,
		Value:    "",
		Expires:  time.Unix(0, 0),
		HttpOnly: true,
		Secure:   sm.secureCookies,
		SameSite: http.SameSiteStrictMode,
		Path:     "/",
	}

	http.SetCookie(w, cookie)
}

// GetSessionFromRequest extracts session from request cookie
func (sm *SessionManager) GetSessionFromRequest(r *http.Request) (*Session, bool) {
	cookie, err := r.Cookie(sm.cookieName)
	if err != nil {
		return nil, false
	}

	return sm.GetSession(cookie.Value)
}

// cleanupExpiredSessions periodically removes expired sessions
func (sm *SessionManager) cleanupExpiredSessions() {
	ticker := time.NewTicker(1 * time.Hour)
	defer ticker.Stop()

	for range ticker.C {
		now := time.Now()
		sm.mutex.Lock()

		for id, session := range sm.sessions {
			if now.After(session.ExpiresAt) {
				delete(sm.sessions, id)
			}
		}

		sm.mutex.Unlock()
	}
}

// generateSessionID generates a cryptographically secure session ID
func generateSessionID() (string, error) {
	bytes := make([]byte, 32)
	if _, err := rand.Read(bytes); err != nil {
		return "", err
	}
	return hex.EncodeToString(bytes), nil
}
