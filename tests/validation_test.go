package tests

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"staccato/internal/config"
	"staccato/internal/database"
	"staccato/internal/server"
)

func TestValidation(t *testing.T) {
	// Create test database and server for validation tests
	testDir := t.TempDir()
	dbPath := testDir + "/test.db"

	db, err := database.NewDatabase(dbPath)
	if err != nil {
		t.Fatalf("Failed to create test database: %v", err)
	}
	defer db.Close()

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
		Music: config.MusicConfig{
			LibraryPath:      testDir + "/music",
			SupportedFormats: []string{".mp3", ".flac", ".wav", ".m4a"},
			WatchForChanges:  false,
			ScanOnStartup:    false,
		},
		Auth: config.AuthConfig{
			Enabled: false, // Disable auth for validation tests
		},
	}

	_, err = server.NewMusicServer(cfg, db)
	if err != nil {
		t.Fatalf("Failed to create music server: %v", err)
	}

	t.Run("ValidateTrackID", func(t *testing.T) {
		testCases := []struct {
			trackID    string
			shouldPass bool
		}{
			{"1", true},
			{"123", true},
			{"0", false},                     // Track ID should be positive
			{"-1", false},                    // Negative numbers
			{"abc", false},                   // Non-numeric
			{"", false},                      // Empty
			{"1.5", false},                   // Decimal
			{"999999999999999999999", false}, // Too large
		}

		for _, tc := range testCases {
			req, _ := http.NewRequest("GET", "/stream/"+tc.trackID, nil)
			rr := httptest.NewRecorder()

			// Use a test handler that includes validation
			handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				// This would normally be handled by the router, but we'll simulate it
				// by checking if the track ID parsing would succeed
				w.WriteHeader(http.StatusOK)
			})

			handler.ServeHTTP(rr, req)

			// For actual validation testing, we'd need to examine the server's internal
			// validation logic more directly. This is a placeholder for that.
		}
	})

	t.Run("SearchQueryValidation", func(t *testing.T) {
		testCases := []struct {
			query      string
			shouldPass bool
		}{
			{"test", true},
			{"artist name", true},
			{"song-title", true},
			{"", true},  // Empty search should be allowed
			{"a", true}, // Single character
			{"very long search query that might be used for testing purposes", true},
			// Add more test cases based on actual validation rules
		}

		for _, tc := range testCases {
			req, _ := http.NewRequest("GET", "/api/tracks?search="+tc.query, nil)
			rr := httptest.NewRecorder()

			// Create a test handler
			handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusOK)
			})

			handler.ServeHTTP(rr, req)
			// Validation testing would need to access the server's validation methods directly
		}
	})

	t.Run("InputSanitization", func(t *testing.T) {
		// Test cases for input sanitization
		testCases := []struct {
			input    string
			expected string
		}{
			{"normal text", "normal text"},
			{"text with spaces", "text with spaces"},
			{"", ""},
		}

		for _, tc := range testCases {
			// This would test the sanitizeInput function from the server
			// We would need to make it accessible for testing
			_ = tc.input
			_ = tc.expected
			// result := server.SanitizeInput(tc.input)
			// if result != tc.expected {
			//     t.Errorf("sanitizeInput(%q): expected %q, got %q", tc.input, tc.expected, result)
			// }
		}
	})
}

func TestHTTPHelpers(t *testing.T) {
	t.Run("ResponseWriting", func(t *testing.T) {
		// Test response writing functionality
		rr := httptest.NewRecorder()

		// Test writing JSON response
		testData := map[string]string{"test": "data"}

		// This would test the server's JSON response methods
		// We need access to the server's response methods for proper testing
		_ = rr
		_ = testData
	})

	t.Run("ErrorHandling", func(t *testing.T) {
		// Test error response handling
		rr := httptest.NewRecorder()
		req, _ := http.NewRequest("GET", "/nonexistent", nil)

		// Test 404 handling
		_ = rr
		_ = req
	})
}

// TestCORSHeaders tests CORS header handling
func TestCORSHeaders(t *testing.T) {
	testDir := t.TempDir()
	dbPath := testDir + "/test.db"

	db, err := database.NewDatabase(dbPath)
	if err != nil {
		t.Fatalf("Failed to create test database: %v", err)
	}
	defer db.Close()

	cfg := &config.Config{
		Server: config.ServerConfig{
			EnableCORS: true,
		},
		Auth: config.AuthConfig{
			Enabled: false,
		},
	}

	musicServer, err := server.NewMusicServer(cfg, db)
	if err != nil {
		t.Fatalf("Failed to create music server: %v", err)
	}

	_ = musicServer

	t.Run("CORSEnabled", func(t *testing.T) {
		// Test that CORS headers are set when enabled
		req, _ := http.NewRequest("OPTIONS", "/api/tracks", nil)
		req.Header.Set("Origin", "http://localhost:3000")
		req.Header.Set("Access-Control-Request-Method", "GET")

		rr := httptest.NewRecorder()

		// This would need the actual CORS middleware from the server
		// handler := musicServer.SetupRoutes()
		// handler.ServeHTTP(rr, req)

		// Check for CORS headers
		// if rr.Header().Get("Access-Control-Allow-Origin") == "" {
		//     t.Error("Expected Access-Control-Allow-Origin header")
		// }

		_ = rr
	})
}
