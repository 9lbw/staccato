package tests

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"staccato/internal/auth"
	"staccato/internal/config"
)

func TestUserFileWatcher(t *testing.T) {
	// Create temporary directory for test
	testDir := t.TempDir()
	usersFile := filepath.Join(testDir, "users.toml")
	userMusicPath := filepath.Join(testDir, "users")

	// Create initial users file with multiple users
	initialUsers := `# Test users file

[[users]]
  username = "user1"
  password = "password1"
  role = "user"
  created = "2025-08-15 00:00:00"

[[users]]
  username = "user2"
  password = "password2"
  role = "user"
  created = "2025-08-15 00:00:00"

[[users]]
  username = "user3"
  password = "password3"
  role = "user"
  created = "2025-08-15 00:00:00"
`

	err := os.WriteFile(usersFile, []byte(initialUsers), 0644)
	if err != nil {
		t.Fatalf("Failed to create initial users file: %v", err)
	}

	// Create auth config
	authConfig := &config.AuthConfig{
		Enabled:         true,
		UsersFilePath:   usersFile,
		SessionDuration: "1h",
		UserFolders:     true,
		UserMusicPath:   userMusicPath,
	}

	// Create auth service
	authService, err := auth.NewService(authConfig)
	if err != nil {
		t.Fatalf("Failed to create auth service: %v", err)
	}

	// Track cleanup calls
	cleanupCalls := make([]string, 0)
	authService.SetCleanupCallback(func(username string) error {
		cleanupCalls = append(cleanupCalls, username)
		return nil
	})

	// Start the user watcher
	err = authService.StartUserWatcher()
	if err != nil {
		t.Fatalf("Failed to start user watcher: %v", err)
	}
	defer authService.StopUserWatcher()

	// Wait a moment for watcher to start
	time.Sleep(100 * time.Millisecond)

	// Verify initial users are loaded
	users := authService.GetUserStore().GetAllUsers()
	if len(users) != 3 {
		t.Fatalf("Expected 3 initial users, got %d", len(users))
	}

	t.Run("AddUserDoesNotTriggerMassCleanup", func(t *testing.T) {
		// Reset cleanup calls
		cleanupCalls = cleanupCalls[:0]

		// Add a new user by rewriting the file (simulating registration)
		newUsers := initialUsers + `
[[users]]
  username = "user4"
  password = "password4"
  role = "user"
  created = "2025-08-15 00:00:00"
`

		err := os.WriteFile(usersFile, []byte(newUsers), 0644)
		if err != nil {
			t.Fatalf("Failed to write new users file: %v", err)
		}

		// Wait for file watcher to process the change
		time.Sleep(500 * time.Millisecond)

		// Should not have triggered any cleanup calls
		if len(cleanupCalls) > 0 {
			t.Errorf("Adding user should not trigger cleanup, but got cleanup calls for: %v", cleanupCalls)
		}

		// Verify all users are still there plus the new one
		users := authService.GetUserStore().GetAllUsers()
		if len(users) != 4 {
			t.Errorf("Expected 4 users after adding one, got %d", len(users))
		}

		// Check that user4 was added
		user4 := authService.GetUserStore().GetUser("user4")
		if user4 == nil {
			t.Error("Expected user4 to be added")
		}
	})

	t.Run("RemoveUserTriggersCleanup", func(t *testing.T) {
		// Reset cleanup calls
		cleanupCalls = cleanupCalls[:0]

		// Remove user2 by rewriting the file
		usersWithoutUser2 := `# Test users file

[[users]]
  username = "user1"
  password = "password1"
  role = "user"
  created = "2025-08-15 00:00:00"

[[users]]
  username = "user3"
  password = "password3"
  role = "user"
  created = "2025-08-15 00:00:00"

[[users]]
  username = "user4"
  password = "password4"
  role = "user"
  created = "2025-08-15 00:00:00"
`

		err := os.WriteFile(usersFile, []byte(usersWithoutUser2), 0644)
		if err != nil {
			t.Fatalf("Failed to write updated users file: %v", err)
		}

		// Wait for file watcher to process the change
		time.Sleep(500 * time.Millisecond)

		// Should have triggered cleanup for user2 only
		if len(cleanupCalls) != 1 {
			t.Errorf("Expected 1 cleanup call, got %d: %v", len(cleanupCalls), cleanupCalls)
		}

		if len(cleanupCalls) > 0 && cleanupCalls[0] != "user2" {
			t.Errorf("Expected cleanup call for user2, got %s", cleanupCalls[0])
		}

		// Verify user2 is gone but others remain
		users := authService.GetUserStore().GetAllUsers()
		if len(users) != 3 {
			t.Errorf("Expected 3 users after removing one, got %d", len(users))
		}

		// Check that user2 is gone
		user2 := authService.GetUserStore().GetUser("user2")
		if user2 != nil {
			t.Error("Expected user2 to be removed")
		}

		// Check that other users are still there
		for _, username := range []string{"user1", "user3", "user4"} {
			user := authService.GetUserStore().GetUser(username)
			if user == nil {
				t.Errorf("Expected %s to still exist", username)
			}
		}
	})

	t.Run("FileCorruptionDoesNotTriggerMassCleanup", func(t *testing.T) {
		// Reset cleanup calls
		cleanupCalls = cleanupCalls[:0]

		// Temporarily write invalid TOML to simulate file corruption during write
		err := os.WriteFile(usersFile, []byte("invalid toml content [[["), 0644)
		if err != nil {
			t.Fatalf("Failed to write invalid file: %v", err)
		}

		// Wait a moment
		time.Sleep(200 * time.Millisecond)

		// Write back valid content
		validUsers := `# Test users file

[[users]]
  username = "user1"
  password = "password1"
  role = "user"
  created = "2025-08-15 00:00:00"

[[users]]
  username = "user3"
  password = "password3"
  role = "user"
  created = "2025-08-15 00:00:00"

[[users]]
  username = "user4"
  password = "password4"
  role = "user"
  created = "2025-08-15 00:00:00"
`

		err = os.WriteFile(usersFile, []byte(validUsers), 0644)
		if err != nil {
			t.Fatalf("Failed to write valid file back: %v", err)
		}

		// Wait for processing
		time.Sleep(500 * time.Millisecond)

		// Should not have triggered mass cleanup due to temporary corruption
		if len(cleanupCalls) > 1 {
			t.Errorf("File corruption should not trigger mass cleanup, got cleanup calls for: %v", cleanupCalls)
		}

		// Verify users are still there
		users := authService.GetUserStore().GetAllUsers()
		if len(users) != 3 {
			t.Errorf("Expected 3 users after file corruption recovery, got %d", len(users))
		}
	})
}
