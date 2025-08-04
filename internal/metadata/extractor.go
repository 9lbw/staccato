package metadata

import (
	"crypto/md5"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"staccato/pkg/models"

	"github.com/dhowden/tag"
	"github.com/sirupsen/logrus"
	"github.com/tcolgate/mp3"
)

// Extractor handles metadata extraction from audio files
type Extractor struct {
	supportedFormats []string
	logger           *logrus.Logger
	albumArtCache    map[string][]byte // Cache for album art
	albumArtMux      sync.RWMutex      // Mutex for album art cache
}

// NewExtractor creates a new metadata extractor
func NewExtractor(supportedFormats []string) *Extractor {
	logger := logrus.New()
	logger.SetFormatter(&logrus.JSONFormatter{})

	return &Extractor{
		supportedFormats: supportedFormats,
		logger:           logger,
		albumArtCache:    make(map[string][]byte),
		albumArtMux:      sync.RWMutex{},
	}
}

// ExtractFromFile extracts metadata from an audio file
func (e *Extractor) ExtractFromFile(filePath string, id int) (models.Track, error) {
	startTime := time.Now()

	file, err := os.Open(filePath)
	if err != nil {
		e.logger.WithFields(logrus.Fields{
			"filePath": filePath,
			"error":    err.Error(),
		}).Error("Failed to open audio file")
		return models.Track{}, err
	}
	defer file.Close()

	// Get file size
	stat, err := file.Stat()
	if err != nil {
		e.logger.WithFields(logrus.Fields{
			"filePath": filePath,
			"error":    err.Error(),
		}).Error("Failed to get file stats")
		return models.Track{}, err
	}

	// Calculate duration
	duration, err := e.calculateDuration(filePath)
	if err != nil {
		e.logger.WithFields(logrus.Fields{
			"filePath": filePath,
			"error":    err.Error(),
		}).Warn("Failed to calculate duration, setting to 0")
		duration = 0
	}

	// Extract metadata using the tag library
	metadata, err := tag.ReadFrom(file)
	if err != nil {
		// If metadata extraction fails, use filename
		filename := filepath.Base(filePath)
		name := strings.TrimSuffix(filename, filepath.Ext(filename))

		e.logger.WithFields(logrus.Fields{
			"filePath": filePath,
			"error":    err.Error(),
		}).Warn("Failed to extract metadata, using filename")

		return models.Track{
			ID:          id,
			Title:       name,
			Artist:      "Unknown Artist",
			Album:       "Unknown Album",
			TrackNumber: 0,
			Duration:    duration,
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

	// Extract album art
	albumArtID, hasAlbumArt := e.extractAlbumArt(metadata)

	processingTime := time.Since(startTime)
	e.logger.WithFields(logrus.Fields{
		"filePath":       filePath,
		"title":          title,
		"artist":         artist,
		"album":          album,
		"duration":       duration,
		"hasAlbumArt":    hasAlbumArt,
		"processingTime": processingTime,
	}).Debug("Successfully extracted metadata")

	return models.Track{
		ID:          id,
		Title:       title,
		Artist:      artist,
		Album:       album,
		TrackNumber: trackNum,
		Duration:    duration,
		FilePath:    filePath,
		FileSize:    stat.Size(),
		HasAlbumArt: hasAlbumArt,
		AlbumArtID:  albumArtID,
	}, nil
}

// calculateDuration calculates the duration of an audio file in seconds
func (e *Extractor) calculateDuration(filePath string) (int, error) {
	ext := strings.ToLower(filepath.Ext(filePath))

	switch ext {
	case ".mp3":
		return e.calculateMP3Duration(filePath)
	case ".flac":
		return e.calculateFLACDuration(filePath)
	case ".wav":
		return e.calculateWAVDuration(filePath)
	case ".m4a":
		return e.calculateM4ADuration(filePath)
	default:
		return 0, fmt.Errorf("unsupported format for duration calculation: %s", ext)
	}
}

// calculateMP3Duration calculates duration for MP3 files
func (e *Extractor) calculateMP3Duration(filePath string) (int, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return 0, err
	}
	defer file.Close()

	decoder := mp3.NewDecoder(file)
	var duration time.Duration
	var skipped int

	for {
		frame := mp3.Frame{}
		err := decoder.Decode(&frame, &skipped)
		if err != nil {
			if err == io.EOF {
				break
			}
			// If we can't decode properly, try to estimate from file size
			return e.estimateDurationFromSize(filePath, 128) // Assume 128kbps
		}
		duration += frame.Duration()
	}

	return int(duration.Seconds()), nil
}

// calculateFLACDuration calculates duration for FLAC files using metadata
func (e *Extractor) calculateFLACDuration(filePath string) (int, error) {
	// For FLAC files, fall back to size estimation
	return e.estimateDurationFromSize(filePath, 1000) // Assume ~1000kbps for FLAC
}

// calculateWAVDuration calculates duration for WAV files
func (e *Extractor) calculateWAVDuration(filePath string) (int, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return 0, err
	}
	defer file.Close()

	// Read WAV header to get sample rate and data size
	header := make([]byte, 44)
	_, err = file.Read(header)
	if err != nil {
		return e.estimateDurationFromSize(filePath, 1411) // CD quality estimation
	}

	// Basic WAV header parsing
	if string(header[0:4]) != "RIFF" || string(header[8:12]) != "WAVE" {
		return e.estimateDurationFromSize(filePath, 1411)
	}

	return e.estimateDurationFromSize(filePath, 1411)
}

