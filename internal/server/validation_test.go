package server

import (
	"testing"

	"staccato/internal/config"
	"staccato/internal/metadata"

	"github.com/sirupsen/logrus"
)

func createTestMusicServer() *MusicServer {
	cfg := config.DefaultConfig()
	cfg.Music.LibraryPath = "/tmp/test-music"

	logger := logrus.New()
	logger.SetLevel(logrus.ErrorLevel) // Reduce noise in tests

	return &MusicServer{
		config:    cfg,
		extractor: metadata.NewExtractor(cfg.Music.SupportedFormats),
		logger:    logger,
	}
}

func TestValidateTrackID(t *testing.T) {
	ms := createTestMusicServer()

	tests := []struct {
		name      string
		pathParts []string
		minParts  int
		wantID    int
		wantError bool
	}{
		{
			name:      "valid track ID",
			pathParts: []string{"", "stream", "123"},
			minParts:  3,
			wantID:    123,
			wantError: false,
		},
		{
			name:      "missing track ID",
			pathParts: []string{"", "stream"},
			minParts:  3,
			wantID:    0,
			wantError: true,
		},
		{
			name:      "invalid track ID format",
			pathParts: []string{"", "stream", "abc"},
			minParts:  3,
			wantID:    0,
			wantError: true,
		},
		{
			name:      "negative track ID",
			pathParts: []string{"", "stream", "-1"},
			minParts:  3,
			wantID:    0,
			wantError: true,
		},
		{
			name:      "zero track ID",
			pathParts: []string{"", "stream", "0"},
			minParts:  3,
			wantID:    0,
			wantError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			id, err := ms.validateTrackID(tt.pathParts, tt.minParts)

			if tt.wantError && err == nil {
				t.Errorf("validateTrackID() expected error but got none")
			}
			if !tt.wantError && err != nil {
				t.Errorf("validateTrackID() unexpected error: %v", err)
			}
			if id != tt.wantID {
				t.Errorf("validateTrackID() = %v, want %v", id, tt.wantID)
			}
		})
	}
}

func TestValidateSearchQuery(t *testing.T) {
	ms := createTestMusicServer()

	tests := []struct {
		name      string
		query     string
		wantError bool
	}{
		{
			name:      "valid search query",
			query:     "Beatles",
			wantError: false,
		},
		{
			name:      "empty search query",
			query:     "",
			wantError: false,
		},
		{
			name:      "long search query",
			query:     string(make([]byte, 1001)), // 1001 characters
			wantError: true,
		},
		{
			name:      "query with null byte",
			query:     "test\x00query",
			wantError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ms.validateSearchQuery(tt.query)

			if tt.wantError && err == nil {
				t.Errorf("validateSearchQuery() expected error but got none")
			}
			if !tt.wantError && err != nil {
				t.Errorf("validateSearchQuery() unexpected error: %v", err)
			}
		})
	}
}

func TestValidateFilePath(t *testing.T) {
	ms := createTestMusicServer()

	tests := []struct {
		name      string
		filePath  string
		wantError bool
	}{
		{
			name:      "valid file path within music directory",
			filePath:  "/tmp/test-music/song.mp3",
			wantError: false,
		},
		{
			name:      "path traversal attempt",
			filePath:  "/tmp/test-music/../../../etc/passwd",
			wantError: true,
		},
		{
			name:      "absolute path outside music directory",
			filePath:  "/etc/passwd",
			wantError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ms.validateFilePath(tt.filePath)

			if tt.wantError && err == nil {
				t.Errorf("validateFilePath() expected error but got none")
			}
			if !tt.wantError && err != nil {
				t.Errorf("validateFilePath() unexpected error: %v", err)
			}
		})
	}
}

func TestValidateURL(t *testing.T) {
	ms := createTestMusicServer()

	tests := []struct {
		name      string
		url       string
		wantError bool
	}{
		{
			name:      "valid HTTP URL",
			url:       "http://example.com/song.mp3",
			wantError: false,
		},
		{
			name:      "valid HTTPS URL",
			url:       "https://example.com/song.mp3",
			wantError: false,
		},
		{
			name:      "empty URL",
			url:       "",
			wantError: true,
		},
		{
			name:      "invalid protocol",
			url:       "ftp://example.com/song.mp3",
			wantError: true,
		},
		{
			name:      "malformed URL",
			url:       "not-a-url",
			wantError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ms.validateURL(tt.url)

			if tt.wantError && err == nil {
				t.Errorf("validateURL() expected error but got none")
			}
			if !tt.wantError && err != nil {
				t.Errorf("validateURL() unexpected error: %v", err)
			}
		})
	}
}

func TestSanitizeInput(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "normal input",
			input:    "Hello World",
			expected: "Hello World",
		},
		{
			name:     "input with null bytes",
			input:    "Hello\x00World",
			expected: "HelloWorld",
		},
		{
			name:     "input with whitespace",
			input:    "  Hello World  ",
			expected: "Hello World",
		},
		{
			name:     "empty input",
			input:    "",
			expected: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := sanitizeInput(tt.input)
			if result != tt.expected {
				t.Errorf("sanitizeInput() = %q, want %q", result, tt.expected)
			}
		})
	}
}
