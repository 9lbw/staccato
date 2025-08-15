package tests

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"staccato/internal/auth"
	"staccato/internal/config"
)

func TestUserStore(t *testing.T) {
	testDir := t.TempDir()
	usersFile := filepath.Join(testDir, "test_users.toml")

	// Create user store
	userStore, err := auth.NewUserStore(usersFile)
	if err != nil {
		t.Fatalf("Failed to create user store: %v", err)
	}

	t.Run("RegisterUser", func(t *testing.T) {
		err := userStore.RegisterUser("testuser", "password123")
		if err != nil {
			t.Fatalf("Failed to register user: %v", err)
		}

		// Try to register the same user again (should fail)
		err = userStore.RegisterUser("testuser", "password456")
		if err == nil {
			t.Error("Expected error when registering duplicate user")
		}
	})

	t.Run("Authenticate", func(t *testing.T) {
		// Valid credentials
		if !userStore.Authenticate("testuser", "password123") {
			t.Error("Expected authentication to succeed with valid credentials")
		}

		// Invalid password
		if userStore.Authenticate("testuser", "wrongpassword") {
			t.Error("Expected authentication to fail with invalid password")
		}

		// Non-existent user
		if userStore.Authenticate("nonexistent", "password123") {
			t.Error("Expected authentication to fail for non-existent user")
		}
	})

	t.Run("GetUser", func(t *testing.T) {
		user := userStore.GetUser("testuser")
		if user == nil {
			t.Fatal("Expected to get user")
		}

		if user.Username != "testuser" {
			t.Errorf("Expected username testuser, got %s", user.Username)
		}

		if user.Password != "" {
			t.Error("Expected password to be empty in returned user")
		}

		// Non-existent user
		user = userStore.GetUser("nonexistent")
		if user != nil {
			t.Error("Expected nil for non-existent user")
		}
	})

	t.Run("GetAllUsers", func(t *testing.T) {
		users := userStore.GetAllUsers()
		if len(users) < 2 { // admin + testuser
			t.Errorf("Expected at least 2 users, got %d", len(users))
		}

		// Check if testuser is in the list
		found := false
		for _, username := range users {
			if username == "testuser" {
				found = true
				break
			}
		}
		if !found {
			t.Error("Expected testuser to be in users list")
		}
	})

	t.Run("DeleteUser", func(t *testing.T) {
		// Register another user for deletion
		err := userStore.RegisterUser("deleteuser", "password123")
		if err != nil {
			t.Fatalf("Failed to register user for deletion: %v", err)
		}

		// Verify user exists
		user := userStore.GetUser("deleteuser")
		if user == nil {
			t.Fatal("Expected deleteuser to exist")
		}

		// Delete user
		err = userStore.DeleteUser("deleteuser")
		if err != nil {
			t.Fatalf("Failed to delete user: %v", err)
		}

		// Verify user is deleted
		user = userStore.GetUser("deleteuser")
		if user != nil {
			t.Error("Expected deleteuser to be deleted")
		}

		// Try to delete non-existent user
		err = userStore.DeleteUser("nonexistent")
		if err == nil {
			t.Error("Expected error when deleting non-existent user")
		}
	})
}

func TestSessionManager(t *testing.T) {
	sessionManager := auth.NewSessionManager(24*time.Hour, false)

	t.Run("CreateSession", func(t *testing.T) {
		session, err := sessionManager.CreateSession("testuser")
		if err != nil {
			t.Fatalf("Failed to create session: %v", err)
		}

		if session.Username != "testuser" {
			t.Errorf("Expected username testuser, got %s", session.Username)
		}

		if session.ID == "" {
			t.Error("Expected non-empty session ID")
		}

		if session.ExpiresAt.Before(time.Now()) {
			t.Error("Expected session to not be expired")
		}
	})

	t.Run("GetSession", func(t *testing.T) {
		// Create a session
		session, err := sessionManager.CreateSession("testuser")
		if err != nil {
			t.Fatalf("Failed to create session: %v", err)
		}

		// Get the session
		retrievedSession, valid := sessionManager.GetSession(session.ID)
		if !valid {
			t.Error("Expected session to be valid")
		}

		if retrievedSession.ID != session.ID {
			t.Errorf("Expected session ID %s, got %s", session.ID, retrievedSession.ID)
		}

		// Try to get non-existent session
		_, valid = sessionManager.GetSession("nonexistent")
		if valid {
			t.Error("Expected non-existent session to be invalid")
		}
	})

	t.Run("DeleteSession", func(t *testing.T) {
		// Create a session
		session, err := sessionManager.CreateSession("testuser")
		if err != nil {
			t.Fatalf("Failed to create session: %v", err)
		}

		// Delete the session
		sessionManager.DeleteSession(session.ID)

		// Verify session is deleted
		_, valid := sessionManager.GetSession(session.ID)
		if valid {
			t.Error("Expected session to be deleted")
		}
	})

	t.Run("DeleteUserSessions", func(t *testing.T) {
		// Create multiple sessions for the same user
		session1, _ := sessionManager.CreateSession("multiuser")
		session2, _ := sessionManager.CreateSession("multiuser")
		session3, _ := sessionManager.CreateSession("otheruser")

		// Delete all sessions for multiuser
		sessionManager.DeleteUserSessions("multiuser")

		// Verify multiuser sessions are deleted
		_, valid1 := sessionManager.GetSession(session1.ID)
		_, valid2 := sessionManager.GetSession(session2.ID)
		if valid1 || valid2 {
			t.Error("Expected multiuser sessions to be deleted")
		}

		// Verify otheruser session still exists
		_, valid3 := sessionManager.GetSession(session3.ID)
		if !valid3 {
			t.Error("Expected otheruser session to still exist")
		}
	})

	t.Run("RefreshSession", func(t *testing.T) {
		// Create a session
		session, err := sessionManager.CreateSession("testuser")
		if err != nil {
			t.Fatalf("Failed to create session: %v", err)
		}

		originalExpiry := session.ExpiresAt

		// Wait a bit to ensure time difference
		time.Sleep(10 * time.Millisecond)

		// Refresh the session
		refreshed := sessionManager.RefreshSession(session.ID)
		if !refreshed {
			t.Error("Expected session to be refreshed")
		}

		// Get the session and check if expiry was updated
		refreshedSession, valid := sessionManager.GetSession(session.ID)
		if !valid {
			t.Error("Expected refreshed session to be valid")
		}

		if !refreshedSession.ExpiresAt.After(originalExpiry) {
			t.Error("Expected session expiry to be updated")
		}

		// Try to refresh non-existent session
		refreshed = sessionManager.RefreshSession("nonexistent")
		if refreshed {
			t.Error("Expected refresh of non-existent session to fail")
		}
	})
}

