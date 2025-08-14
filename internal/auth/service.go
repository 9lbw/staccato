package auth

import (
	"fmt"
	"time"

	"staccato/internal/config"
)

// Service provides authentication functionality
type Service struct {
	config         *config.AuthConfig
	userStore      *UserStore
	sessionManager *SessionManager
	enabled        bool
}

// NewService creates a new authentication service
func NewService(config *config.AuthConfig) (*Service, error) {
	if !config.Enabled {
		return &Service{
			config:  config,
			enabled: false,
		}, nil
	}

	// Parse session duration
	duration, err := time.ParseDuration(config.SessionDuration)
	if err != nil {
		return nil, fmt.Errorf("invalid session duration: %w", err)
	}

	// Create user store
	userStore, err := NewUserStore(config.UsersFilePath)
	if err != nil {
		return nil, fmt.Errorf("failed to create user store: %w", err)
	}

	// Create session manager
	sessionManager := NewSessionManager(duration, config.SecureCookies)

	return &Service{
		config:         config,
		userStore:      userStore,
		sessionManager: sessionManager,
		enabled:        true,
	}, nil
}

// IsEnabled returns whether authentication is enabled
func (s *Service) IsEnabled() bool {
	return s.enabled
}

// Login attempts to authenticate a user and create a session
func (s *Service) Login(username, password string) (*Session, error) {
	if !s.enabled {
		return nil, fmt.Errorf("authentication is disabled")
	}

	if !s.userStore.Authenticate(username, password) {
		return nil, fmt.Errorf("invalid credentials")
	}

	session, err := s.sessionManager.CreateSession(username)
	if err != nil {
		return nil, fmt.Errorf("failed to create session: %w", err)
	}

	return session, nil
}

// ValidateSession checks if a session ID is valid
func (s *Service) ValidateSession(sessionID string) (*Session, bool) {
	if !s.enabled {
		return nil, true // If auth is disabled, consider all sessions valid
	}

	return s.sessionManager.GetSession(sessionID)
}

// Logout invalidates a session
func (s *Service) Logout(sessionID string) {
	if !s.enabled {
		return
	}

	s.sessionManager.DeleteSession(sessionID)
}

// RefreshSession extends a session's expiration
func (s *Service) RefreshSession(sessionID string) bool {
	if !s.enabled {
		return true
	}

	return s.sessionManager.RefreshSession(sessionID)
}

// GetSessionManager returns the session manager (for middleware)
func (s *Service) GetSessionManager() *SessionManager {
	return s.sessionManager
}