// calculateM4ADuration calculates duration for M4A files
func (e *Extractor) calculateM4ADuration(filePath string) (int, error) {
	return e.estimateDurationFromSize(filePath, 256) // Assume 256kbps for M4A
}

// estimateDurationFromSize estimates duration based on file size and bitrate
func (e *Extractor) estimateDurationFromSize(filePath string, estimatedBitrate int) (int, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return 0, err
	}
	defer file.Close()

	stat, err := file.Stat()
	if err != nil {
		return 0, err
	}

	// Calculate duration: (file size in bits) / (bitrate in bits per second)
	fileSizeBits := stat.Size() * 8
	bitratePerSecond := int64(estimatedBitrate * 1000) // Convert kbps to bps

	if bitratePerSecond == 0 {
		return 0, fmt.Errorf("invalid bitrate for estimation")
	}

	durationSeconds := fileSizeBits / bitratePerSecond
	return int(durationSeconds), nil
}

// extractAlbumArt extracts album art from metadata or looks for cover files
func (e *Extractor) extractAlbumArt(metadata tag.Metadata) (string, bool) {
	// First try to extract embedded album art
	if metadata != nil {
		if picture := metadata.Picture(); picture != nil {
			// Create a unique ID for this album art based on content hash
			hash := md5.Sum(picture.Data)
			artID := fmt.Sprintf("%x", hash)

			// Cache the album art data safely
			e.albumArtMux.Lock()
			e.albumArtCache[artID] = picture.Data
			e.albumArtMux.Unlock()

			return artID, true
		}
	}

	// No embedded album art found; skip directory cover fallback to avoid incorrect cover assignment
	return "", false
}

// GetAlbumArt retrieves cached album art by ID
func (e *Extractor) GetAlbumArt(artID string) ([]byte, bool) {
	// Thread-safe read
	e.albumArtMux.RLock()
	data, exists := e.albumArtCache[artID]
	e.albumArtMux.RUnlock()
	return data, exists
}

// GetAlbumArtMimeType guesses MIME type from album art data
func (e *Extractor) GetAlbumArtMimeType(data []byte) string {
	if len(data) < 4 {
		return "application/octet-stream"
	}

	// Check for common image formats
	if data[0] == 0xFF && data[1] == 0xD8 {
		return "image/jpeg"
	}
	if data[0] == 0x89 && data[1] == 0x50 && data[2] == 0x4E && data[3] == 0x47 {
		return "image/png"
	}
	if data[0] == 0x47 && data[1] == 0x49 && data[2] == 0x46 {
		return "image/gif"
	}

	return "application/octet-stream"
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
