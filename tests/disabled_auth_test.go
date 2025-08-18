package tests

import (
	"testing"

	"staccato/internal/auth"
	"staccato/internal/config"
)

func TestDisabledAuthNilPointer(t *testing.T) {
	t.Run("DisabledAuthServiceCreation", func(t *testing.T) {
		// Create auth config with auth disabled
		authConfig := &config.AuthConfig{
			Enabled:         false,
			UsersFilePath:   "./users.toml",
			SessionDuration: "1h",
			UserFolders:     false,
			UserMusicPath:   "./users",
		}

		// Create auth service - this should not fail
		authService, err := auth.NewService(authConfig)
		if err != nil {
			t.Fatalf("Failed to create disabled auth service: %v", err)
		}

		// Verify service is disabled
		if authService.IsEnabled() {
			t.Error("Expected auth service to be disabled")
		}

		// Verify GetUserFolderManager doesn't cause nil pointer dereference
		folderManager := authService.GetUserFolderManager()
		if folderManager == nil {
			t.Error("Expected UserFolderManager to be initialized even when auth is disabled")
		}

		// Verify folder manager is disabled
		if folderManager.IsEnabled() {
			t.Error("Expected UserFolderManager to be disabled when auth is disabled")
		}
	})

	t.Run("DisabledAuthOperations", func(t *testing.T) {
		// Create disabled auth service
		authConfig := &config.AuthConfig{
			Enabled:         false,
			UsersFilePath:   "./users.toml",
			SessionDuration: "1h",
			UserFolders:     false,
			UserMusicPath:   "./users",
		}

		authService, err := auth.NewService(authConfig)
		if err != nil {
			t.Fatalf("Failed to create disabled auth service: %v", err)
		}

		// Test that operations on disabled auth service behave correctly

		// Login should fail
		session, err := authService.Login("testuser", "password")
		if err == nil {
			t.Error("Expected login to fail when auth is disabled")
		}
		if session != nil {
			t.Error("Expected no session when auth is disabled")
		}

		// ValidateSession should return true when auth is disabled (allow all)
		_, valid := authService.ValidateSession("fake-session-id")
		if !valid {
			t.Error("Expected session validation to succeed when auth is disabled (allow all)")
		}

		// Registration should be allowed based on config
		if authService.IsRegistrationAllowed() {
			t.Error("Expected registration to be disallowed when auth is disabled")
		}
	})
}