func TestAuthService(t *testing.T) {
	testDir := t.TempDir()

	cfg := &config.AuthConfig{
		Enabled:           true,
		UsersFilePath:     filepath.Join(testDir, "test_users.toml"),
		SessionDuration:   "1h",
		SecureCookies:     false,
		AllowRegistration: true,
		UserFolders:       true,
		UserMusicPath:     filepath.Join(testDir, "users"),
	}

	authService, err := auth.NewService(cfg)
	if err != nil {
		t.Fatalf("Failed to create auth service: %v", err)
	}

	t.Run("IsEnabled", func(t *testing.T) {
		if !authService.IsEnabled() {
			t.Error("Expected auth service to be enabled")
		}
	})

	t.Run("IsRegistrationAllowed", func(t *testing.T) {
		if !authService.IsRegistrationAllowed() {
			t.Error("Expected registration to be allowed")
		}
	})

	t.Run("Register", func(t *testing.T) {
		err := authService.Register("testuser", "password123")
		if err != nil {
			t.Fatalf("Failed to register user: %v", err)
		}

		// Check if user folder was created
		userPath := filepath.Join(testDir, "users", "testuser")
		if _, err := os.Stat(userPath); os.IsNotExist(err) {
			t.Error("Expected user folder to be created")
		}
	})

	t.Run("Login", func(t *testing.T) {
		session, err := authService.Login("testuser", "password123")
		if err != nil {
			t.Fatalf("Failed to login: %v", err)
		}

		if session.Username != "testuser" {
			t.Errorf("Expected username testuser, got %s", session.Username)
		}

		// Invalid credentials
		_, err = authService.Login("testuser", "wrongpassword")
		if err == nil {
			t.Error("Expected login to fail with wrong password")
		}
	})

	t.Run("ValidateSession", func(t *testing.T) {
		// Login to get a session
		session, err := authService.Login("testuser", "password123")
		if err != nil {
			t.Fatalf("Failed to login: %v", err)
		}

		// Validate the session
		validatedSession, valid := authService.ValidateSession(session.ID)
		if !valid {
			t.Error("Expected session to be valid")
		}

		if validatedSession.ID != session.ID {
			t.Errorf("Expected session ID %s, got %s", session.ID, validatedSession.ID)
		}
	})

	t.Run("Logout", func(t *testing.T) {
		// Login to get a session
		session, err := authService.Login("testuser", "password123")
		if err != nil {
			t.Fatalf("Failed to login: %v", err)
		}

		// Logout
		authService.Logout(session.ID)

		// Verify session is invalid
		_, valid := authService.ValidateSession(session.ID)
		if valid {
			t.Error("Expected session to be invalid after logout")
		}
	})

	t.Run("DeleteUser", func(t *testing.T) {
		// Register a user for deletion
		err := authService.Register("deleteuser", "password123")
		if err != nil {
			t.Fatalf("Failed to register user for deletion: %v", err)
		}

		userPath := filepath.Join(testDir, "users", "deleteuser")

		// Verify user folder exists
		if _, err := os.Stat(userPath); os.IsNotExist(err) {
			t.Fatal("Expected user folder to exist before deletion")
		}

		// Delete user
		err = authService.DeleteUser("deleteuser")
		if err != nil {
			t.Fatalf("Failed to delete user: %v", err)
		}

		// Verify user folder is deleted
		if _, err := os.Stat(userPath); !os.IsNotExist(err) {
			t.Error("Expected user folder to be deleted")
		}
	})
}

func TestDisabledAuth(t *testing.T) {
	cfg := &config.AuthConfig{
		Enabled: false,
	}

	authService, err := auth.NewService(cfg)
	if err != nil {
		t.Fatalf("Failed to create disabled auth service: %v", err)
	}

	t.Run("IsEnabled", func(t *testing.T) {
		if authService.IsEnabled() {
			t.Error("Expected auth service to be disabled")
		}
	})

	t.Run("ValidateSession", func(t *testing.T) {
		// When auth is disabled, all sessions should be considered valid
		_, valid := authService.ValidateSession("any-session-id")
		if !valid {
			t.Error("Expected all sessions to be valid when auth is disabled")
		}
	})

	t.Run("Login", func(t *testing.T) {
		_, err := authService.Login("user", "password")
		if err == nil {
			t.Error("Expected login to fail when auth is disabled")
		}
	})
}
