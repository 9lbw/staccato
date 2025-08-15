package auth

import (
	"fmt"
	"time"

	"staccato/internal/config"
)

// Service provides authentication functionality
type Service struct {
	config            *config.AuthConfig
	userStore         *UserStore
	sessionManager    *SessionManager
	userFolderManager *UserFolderManager
	enabled           bool
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

	// Create user folder manager
	userFolderManager := NewUserFolderManager(config.UserFolders, config.UserMusicPath)

	return &Service{
		config:            config,
		userStore:         userStore,
		sessionManager:    sessionManager,
		userFolderManager: userFolderManager,
		enabled:           true,
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

// IsRegistrationAllowed returns whether user registration is enabled
func (s *Service) IsRegistrationAllowed() bool {
	return s.enabled && s.config.AllowRegistration
}

// Register creates a new user account
func (s *Service) Register(username, password string) error {
	if !s.IsRegistrationAllowed() {
		return fmt.Errorf("registration is disabled")
	}

	// Validate input
	if username == "" || password == "" {
		return fmt.Errorf("username and password are required")
	}

	// Register user in store
	if err := s.userStore.RegisterUser(username, password); err != nil {
		return fmt.Errorf("failed to register user: %w", err)
	}

	// Create user folder if enabled
	if err := s.userFolderManager.CreateUserFolder(username); err != nil {
		// If folder creation fails, we should probably remove the user
		// but for now, we'll just return the error
		return fmt.Errorf("failed to create user folder: %w", err)
	}

	return nil
}

// GetUserFolderManager returns the user folder manager
func (s *Service) GetUserFolderManager() *UserFolderManager {
	return s.userFolderManager
}
