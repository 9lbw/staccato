package metadata

import (
	"os"
	"path/filepath"
	"strings"

	"staccato/pkg/models"

	"github.com/dhowden/tag"
)

// Extractor handles metadata extraction from audio files
type Extractor struct {
	supportedFormats []string
}

// NewExtractor creates a new metadata extractor
func NewExtractor(supportedFormats []string) *Extractor {
	return &Extractor{
		supportedFormats: supportedFormats,
	}
}

// ExtractFromFile extracts metadata from an audio file
func (e *Extractor) ExtractFromFile(filePath string, id int) (models.Track, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return models.Track{}, err
	}
	defer file.Close()

	// Get file size
	stat, err := file.Stat()
	if err != nil {
		return models.Track{}, err
	}

	// Extract metadata using the tag library
	metadata, err := tag.ReadFrom(file)
	if err != nil {
		// If metadata extraction fails, use filename
		filename := filepath.Base(filePath)
		name := strings.TrimSuffix(filename, filepath.Ext(filename))

		return models.Track{
			ID:          id,
			Title:       name,
			Artist:      "Unknown Artist",
			Album:       "Unknown Album",
			TrackNumber: 0,
			Duration:    0, // Duration unknown without metadata
			FilePath:    filePath,
			FileSize:    stat.Size(),
		}, nil
	}

	title := metadata.Title()
	if title == "" {
		title = strings.TrimSuffix(filepath.Base(filePath), filepath.Ext(filePath))
	}

	artist := metadata.Artist()
	if artist == "" {
		artist = "Unknown Artist"
	}

	album := metadata.Album()
	if album == "" {
		album = "Unknown Album"
	}

	// Extract track number
	trackNum, _ := metadata.Track()

	return models.Track{
		ID:          id,
		Title:       title,
		Artist:      artist,
		Album:       album,
		TrackNumber: trackNum,
		Duration:    0, // We could calculate this, but it's complex for different formats
		FilePath:    filePath,
		FileSize:    stat.Size(),
	}, nil
}

// IsAudioFile checks if a file is a supported audio format
func (e *Extractor) IsAudioFile(filePath string) bool {
	ext := strings.ToLower(filepath.Ext(filePath))
	for _, format := range e.supportedFormats {
		if ext == format {
			return true
		}
	}
	return false
}

// GetContentType returns the MIME type for an audio file
func (e *Extractor) GetContentType(filePath string) string {
	ext := strings.ToLower(filepath.Ext(filePath))
	switch ext {
	case ".mp3":
		return "audio/mpeg"
	case ".flac":
		return "audio/flac"
	case ".wav":
		return "audio/wav"
	case ".m4a":
		return "audio/mp4"
	default:
		return "application/octet-stream"
	}
}
