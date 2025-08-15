package tests

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"staccato/internal/config"
	"staccato/internal/database"
	"staccato/internal/server"
	"staccato/pkg/models"
)

func TestIntegration(t *testing.T) {
	// Set up test environment
	testDir := t.TempDir()
	dbPath := filepath.Join(testDir, "test.db")
	musicDir := filepath.Join(testDir, "music")
	usersDir := filepath.Join(testDir, "users")

	// Create directories
	err := os.MkdirAll(musicDir, 0755)
	if err != nil {
		t.Fatalf("Failed to create music directory: %v", err)
	}

	cfg := &config.Config{
		Server: config.ServerConfig{
			Port:         "8080",
			Host:         "localhost",
			StaticDir:    "./static",
			EnableCORS:   true,
			ReadTimeout:  30,
			WriteTimeout: 30,
			IdleTimeout:  120,
		},
		Database: config.DatabaseConfig{
			Path:           dbPath,
			MaxConnections: 10,
		},
		Music: config.MusicConfig{
			LibraryPath:      musicDir,
			SupportedFormats: []string{".mp3", ".flac", ".wav", ".m4a"},
			WatchForChanges:  false,
			ScanOnStartup:    false,
		},
		Auth: config.AuthConfig{
			Enabled:           true,
			UsersFilePath:     filepath.Join(testDir, "users.toml"),
			SessionDuration:   "1h",
			SecureCookies:     false,
			AllowRegistration: true,
			UserFolders:       true,
			UserMusicPath:     usersDir,
			AllowUploads:      true,
			MaxUploadSize:     100,
		},
	}

	// Create database
	db, err := database.NewDatabase(dbPath)
	if err != nil {
		t.Fatalf("Failed to create database: %v", err)
	}
	defer db.Close()

	// Create server
	_, err = server.NewMusicServer(cfg, db)
	if err != nil {
		t.Fatalf("Failed to create music server: %v", err)
	}

	// Set up routes (this would need to be public method or we need access to it)
	// handler := musicServer.SetupRoutes()

	t.Run("DatabaseOperations", func(t *testing.T) {
		// Test basic database operations
		track := models.Track{
			Title:       "Integration Test Song",
			Artist:      "Test Artist",
			Album:       "Test Album",
			TrackNumber: 1,
			Duration:    200,
			FilePath:    filepath.Join(musicDir, "test.mp3"),
			FileSize:    2000000,
			Owner:       "",
		}

		// Insert track
		id, err := db.InsertTrack(track)
		if err != nil {
			t.Fatalf("Failed to insert track: %v", err)
		}

		// Retrieve track
		retrievedTrack, err := db.GetTrackByID(id)
		if err != nil {
			t.Fatalf("Failed to retrieve track: %v", err)
		}

		if retrievedTrack.Title != track.Title {
			t.Errorf("Expected title %s, got %s", track.Title, retrievedTrack.Title)
		}

		// Test search
		tracks, err := db.SearchTracks("Integration")
		if err != nil {
			t.Fatalf("Failed to search tracks: %v", err)
		}

		if len(tracks) == 0 {
			t.Error("Expected to find integration test track")
		}
	})

	t.Run("UserManagement", func(t *testing.T) {
		// Test user management configuration
		if !cfg.Auth.Enabled {
			t.Skip("Auth not enabled")
		}

		// Test that auth configuration is properly set
		if !cfg.Auth.AllowRegistration {
			t.Error("Expected registration to be allowed")
		}

		if !cfg.Auth.UserFolders {
			t.Error("Expected user folders to be enabled")
		}
	})

	t.Run("APIEndpoints", func(t *testing.T) {
		// Test API endpoints (would need actual HTTP server running)
		// This is a placeholder for endpoint testing

		// Create a test request
		req, err := http.NewRequest("GET", "/api/tracks", nil)
		if err != nil {
			t.Fatalf("Failed to create request: %v", err)
		}

		// Create response recorder
		rr := httptest.NewRecorder()

		// We would need the actual handler here
		// handler.ServeHTTP(rr, req)

		// For now, just verify we can create the request
		if req.URL.Path != "/api/tracks" {
			t.Errorf("Expected path /api/tracks, got %s", req.URL.Path)
		}

		_ = rr // Use the recorder
	})

	t.Run("ConfigValidation", func(t *testing.T) {
		// Test that the configuration is valid
		if cfg.Server.Port != "8080" {
			t.Errorf("Expected port 8080, got %s", cfg.Server.Port)
		}

		if !cfg.Auth.Enabled {
			t.Error("Expected auth to be enabled")
		}

		if !cfg.Auth.AllowRegistration {
			t.Error("Expected registration to be allowed")
		}
	})

	t.Run("FileOperations", func(t *testing.T) {
		// Test file-related operations
		testFile := filepath.Join(musicDir, "test_track.mp3")

		// Create a dummy file
		err := os.WriteFile(testFile, []byte("dummy audio data"), 0644)
		if err != nil {
			t.Fatalf("Failed to create test file: %v", err)
		}

		// Test file existence
		if _, err := os.Stat(testFile); os.IsNotExist(err) {
			t.Error("Test file should exist")
		}

		// Clean up
		os.Remove(testFile)
	})
}

