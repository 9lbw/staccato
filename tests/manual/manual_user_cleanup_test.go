package main

import (
	"fmt"
	"log"
	"os"
	"path/filepath"

	"staccato/internal/auth"
	"staccato/internal/config"
	"staccato/internal/database"
	"staccato/pkg/models"
)

func main() {
	// Set up test environment
	testDir := "./test_cleanup"
	os.RemoveAll(testDir) // Clean up any previous test
	os.MkdirAll(testDir, 0755)
	defer os.RemoveAll(testDir) // Clean up after test

	// Create test config
	cfg := &config.AuthConfig{
		Enabled:           true,
		UsersFilePath:     filepath.Join(testDir, "users.toml"),
		SessionDuration:   "24h",
		SecureCookies:     false,
		AllowRegistration: true,
		UserFolders:       true,
		UserMusicPath:     filepath.Join(testDir, "users"),
	}

	// Create test database
	dbPath := filepath.Join(testDir, "test.db")
	db, err := database.NewDatabase(dbPath)
	if err != nil {
		log.Fatalf("Failed to create database: %v", err)
	}
	defer db.Close()

	// Create auth service
	authService, err := auth.NewService(cfg)
	if err != nil {
		log.Fatalf("Failed to create auth service: %v", err)
	}

	// Set up cleanup callback
	authService.SetCleanupCallback(func(username string) error {
		fmt.Printf("Cleaning up database entries for user: %s\n", username)
		return db.DeleteTracksByOwner(username)
	})

	// Register a test user
	testUser := "testcleanup"
	testPassword := "password123"

	fmt.Printf("Registering user: %s\n", testUser)
	err = authService.Register(testUser, testPassword)
	if err != nil {
		log.Fatalf("Failed to register user: %v", err)
	}

	// Verify user folder was created
	userPath := filepath.Join(testDir, "users", testUser)
	if _, err := os.Stat(userPath); os.IsNotExist(err) {
		log.Fatalf("User folder was not created: %s", userPath)
	}
	fmt.Printf("User folder created: %s\n", userPath)

	// Add some test tracks for the user
	testTrack := models.Track{
		Title:       "Test Song",
		Artist:      "Test Artist",
		Album:       "Test Album",
		TrackNumber: 1,
		Duration:    180,
		FilePath:    filepath.Join(userPath, "test.mp3"),
		FileSize:    1024000,
		Owner:       testUser,
	}

	trackID, err := db.InsertTrack(testTrack)
	if err != nil {
		log.Fatalf("Failed to insert test track: %v", err)
	}
	fmt.Printf("Inserted test track with ID: %d\n", trackID)

	// Verify tracks exist for user
	tracks, err := db.GetTracksByOwner(testUser)
	if err != nil {
		log.Fatalf("Failed to get tracks for user: %v", err)
	}
	fmt.Printf("Found %d tracks for user %s\n", len(tracks), testUser)

	// Simulate user deletion by calling DeleteUser directly
	fmt.Printf("Deleting user: %s\n", testUser)
	err = authService.DeleteUser(testUser)
	if err != nil {
		log.Fatalf("Failed to delete user: %v", err)
	}

	// Verify user folder was deleted
	if _, err := os.Stat(userPath); !os.IsNotExist(err) {
		log.Fatalf("User folder was not deleted: %s", userPath)
	}
	fmt.Printf("User folder deleted: %s\n", userPath)

	// Verify tracks were deleted from database
	tracks, err = db.GetTracksByOwner(testUser)
	if err != nil {
		log.Fatalf("Failed to get tracks for user after deletion: %v", err)
	}
	fmt.Printf("Found %d tracks for deleted user %s (should be 0)\n", len(tracks), testUser)

	if len(tracks) == 0 {
		fmt.Println("✓ User cleanup test passed! All data was properly cleaned up.")
	} else {
		fmt.Printf("✗ User cleanup test failed! %d tracks still exist for deleted user.\n", len(tracks))
	}
}
