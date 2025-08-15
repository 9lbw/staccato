package auth

import (
	"fmt"
	"time"

	"staccato/internal/config"

	"github.com/fsnotify/fsnotify"
	"github.com/sirupsen/logrus"
)

// Service provides authentication functionality
type Service struct {
	config            *config.AuthConfig
	userStore         *UserStore
	sessionManager    *SessionManager
	userFolderManager *UserFolderManager
	enabled           bool
	logger            *logrus.Logger
	usersWatcher      *fsnotify.Watcher
	cleanupCallback   func(username string) error // Callback for cleaning up user data
}

// NewService creates a new authentication service
func NewService(config *config.AuthConfig) (*Service, error) {
	// Initialize logger
	logger := logrus.New()
	logger.SetFormatter(&logrus.JSONFormatter{})

	if !config.Enabled {
		return &Service{
			config:  config,
			enabled: false,
			logger:  logger,
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
		logger:            logger,
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

// DeleteUser removes a user account and cleans up associated data
func (s *Service) DeleteUser(username string) error {
	if !s.enabled {
		return fmt.Errorf("authentication is disabled")
	}

	// Delete user from store
	if err := s.userStore.DeleteUser(username); err != nil {
		return fmt.Errorf("failed to delete user from store: %w", err)
	}

	// Delete user folder if enabled
	if err := s.userFolderManager.DeleteUserFolder(username); err != nil {
		return fmt.Errorf("failed to delete user folder: %w", err)
	}

	// Call cleanup callback if set (for database cleanup)
	if s.cleanupCallback != nil {
		if err := s.cleanupCallback(username); err != nil {
			return fmt.Errorf("failed to clean up user data: %w", err)
		}
	}

	return nil
}

// SetCleanupCallback sets the callback function for cleaning up user data
func (s *Service) SetCleanupCallback(callback func(username string) error) {
	s.cleanupCallback = callback
}

// StartUserWatcher starts watching the users.toml file for changes
func (s *Service) StartUserWatcher() error {
	if !s.enabled {
		return nil
	}

	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		return fmt.Errorf("failed to create users file watcher: %w", err)
	}

	s.usersWatcher = watcher

	// Start watching in a goroutine
	go s.watchUsersFile()

	// Add the users file to the watcher
	err = s.usersWatcher.Add(s.config.UsersFilePath)
	if err != nil {
		return fmt.Errorf("failed to add users file to watcher: %w", err)
	}

	s.logger.WithField("users_file", s.config.UsersFilePath).Info("Started watching users file for changes")
	return nil
}

// StopUserWatcher stops watching the users.toml file
func (s *Service) StopUserWatcher() {
	if s.usersWatcher != nil {
		s.usersWatcher.Close()
		s.usersWatcher = nil
	}
}

// watchUsersFile monitors the users file for changes and handles user deletion
func (s *Service) watchUsersFile() {
	if s.usersWatcher == nil {
		return
	}

	defer s.usersWatcher.Close()

	for {
		select {
		case event, ok := <-s.usersWatcher.Events:
			if !ok {
				return
			}

			// Only handle write events (file modifications)
			if event.Has(fsnotify.Write) {
				s.handleUsersFileChange()
			}

		case err, ok := <-s.usersWatcher.Errors:
			if !ok {
				return
			}
			s.logger.WithError(err).Error("Users file watcher error")
		}
	}
}

// handleUsersFileChange processes changes to the users file and detects deleted users
func (s *Service) handleUsersFileChange() {
	// Store current users before reload
	previousUsers := make(map[string]bool)
	for _, username := range s.userStore.GetAllUsers() {
		previousUsers[username] = true
	}

	// Reload users from file
	newUserStore, err := NewUserStore(s.config.UsersFilePath)
	if err != nil {
		s.logger.WithError(err).Error("Failed to reload users after file change")
		return
	}

	// Update the user store
	s.userStore = newUserStore

	// Get current users
	currentUsers := make(map[string]bool)
	for _, username := range s.userStore.GetAllUsers() {
		currentUsers[username] = true
	}

	// Find deleted users
	for username := range previousUsers {
		if !currentUsers[username] {
			s.logger.WithField("username", username).Info("User deleted from users file, cleaning up data")
			s.handleUserDeletion(username)
		}
	}
}

// handleUserDeletion cleans up all data for a deleted user
func (s *Service) handleUserDeletion(username string) {
	// Delete user folder
	if err := s.userFolderManager.DeleteUserFolder(username); err != nil {
		s.logger.WithError(err).WithField("username", username).Error("Failed to delete user folder")
	} else {
		s.logger.WithField("username", username).Info("Deleted user folder")
	}

	// Call cleanup callback if set (for database cleanup)
	if s.cleanupCallback != nil {
		if err := s.cleanupCallback(username); err != nil {
			s.logger.WithError(err).WithField("username", username).Error("Failed to clean up user data")
		} else {
			s.logger.WithField("username", username).Info("Cleaned up user database entries")
		}
	}

	// Invalidate all sessions for the deleted user
	s.sessionManager.DeleteUserSessions(username)
	s.logger.WithField("username", username).Info("Invalidated all sessions for deleted user")
}

// GetUserFolderManager returns the user folder manager
func (s *Service) GetUserFolderManager() *UserFolderManager {
	return s.userFolderManager
}