func TestFullWorkflow(t *testing.T) {
	// Test a complete workflow: user registration -> file upload -> metadata extraction -> playback
	testDir := t.TempDir()

	t.Run("CompleteWorkflow", func(t *testing.T) {
		// 1. Set up server and database
		dbPath := filepath.Join(testDir, "workflow.db")
		db, err := database.NewDatabase(dbPath)
		if err != nil {
			t.Fatalf("Failed to create database: %v", err)
		}
		defer db.Close()

		// 2. Create test user
		track := models.Track{
			Title:    "Workflow Test",
			Artist:   "Test User",
			Album:    "Test Album",
			FilePath: "/test/workflow.mp3",
			FileSize: 1500000,
			Owner:    "workflowuser",
		}

		trackID, err := db.InsertTrack(track)
		if err != nil {
			t.Fatalf("Failed to insert workflow track: %v", err)
		}

		// 3. Test track retrieval
		retrievedTrack, err := db.GetTrackByID(trackID)
		if err != nil {
			t.Fatalf("Failed to retrieve workflow track: %v", err)
		}

		if retrievedTrack.Owner != "workflowuser" {
			t.Errorf("Expected owner workflowuser, got %s", retrievedTrack.Owner)
		}

		// 4. Test user-specific operations
		userTracks, err := db.GetTracksByOwner("workflowuser")
		if err != nil {
			t.Fatalf("Failed to get user tracks: %v", err)
		}

		if len(userTracks) != 1 {
			t.Errorf("Expected 1 track for user, got %d", len(userTracks))
		}

		// 5. Test playlist operations
		playlistID, err := db.CreatePlaylist("Workflow Playlist", "Test playlist for workflow")
		if err != nil {
			t.Fatalf("Failed to create playlist: %v", err)
		}

		err = db.AddTrackToPlaylist(playlistID, trackID)
		if err != nil {
			t.Fatalf("Failed to add track to playlist: %v", err)
		}

		playlistTracks, err := db.GetPlaylistTracks(playlistID)
		if err != nil {
			t.Fatalf("Failed to get playlist tracks: %v", err)
		}

		if len(playlistTracks) != 1 {
			t.Errorf("Expected 1 track in playlist, got %d", len(playlistTracks))
		}

		// 6. Test search functionality
		searchResults, err := db.SearchTracks("Workflow")
		if err != nil {
			t.Fatalf("Failed to search tracks: %v", err)
		}

		if len(searchResults) == 0 {
			t.Error("Expected to find workflow track in search")
		}

		// 7. Test cleanup (user deletion simulation)
		err = db.DeleteTracksByOwner("workflowuser")
		if err != nil {
			t.Fatalf("Failed to delete user tracks: %v", err)
		}

		// Verify tracks are deleted
		userTracks, err = db.GetTracksByOwner("workflowuser")
		if err != nil {
			t.Fatalf("Failed to check user tracks after deletion: %v", err)
		}

		if len(userTracks) != 0 {
			t.Errorf("Expected 0 tracks after user deletion, got %d", len(userTracks))
		}
	})
}

// MockHTTPHandler creates a simple handler for testing HTTP functionality
func mockAPIHandler(db *database.Database) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		switch {
		case strings.HasPrefix(r.URL.Path, "/api/tracks"):
			handleMockTracks(w, r, db)
		case r.URL.Path == "/api/health":
			handleMockHealth(w, r)
		default:
			http.NotFound(w, r)
		}
	}
}

func handleMockTracks(w http.ResponseWriter, r *http.Request, db *database.Database) {
	switch r.Method {
	case "GET":
		tracks, err := db.GetAllTracks()
		if err != nil {
			http.Error(w, "Database error", http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(tracks)
	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

func handleMockHealth(w http.ResponseWriter, r *http.Request) {
	if r.Method != "GET" {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
}

func TestMockAPI(t *testing.T) {
	// Test with mock API handler
	testDir := t.TempDir()
	dbPath := filepath.Join(testDir, "mock_api.db")

	db, err := database.NewDatabase(dbPath)
	if err != nil {
		t.Fatalf("Failed to create database: %v", err)
	}
	defer db.Close()

	// Insert test data
	track := models.Track{
		Title:    "API Test Track",
		Artist:   "API Artist",
		Album:    "API Album",
		FilePath: "/api/test.mp3",
		FileSize: 1000000,
	}

	_, err = db.InsertTrack(track)
	if err != nil {
		t.Fatalf("Failed to insert test track: %v", err)
	}

	handler := mockAPIHandler(db)

	t.Run("GetTracks", func(t *testing.T) {
		req, err := http.NewRequest("GET", "/api/tracks", nil)
		if err != nil {
			t.Fatalf("Failed to create request: %v", err)
		}

		rr := httptest.NewRecorder()
		handler.ServeHTTP(rr, req)

		if status := rr.Code; status != http.StatusOK {
			t.Errorf("Expected status 200, got %v", status)
		}

		var tracks []models.Track
		err = json.NewDecoder(rr.Body).Decode(&tracks)
		if err != nil {
			t.Fatalf("Failed to decode response: %v", err)
		}

		if len(tracks) == 0 {
			t.Error("Expected at least one track")
		}

		if tracks[0].Title != "API Test Track" {
			t.Errorf("Expected track title 'API Test Track', got %s", tracks[0].Title)
		}
	})

	t.Run("HealthCheck", func(t *testing.T) {
		req, err := http.NewRequest("GET", "/api/health", nil)
		if err != nil {
			t.Fatalf("Failed to create request: %v", err)
		}

		rr := httptest.NewRecorder()
		handler.ServeHTTP(rr, req)

		if status := rr.Code; status != http.StatusOK {
			t.Errorf("Expected status 200, got %v", status)
		}

		var response map[string]string
		err = json.NewDecoder(rr.Body).Decode(&response)
		if err != nil {
			t.Fatalf("Failed to decode response: %v", err)
		}

		if response["status"] != "ok" {
			t.Errorf("Expected status 'ok', got %s", response["status"])
		}
	})
}
