package tests

import (
	"os"
	"path/filepath"
	"testing"

	"staccato/internal/metadata"
)

func TestMetadataExtractor(t *testing.T) {
	supportedFormats := []string{".mp3", ".flac", ".wav", ".m4a"}
	extractor := metadata.NewExtractor(supportedFormats)

	t.Run("IsAudioFile", func(t *testing.T) {
		// Test supported formats
		testCases := []struct {
			filename string
			expected bool
		}{
			{"song.mp3", true},
			{"song.MP3", true},
			{"song.flac", true},
			{"song.FLAC", true},
			{"song.wav", true},
			{"song.m4a", true},
			{"song.txt", false},
			{"song.jpg", false},
			{"song", false},
			{"", false},
		}

		for _, tc := range testCases {
			result := extractor.IsAudioFile(tc.filename)
			if result != tc.expected {
				t.Errorf("IsAudioFile(%s): expected %v, got %v", tc.filename, tc.expected, result)
			}
		}
	})

	t.Run("GetContentType", func(t *testing.T) {
		testCases := []struct {
			filename string
			expected string
		}{
			{"song.mp3", "audio/mpeg"},
			{"song.MP3", "audio/mpeg"},
			{"song.flac", "audio/flac"},
			{"song.FLAC", "audio/flac"},
			{"song.wav", "audio/wav"},
			{"song.WAV", "audio/wav"},
			{"song.m4a", "audio/mp4"},
			{"song.M4A", "audio/mp4"},
			{"song.txt", "application/octet-stream"},
			{"song.unknown", "application/octet-stream"},
		}

		for _, tc := range testCases {
			result := extractor.GetContentType(tc.filename)
			if result != tc.expected {
				t.Errorf("GetContentType(%s): expected %s, got %s", tc.filename, tc.expected, result)
			}
		}
	})

	t.Run("GetAlbumArtMimeType", func(t *testing.T) {
		testCases := []struct {
			name     string
			data     []byte
			expected string
		}{
			{"JPEG", []byte{0xFF, 0xD8, 0xFF, 0xE0}, "image/jpeg"},
			{"PNG", []byte{0x89, 0x50, 0x4E, 0x47}, "image/png"},
			{"GIF", []byte{0x47, 0x49, 0x46, 0x38}, "image/gif"},
			{"Unknown", []byte{0x00, 0x00, 0x00, 0x00}, "application/octet-stream"},
			{"Too short", []byte{0xFF}, "application/octet-stream"},
			{"Empty", []byte{}, "application/octet-stream"},
		}

		for _, tc := range testCases {
			result := extractor.GetAlbumArtMimeType(tc.data)
			if result != tc.expected {
				t.Errorf("GetAlbumArtMimeType(%s): expected %s, got %s", tc.name, tc.expected, result)
			}
		}
	})

	t.Run("AlbumArtCache", func(t *testing.T) {
		// Test album art cache functionality
		testID := "test_art_id"

		// Initially, art should not exist
		_, exists := extractor.GetAlbumArt(testID)
		if exists {
			t.Error("Expected album art to not exist initially")
		}

		// Since we can't easily create real audio files with embedded art in a test,
		// we'll test the cache functionality directly would require more complex setup
		// The ExtractFromFile method is tested in integration tests with real files
	})
}

func TestMetadataExtractionFallback(t *testing.T) {
	supportedFormats := []string{".mp3", ".flac", ".wav", ".m4a"}
	extractor := metadata.NewExtractor(supportedFormats)

	t.Run("ExtractFromNonExistentFile", func(t *testing.T) {
		_, err := extractor.ExtractFromFile("/nonexistent/file.mp3", 0)
		if err == nil {
			t.Error("Expected error when extracting from non-existent file")
		}
	})

	t.Run("ExtractFromInvalidFile", func(t *testing.T) {
		// Create a temporary file that's not a valid audio file
		testDir := t.TempDir()
		invalidFile := filepath.Join(testDir, "invalid.mp3")

		err := os.WriteFile(invalidFile, []byte("this is not an audio file"), 0644)
		if err != nil {
			t.Fatalf("Failed to create test file: %v", err)
		}

		// This should not panic and should handle the error gracefully
		track, _ := extractor.ExtractFromFile(invalidFile, 1)

		// The function should return a track with fallback values even if metadata extraction fails
		if track.ID != 1 {
			t.Errorf("Expected ID 1, got %d", track.ID)
		}

		if track.FilePath != invalidFile {
			t.Errorf("Expected file path %s, got %s", invalidFile, track.FilePath)
		}

		// Should have fallback title based on filename
		expectedTitle := "invalid"
		if track.Title != expectedTitle {
			t.Errorf("Expected title %s, got %s", expectedTitle, track.Title)
		}

		// Should have fallback values for missing metadata
		if track.Artist != "Unknown Artist" {
			t.Errorf("Expected artist 'Unknown Artist', got %s", track.Artist)
		}

		if track.Album != "Unknown Album" {
			t.Errorf("Expected album 'Unknown Album', got %s", track.Album)
		}
	})
}
